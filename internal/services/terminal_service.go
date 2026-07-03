package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/vsuhanov/scripto/internal/tui/colors"
	"github.com/vsuhanov/scripto/internal/utils"

	"github.com/charmbracelet/lipgloss"
	xterm "github.com/charmbracelet/x/term"
)

type osExitFunc func(code int)
type osWriteFileFunc func(name string, data []byte, perm os.FileMode) error

type exitCode int

const (
	exitCodeSuccess         exitCode = 0
	exitCodeError           exitCode = 1
	exitCodeBuiltinComplete exitCode = 3
	exitCodeExternalEditor  exitCode = 4
)

type TerminalServiceCommand interface{}

type ExitCommand struct {
	Code int
}

type ExecuteScriptCommand struct {
	Command           string
	Name              string
	PlaceholderValues map[string]string
	WorkingDir        string
	WriteHistory      bool
}

type EditScriptExternalCommand struct {
	ScriptPath string
}

type TerminalServiceOptions struct {
	targetCommandFile string
}

type TerminalService struct {
	options       TerminalServiceOptions
	exitFunc      osExitFunc
	writeFileFunc osWriteFileFunc
}

func NewTerminalService(options TerminalServiceOptions) *TerminalService {
	return &TerminalService{
		options:       options,
		exitFunc:      os.Exit,
		writeFileFunc: os.WriteFile,
	}
}

func (ts *TerminalService) PrepareExit(code int) TerminalServiceCommand {
	return &ExitCommand{Code: code}
}

func (ts *TerminalService) PrepareScriptExecution(command, name string, placeholderValues map[string]string, workingDir string, writeHistory bool) TerminalServiceCommand {
	return &ExecuteScriptCommand{Command: command, Name: name, PlaceholderValues: placeholderValues, WorkingDir: workingDir, WriteHistory: writeHistory}
}

func (ts *TerminalService) PrepareExternalEditing(scriptPath string) TerminalServiceCommand {
	return &EditScriptExternalCommand{ScriptPath: scriptPath}
}

func (ts *TerminalService) ExecuteCommand(cmd TerminalServiceCommand) {
	if cmd == nil {
		return
	}

	switch c := cmd.(type) {
	case *ExitCommand:
		ts.exitFunc(c.Code)
	case *ExecuteScriptCommand:
		ts.executeScriptCommand(c.Command, c.Name, c.PlaceholderValues, c.WorkingDir, c.WriteHistory)
	case *EditScriptExternalCommand:
		ts.editScriptExternalCommand(c.ScriptPath)
	}
}

func (ts *TerminalService) PrintScriptSavedBox(name, scope, scopeColor, command string) {
	width, _, err := xterm.GetSize(os.Stderr.Fd())
	if err != nil || width <= 0 {
		width = 80
	}
	titleStyle := lipgloss.NewStyle().Foreground(colors.Primary).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#e5e7eb"})
	scopeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(scopeColor)).Bold(true)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#4b5563"})
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(false).
		BorderRight(false).
		BorderForeground(colors.Primary).
		PaddingLeft(1).
		Width(width - 2)

	separator := strings.Repeat("─", width-4)
	content := titleStyle.Render("Scripto command saved") + "\n" +
		nameStyle.Render(name) + "  " + scopeStyle.Render("["+scope+"]") + "\n" +
		separatorStyle.Render(separator) + "\n" +
		command
	fmt.Fprintln(os.Stderr, boxStyle.Render(content))
}

func printScriptBox(command, name string) {
	width, _, err := xterm.GetSize(os.Stderr.Fd())
	if err != nil || width <= 0 {
		width = 80
	}
	titleStyle := lipgloss.NewStyle().Foreground(colors.Primary).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#e5e7eb"})
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(false).
		BorderRight(false).
		BorderForeground(colors.Primary).
		PaddingLeft(1).
		Width(width - 2)

	title := "Scripto command"
	if name != "" {
		title = title + "  " + nameStyle.Render(name)
	}
	content := titleStyle.Render(title) + "\n" + command
	fmt.Fprintln(os.Stderr, boxStyle.Render(content))
}

func (ts *TerminalService) executeScriptCommand(command, name string, placeholderValues map[string]string, workingDir string, writeHistory bool) {
	if utils.IsStderrTerminal() {
		printScriptBox(command, name)
	}
	cmdFdPath := ts.options.targetCommandFile
	if cmdFdPath != "" {
		content := ""
		if name != "" {
			content += "printf " + shellescape("\\e]2;scripto "+name+"\\a") + "\n"
		}
		if writeHistory && name != "" {
			cwd, _ := os.Getwd()
			richEntry := buildRichHistoryEntry(name, placeholderValues, workingDir, cwd)
			if richEntry != "" {
				content += "\nprint -s " + shellescape(richEntry)
			} else {
				content += "\nprint -s " + shellescape("scripto "+name)
			}
			content += "\n"
		}
		content += command
		_ = ts.writeFileFunc(cmdFdPath, []byte(content), 0600)
	} else {
		fmt.Print(command)
	}
	ts.exitFunc(int(exitCodeSuccess))
}

func buildRichHistoryEntry(name string, values map[string]string, workingDir, cwd string) string {
	if len(values) == 0 && (workingDir == "" || workingDir == cwd) {
		return ""
	}
	entry := "scripto " + name + " --"
	for k, v := range values {
		entry += " --" + k + "=" + shellQuoteValue(v)
	}
	if workingDir != "" && workingDir != cwd {
		entry += " --working-dir=" + shellQuoteValue(workingDir)
	}
	return entry
}

func shellQuoteValue(v string) string {
	if strings.ContainsAny(v, " \t\n'\"\\$`!") {
		return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
	}
	return v
}

func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (ts *TerminalService) editScriptExternalCommand(scriptPath string) {
	cmdFdPath := ts.options.targetCommandFile
	if cmdFdPath != "" {
		_ = ts.writeFileFunc(cmdFdPath, []byte(scriptPath), 0600)
	} else {
		fmt.Print(scriptPath)
	}
	ts.exitFunc(int(exitCodeExternalEditor))
}

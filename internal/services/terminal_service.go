package services

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/vsuhanov/scripto/internal/tui/colors"
	"github.com/vsuhanov/scripto/internal/utils"

	"github.com/charmbracelet/lipgloss"
	xterm "github.com/charmbracelet/x/term"
)

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
	ExecutionID       string
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
	executionIDFile   string
}

type TerminalService struct {
	options       TerminalServiceOptions
	writeFileFunc osWriteFileFunc
}

func NewTerminalService(options TerminalServiceOptions) *TerminalService {
	return &TerminalService{
		options:       options,
		writeFileFunc: os.WriteFile,
	}
}

func (ts *TerminalService) PrepareExit(code int) TerminalServiceCommand {
	return &ExitCommand{Code: code}
}

func (ts *TerminalService) PrepareScriptExecution(command, name string, placeholderValues map[string]string, workingDir string, writeHistory bool) TerminalServiceCommand {
	return &ExecuteScriptCommand{Command: command, Name: name, PlaceholderValues: placeholderValues, WorkingDir: workingDir, WriteHistory: writeHistory}
}

func (ts *TerminalService) SetExecutionID(cmd TerminalServiceCommand, id string) {
	if c, ok := cmd.(*ExecuteScriptCommand); ok {
		c.ExecutionID = id
	}
}

func (ts *TerminalService) PrepareExternalEditing(scriptPath string) TerminalServiceCommand {
	return &EditScriptExternalCommand{ScriptPath: scriptPath}
}

func (ts *TerminalService) ExecuteCommand(cmd TerminalServiceCommand) int {
	switch c := cmd.(type) {
	case *ExitCommand:
		return c.Code
	case *ExecuteScriptCommand:
		return ts.executeScriptCommand(c)
	case *EditScriptExternalCommand:
		return ts.editScriptExternalCommand(c.ScriptPath)
	}
	return int(exitCodeBuiltinComplete)
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

func (ts *TerminalService) executeScriptCommand(c *ExecuteScriptCommand) int {
	if utils.IsStderrTerminal() {
		printScriptBox(c.Command, c.Name)
	}
	cmdFdPath := ts.options.targetCommandFile
	if cmdFdPath == "" {
		fmt.Print(c.Command)
		return int(exitCodeSuccess)
	}

	content := ""
	if c.Name != "" {
		content += "printf " + utils.ShellQuote("\\e]2;scripto "+c.Name+"\\a") + "\n"
	}
	if c.WriteHistory {
		if c.Name != "" {
			cwd, _ := os.Getwd()
			richEntry := buildRichHistoryEntry(c.Name, c.PlaceholderValues, c.WorkingDir, cwd)
			if richEntry != "" {
				content += "\nprint -s " + utils.ShellQuote(richEntry)
			} else {
				content += "\nprint -s " + utils.ShellQuote("scripto "+c.Name)
			}
		} else {
			content += "\nprint -s " + utils.ShellQuote(c.Command)
		}
		content += "\n"
	}
	content += c.Command
	if err := ts.writeFileFunc(cmdFdPath, []byte(content), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to hand command to shell: %v\n", err)
		return int(exitCodeError)
	}
	if ts.options.executionIDFile != "" && c.ExecutionID != "" {
		_ = ts.writeFileFunc(ts.options.executionIDFile, []byte(c.ExecutionID), 0600)
	}
	return int(exitCodeSuccess)
}

func buildRichHistoryEntry(name string, values map[string]string, workingDir, cwd string) string {
	if len(values) == 0 && (workingDir == "" || workingDir == cwd) {
		return ""
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	entry := "scripto " + utils.ShellQuoteIfNeeded(name) + " --"
	for _, k := range keys {
		entry += " " + utils.ShellQuoteIfNeeded("--"+k+"="+values[k])
	}
	if workingDir != "" && workingDir != cwd {
		entry += " " + utils.ShellQuoteIfNeeded("--working-dir="+workingDir)
	}
	return entry
}

func (ts *TerminalService) editScriptExternalCommand(scriptPath string) int {
	cmdFdPath := ts.options.targetCommandFile
	if cmdFdPath != "" {
		_ = ts.writeFileFunc(cmdFdPath, []byte(scriptPath), 0600)
	} else {
		fmt.Print(scriptPath)
	}
	return int(exitCodeExternalEditor)
}

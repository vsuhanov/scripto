package services

import (
	"fmt"
	"os"
	"strings"

	"scripto/internal/tui/colors"
	"scripto/internal/utils"

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
	Command string
}

type EditScriptExternalCommand struct {
	ScriptPath string
}

type TerminalServiceOptions struct {
	targetCommandFile string
}

type TerminalService struct {
	options TerminalServiceOptions
	exitFunc osExitFunc
	writeFileFunc osWriteFileFunc	

}

func NewTerminalService(options TerminalServiceOptions) *TerminalService {
	return &TerminalService{
		options: options,
		exitFunc: os.Exit,
		writeFileFunc: os.WriteFile,
	}
}

func (ts *TerminalService) PrepareExit(code int) TerminalServiceCommand {
	return &ExitCommand{Code: code}
}

func (ts *TerminalService) PrepareScriptExecution(command string) TerminalServiceCommand {
	return &ExecuteScriptCommand{Command: command}
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
		ts.executeScriptCommand(c.Command)
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

func printScriptBox(command string) {
	width, _, err := xterm.GetSize(os.Stderr.Fd())
	if err != nil || width <= 0 {
		width = 80
	}
	titleStyle := lipgloss.NewStyle().Foreground(colors.Primary).Bold(true)
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(false).
		BorderRight(false).
		BorderForeground(colors.Primary).
		PaddingLeft(1).
		Width(width - 2)

	content := titleStyle.Render("Scripto command") + "\n" + command
	fmt.Fprintln(os.Stderr, boxStyle.Render(content))
}

func (ts *TerminalService) executeScriptCommand(command string) {
	if utils.IsStderrTerminal() {
		printScriptBox(command)
	}
	cmdFdPath := ts.options.targetCommandFile
	if cmdFdPath != "" {
		_ = ts.writeFileFunc(cmdFdPath, []byte(command), 0600)
	} else {
		fmt.Print(command)
	}
	ts.exitFunc(int(exitCodeSuccess))
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


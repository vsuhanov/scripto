package services

import (
	"fmt"
	"os"
)

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

type TerminalService struct{}

func NewTerminalService() *TerminalService {
	return &TerminalService{}
}

func (ts *TerminalService) ExecuteScript(finalCommand string) error {
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		err := os.WriteFile(cmdFdPath, []byte(finalCommand), 0600)
		if err != nil {
			return fmt.Errorf("failed to write command to descriptor: %w", err)
		}
		ts.exit(int(exitCodeSuccess))
		return nil
	}

	fmt.Print(finalCommand)
	ts.exit(int(exitCodeSuccess))
	return nil
}

func (ts *TerminalService) EditScriptExternal(scriptPath string) error {
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		err := os.WriteFile(cmdFdPath, []byte(scriptPath), 0600)
		if err != nil {
			return fmt.Errorf("failed to write script path to descriptor: %w", err)
		}
		ts.exit(int(exitCodeExternalEditor))
		return nil
	}

	fmt.Print(scriptPath)
	ts.exit(int(exitCodeExternalEditor))
	return nil
}

func (ts *TerminalService) ExitWithError(message string) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", message)
	ts.exit(int(exitCodeError))
}

func (ts *TerminalService) ExitBuiltinComplete() {
	ts.exit(int(exitCodeBuiltinComplete))
}

func (ts *TerminalService) ExitWithCode(code int) {
	ts.exit(code)
}

func (ts *TerminalService) exit(code int) {
	os.Exit(code)
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
		ts.exit(c.Code)
	case *ExecuteScriptCommand:
		ts.executeScriptCommand(c.Command)
	case *EditScriptExternalCommand:
		ts.editScriptExternalCommand(c.ScriptPath)
	}
}

func (ts *TerminalService) executeScriptCommand(finalCommand string) {
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		_ = os.WriteFile(cmdFdPath, []byte(finalCommand), 0600)
		ts.exit(int(exitCodeSuccess))
		return
	}

	fmt.Print(finalCommand)
	ts.exit(int(exitCodeSuccess))
}

func (ts *TerminalService) editScriptExternalCommand(scriptPath string) {
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		_ = os.WriteFile(cmdFdPath, []byte(scriptPath), 0600)
		ts.exit(int(exitCodeExternalEditor))
		return
	}

	fmt.Print(scriptPath)
	ts.exit(int(exitCodeExternalEditor))
}

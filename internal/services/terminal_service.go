package services

import (
	"fmt"
	"os"
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
		exitFunc: os.Exit
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
		ts.exit(c.Code)
	case *ExecuteScriptCommand:
		ts.executeScriptCommand(c.Command)
	case *EditScriptExternalCommand:
		ts.editScriptExternalCommand(c.ScriptPath)
	}
}

func (ts *TerminalService) executeScriptCommand(command string) {
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


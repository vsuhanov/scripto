package services

import (
	"fmt"
	"os"
)

type ExitCode int

const (
	ExitCodeSuccess         ExitCode = 0
	ExitCodeError           ExitCode = 1
	ExitCodeBuiltinComplete ExitCode = 3
	ExitCodeExternalEditor  ExitCode = 4
)

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
		os.Exit(int(ExitCodeSuccess))
		return nil
	}

	fmt.Print(finalCommand)
	os.Exit(int(ExitCodeSuccess))
	return nil
}

func (ts *TerminalService) EditScriptExternal(scriptPath string) error {
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		err := os.WriteFile(cmdFdPath, []byte(scriptPath), 0600)
		if err != nil {
			return fmt.Errorf("failed to write script path to descriptor: %w", err)
		}
		os.Exit(int(ExitCodeExternalEditor))
		return nil
	}

	fmt.Print(scriptPath)
	os.Exit(int(ExitCodeExternalEditor))
	return nil
}

func (ts *TerminalService) ExitWithError(message string) error {
	fmt.Fprintf(os.Stderr, "Error: %v\n", message)
	os.Exit(int(ExitCodeError))
	return nil
}

func (ts *TerminalService) ExitBuiltinComplete() error {
	os.Exit(int(ExitCodeBuiltinComplete))
	return nil
}

package services

import (
	"log"
	"os"
)

type Container struct {
	ScriptService           *ScriptService
	ExecutionService        *ExecutionService
	TerminalService         *TerminalService
	HistoryService          *HistoryService
	ExecutionHistoryService *ExecutionHistoryService
	ShellHistoryService     *ShellHistoryService
}

func NewContainer() (*Container, error) {
	scriptService, err := NewScriptService()
	if err != nil {
		return nil, err
	}

	log.Printf("SCRIPTO_CMD_FD=%v", os.Getenv("SCRIPTO_CMD_FD"))

	executionHistoryService, err := NewExecutionHistoryService()
	if err != nil {
		log.Printf("Warning: failed to initialize execution history service: %v", err)
		executionHistoryService = nil
	} else {
		log.Printf("Container: execution history service initialized")
	}

	var shellHistoryService *ShellHistoryService
	if executionHistoryService != nil {
		shellHistoryService = NewShellHistoryServiceWithDB(executionHistoryService.DB())
	} else if svc, err := NewShellHistoryService(); err != nil {
		log.Printf("Warning: failed to initialize shell history service: %v", err)
	} else {
		shellHistoryService = svc
	}

	return &Container{
		ScriptService:    scriptService,
		ExecutionService: NewExecutionService(),
		TerminalService: NewTerminalService(TerminalServiceOptions{
			targetCommandFile: os.Getenv("SCRIPTO_CMD_FD"),
			executionIDFile:   os.Getenv("SCRIPTO_EXEC_ID_FILE"),
		}),
		HistoryService:          NewHistoryService(),
		ExecutionHistoryService: executionHistoryService,
		ShellHistoryService:     shellHistoryService,
	}, nil
}

func (c *Container) Close() {
	if c.ExecutionHistoryService != nil {
		c.ExecutionHistoryService.Close()
	} else if c.ShellHistoryService != nil {
		c.ShellHistoryService.Close()
	}
}

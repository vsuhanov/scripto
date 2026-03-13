package services

import (
	"log"
	"os"
)

type Container struct {
	ScriptService    *ScriptService
	ExecutionService *ExecutionService
	TerminalService  *TerminalService
	HistoryService   *HistoryService
}

func NewContainer() (*Container, error) {
	scriptService, err := NewScriptService()
	if err != nil {
		return nil, err
	}

	log.Printf("SCRIPTO_CMD_FD=%v", os.Getenv("SCRIPTO_CMD_FD"))

	return &Container{
		ScriptService:    scriptService,
		ExecutionService: NewExecutionService(),
		TerminalService: NewTerminalService(TerminalServiceOptions{
			targetCommandFile: os.Getenv("SCRIPTO_CMD_FD"),
		}),
		HistoryService: NewHistoryService(),
	}, nil
}

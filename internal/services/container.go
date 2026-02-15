package services

// Container holds all application services
type Container struct {
	ScriptService    *ScriptService
	ExecutionService *ExecutionService
	TerminalService  *TerminalService
	HistoryService   *HistoryService
}

// New creates and initializes all services
func NewContainer() (*Container, error) {
	scriptService, err := NewScriptService()
	if err != nil {
		return nil, err
	}

	return &Container{
		ScriptService:    scriptService,
		ExecutionService: NewExecutionService(),
		TerminalService:  NewTerminalService(),
		HistoryService:  NewHistoryService(),
	}, nil
}

package tui

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/internal/services"
)

func RunApp(container *services.Container, request TuiRequest) error {
	rootModel, err := NewRootModel(container, request)
	if err != nil {
		return err
	}

	program := tea.NewProgram(rootModel, tea.WithAltScreen())
	finalModel, err := program.Run()

	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if m, ok := finalModel.(RootModel); ok {
		cmd := m.GetPendingCommand()
		log.Printf("RunApp: pendingCommand=%T", cmd)
		if cmd != nil {
			record := m.GetPendingHistoryRecord()
			log.Printf("RunApp: pendingHistoryRecord=%v, ExecutionHistoryService=%v", record != nil, container.ExecutionHistoryService != nil)
			if record != nil && container.ExecutionHistoryService != nil {
				log.Printf("RunApp: saving execution record scriptID=%q executedScript=%q", record.ScriptID, record.ExecutedScript)
				container.ExecutionHistoryService.SaveExecution(*record)
			}
			container.TerminalService.ExecuteCommand(cmd)
		}
	}

	return nil
}

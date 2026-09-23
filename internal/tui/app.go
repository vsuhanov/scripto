package tui

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/vsuhanov/scripto/internal/services"
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
			if record != nil && record.ScriptID != "" && container.ExecutionHistoryService != nil {
				log.Printf("RunApp: saving execution record scriptID=%q executedScript=%q", record.ScriptID, record.ExecutedScript)
				container.ExecutionHistoryService.SaveExecution(*record)
			}
			if record != nil && container.ShellHistoryService != nil {
				recordScriptoRun(container, record)
			}
			container.TerminalService.ExecuteCommand(cmd)
		}
		if saved := m.GetPendingSavedScript(); saved != nil {
			container.TerminalService.PrintScriptSavedBox(saved.Name, saved.Scope, GetScopeColorHex(saved.Scope), m.GetPendingSavedCommand())
		}
	}

	return nil
}

func recordScriptoRun(container *services.Container, record *services.ExecutionRecord) {
	if record.ExecutedScript == "" {
		return
	}
	workingDir := record.WorkingDirectory
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	id := fmt.Sprintf("scripto-%s", uuid.New().String())
	if err := container.ShellHistoryService.RecordStart(id, record.ExecutedScript, workingDir,
		strconv.Itoa(os.Getppid()), services.ShellHistorySourceScripto, time.Now().Unix()); err != nil {
		log.Printf("RunApp: failed to record shell history entry: %v", err)
		return
	}
	if record.ScriptID != "" {
		if err := container.ShellHistoryService.MarkSaved(id, record.ScriptID); err != nil {
			log.Printf("RunApp: failed to link shell history entry to script: %v", err)
		}
	}
}

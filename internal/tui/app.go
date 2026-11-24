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

func RunApp(container *services.Container, request TuiRequest) (int, error) {
	rootModel, err := NewRootModel(container, request)
	if err != nil {
		return ExitError, err
	}

	program := tea.NewProgram(rootModel, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return ExitError, fmt.Errorf("TUI error: %w", err)
	}

	var m *RootModel
	switch fm := finalModel.(type) {
	case RootModel:
		m = &fm
	case *RootModel:
		m = fm
	default:
		return ExitBuiltinComplete, nil
	}

	code := ExitBuiltinComplete
	if cmd := m.GetPendingCommand(); cmd != nil {
		if record := m.GetPendingHistoryRecord(); record != nil {
			if record.ID == "" {
				record.ID = uuid.New().String()
			}
			container.TerminalService.SetExecutionID(cmd, record.ID)
			if record.ScriptID != "" && container.ExecutionHistoryService != nil {
				container.ExecutionHistoryService.SaveExecution(*record)
			}
			if container.ShellHistoryService != nil {
				recordScriptoRun(container, record)
			}
		}
		code = container.TerminalService.ExecuteCommand(cmd)
	}
	if saved := m.GetPendingSavedScript(); saved != nil {
		container.TerminalService.PrintScriptSavedBox(saved.Name, saved.Scope, GetScopeColorHex(saved.Scope), m.GetPendingSavedCommand())
	}

	return code, nil
}

func ScriptoRunHistoryID(executionID string) string {
	return "scripto-" + executionID
}

func recordScriptoRun(container *services.Container, record *services.ExecutionRecord) {
	if record.ExecutedScript == "" {
		return
	}
	workingDir := record.WorkingDirectory
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	id := ScriptoRunHistoryID(record.ID)
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

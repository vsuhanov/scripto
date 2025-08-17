package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI and returns the result
func Run() error {
	// Create and run the TUI
	model := NewModel()
	program := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Handle the result
	if m, ok := finalModel.(MainModel); ok {
		return handleTUIResult(m)
	}

	return nil
}

// handleTUIResult handles the TUI exit result
func handleTUIResult(m MainModel) error {
	// Check if we have any pending messages that indicate special exit conditions
	// This is a simplified approach - in a more complex implementation,
	// we'd use proper message handling

	// For now, we'll use environment variables or exit codes to communicate
	// with the shell wrapper

	// If we reach here without special handling, it means normal quit
	return nil
}


// TUIResult represents the result of TUI interaction
type TUIResult struct {
	Action     ActionType
	ScriptPath string
	ExitCode   int
}


// RunWithResult runs the TUI and returns the selected script or action
func RunWithResult() (TUIResult, error) {
	model := NewModel()

	// Use a custom program that can capture final state
	program := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return TUIResult{ExitCode: 1}, fmt.Errorf("TUI error: %w", err)
	}

	// Extract result from final model
	if m, ok := finalModel.(MainModel); ok {
		// Check if user quit - don't execute anything
		if m.quitting {
			return TUIResult{ExitCode: 3}, nil // Normal quit
		}

		// Check for external edit mode
		if m.editMode && m.externalEdit && m.selectedIdx >= 0 && m.selectedIdx < len(m.scripts) {
			selected := m.scripts[m.selectedIdx]
			scriptPath := selected.Script.FilePath

			if scriptPath == "" {
				return TUIResult{ExitCode: 1}, fmt.Errorf("cannot edit: script has no file path")
			}

			return TUIResult{
				Action:     ActionEditScriptExternal,
				ScriptPath: scriptPath,
				ExitCode:   4, // Special exit code for edit
			}, nil
		}

		// Check for execute mode
		if m.selectedIdx >= 0 && m.selectedIdx < len(m.scripts) && !m.editMode {
			selected := m.scripts[m.selectedIdx]

			// Return script path for execution
			scriptPath := selected.Script.FilePath
			return TUIResult{
				Action:     ActionExecuteScript,
				ScriptPath: scriptPath,
				ExitCode:   0,
			}, nil
		}
	}

	// Normal quit without selection
	return TUIResult{ExitCode: 3}, nil // Exit code 3 indicates built-in command completion
}

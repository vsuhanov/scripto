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
	ScriptPath string
	ExitCode   int
}

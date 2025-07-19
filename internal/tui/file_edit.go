package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// RunFileEditTUI runs the TUI for editing a script loaded from a file
func RunFileEditTUI(command, filePath, suggestedName string, isGlobal bool) (AddTUIResult, error) {
	model := NewFileEditModel(command, filePath, suggestedName, isGlobal)
	program := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return AddTUIResult{Cancelled: true}, fmt.Errorf("TUI error: %w", err)
	}

	// Extract result from final model
	if m, ok := finalModel.(FileEditModel); ok {
		return AddTUIResult{
			Cancelled: m.cancelled,
		}, nil
	}

	return AddTUIResult{Cancelled: true}, nil
}

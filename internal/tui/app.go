package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/internal/services"
)

func RunApp(container *services.Container, mode StartMode) error {
	rootModel, err := NewRootModel(container, mode)
	if err != nil {
		return err
	}

	program := tea.NewProgram(rootModel, tea.WithAltScreen())
	_, err = program.Run()

	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

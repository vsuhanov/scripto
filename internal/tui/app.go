package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/internal/services"
)

type StartMode int

const (
	StartAtMainList StartMode = iota
	StartAtAdd
)

func RunApp(container *services.Container, mode StartMode) error {
	rootModel, err := NewRootModel(container)
	if err != nil {
		return err
	}

	if mode == StartAtAdd {
		historyScreen := NewHistoryScreen(container)
		rootModel.screenStack = append(rootModel.screenStack, rootModel.currentScreen)
		rootModel.currentScreen = historyScreen
	}

	program := tea.NewProgram(rootModel, tea.WithAltScreen())
	_, err = program.Run()

	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

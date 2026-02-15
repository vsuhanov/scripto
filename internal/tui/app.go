package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// StartMode determines which screen to start with
type StartMode int

const (
	StartAtMainList StartMode = iota
	StartAtHistory
)

// RunApp starts the main TUI application with the specified start mode
func RunApp(mode StartMode) (TUIResult, error) {
	rootModel, err := NewRootModel()
	if err != nil {
		return TUIResult{ExitCode: 1}, err
	}

	// Set the initial screen based on mode
	if mode == StartAtHistory {
		historyScreen := NewHistoryScreen()
		historyScreen.SetServices(rootModel.scriptService)
		rootModel.screenStack = append(rootModel.screenStack, rootModel.currentScreen)
		rootModel.currentScreen = historyScreen
	}

	program := tea.NewProgram(rootModel, tea.WithAltScreen())
	finalModel, err := program.Run()

	if err != nil {
		return TUIResult{ExitCode: 1}, fmt.Errorf("TUI error: %w", err)
	}

	root := finalModel.(*RootModel)
	return TUIResult{
		ExitCode:   root.exitCode,
		ScriptPath: root.exitMessage,
	}, nil
}

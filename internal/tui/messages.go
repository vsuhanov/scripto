package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/entities"
	"scripto/internal/script"
	"scripto/internal/storage"
)

// Messages for the TUI components
type (
	ScriptsLoadedMsg []script.MatchResult
	ErrorMsg         error
	StatusMsg        string
)

// Navigation messages
type NavigateToScreenMsg struct {
	screen tea.Model
}

type NavigateBackMsg struct{}

type ExitAppMsg struct {
	exitCode int
	message  string
}

// Business action messages
type ExecuteScriptMsg struct {
	scriptPath string
}

type SaveScriptMsg struct {
	script   entities.Script
	command  string
	original *entities.Script
}

type EditScriptExternalMsg struct {
	scriptPath string
}

type DeleteScriptMsg struct {
	script entities.Script
}

type RefreshScriptsMsg struct{}

type ShowScriptEditorMsg struct {
	script entities.Script
}

type ShowHistoryScreenMsg struct{}

// ScriptEditorResult represents the result of the script editor
type ScriptEditorResult struct {
	Script    entities.Script
	Command   string
	Cancelled bool
}

// loadScripts loads all available scripts
func loadScripts() tea.Cmd {
	return func() tea.Msg {
		// Load configuration
		configPath, err := storage.GetConfigPath()
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to get config path: %w", err))
		}

		config, err := storage.ReadConfig(configPath)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to read config: %w", err))
		}

		// Create matcher and find all scripts
		matcher := script.NewMatcher(config)
		scripts, err := matcher.FindAllScripts()
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to find scripts: %w", err))
		}

		return ScriptsLoadedMsg(scripts)
	}
}

// readScriptFile reads the content of a script file
func readScriptFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
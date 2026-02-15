package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/entities"
	"scripto/internal/script"
	"scripto/internal/services"
)

// Exit codes for the TUI application
const (
	ExitSuccess         = 0
	ExitError           = 1
	ExitBuiltinComplete = 3
	ExitExternalEditor  = 4
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
func loadScripts(scriptService *services.ScriptService) tea.Cmd {
	return func() tea.Msg {
		scripts, err := scriptService.FindAllScripts()
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
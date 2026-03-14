package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/entities"
	"scripto/internal/args"
	"scripto/internal/services"
)

const (
	ExitSuccess         = 0
	ExitError           = 1
	ExitBuiltinComplete = 3
	ExitExternalEditor  = 4
)

type (
	ScriptsLoadedMsg []*entities.Script
	ErrorMsg         error
	StatusMsg        string
)

type NavigateToScreenMsg struct {
	screen tea.Model
}

type NavigateBackMsg struct{}

type ExitAppMsg struct {
	exitCode int
	message  string
}

type ExecuteScriptMsg struct {
	script     *entities.Script
	scriptArgs []string
}

type SaveScriptMsg struct {
	script   *entities.Script
	command  string
	original *entities.Script
}

type EditScriptExternalMsg struct {
	script *entities.Script
}

type DeleteScriptMsg struct {
	script *entities.Script
}

type CopyScriptToClipboardMsg struct {
	script *entities.Script
}

type RefreshScriptsMsg struct{}

type ShowScriptEditorMsg struct {
	script *entities.Script
}

type ShowHistoryScreenMsg struct{}

type HistoryCommandSelectedMsg struct {
	command string
}

type ShowExecutionHistoryMsg struct {
	scriptID string
}

type PendingExecutionHistoryRecord struct {
	record services.ExecutionRecord
}

type ShowPlaceholderFormMsg struct {
	script         *entities.Script
	action         string
	placeholders   []args.PlaceholderValue
	originalScript string
}

type PlaceholderFormDoneMsg struct {
	values    map[string]string
	cancelled bool
}

type ScriptEditorResult struct {
	Script    *entities.Script
	Command   string
	Cancelled bool
}

func readScriptFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}


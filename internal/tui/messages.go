package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/entities"
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
	scriptPath string
}

type SaveScriptMsg struct {
	script   *entities.Script
	command  string
	original *entities.Script
}

type EditScriptExternalMsg struct {
	scriptPath string
}

type DeleteScriptMsg struct {
	script *entities.Script
}

type RefreshScriptsMsg struct{}

type ShowScriptEditorMsg struct {
	script *entities.Script
}

type ShowHistoryScreenMsg struct{}

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


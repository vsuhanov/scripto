package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/entities"
)

// ActionType represents the type of action a screen wants to perform
type ActionType int

const (
	// Navigation actions
	ActionNone ActionType = iota
	ActionNavigateBack
	ActionExitApp

	// Script actions
	ActionExecuteScript
	ActionEditScriptExternal
	ActionEditScriptInline
	ActionSaveScript
	ActionDeleteScript
	ActionRefreshScripts

	// Add flow actions
	ActionShowHistory
	ActionSelectFromHistory
	ActionCreateNewScript

	// Editor actions
	ActionShowScriptEditor
	ActionScriptEditorSave
	ActionScriptEditorCancel
)

// ScreenResult represents the result of a screen interaction
type ScreenResult struct {
	Action     ActionType
	Data       interface{}
	ShouldExit bool
	ExitCode   int
	Message    string
}

// Screen represents an embeddable screen component
type Screen interface {
	tea.Model
	GetResult() ScreenResult
	IsComplete() bool
	SetServices(services interface{})
}

// FlowController manages navigation and business logic for a flow
type FlowController interface {
	Run() (TUIResult, error)
	HandleScreenResult(result ScreenResult) error
}

// BaseFlowController provides common functionality for flow controllers
type BaseFlowController struct {
	currentScreen Screen
	width         int
	height        int
	exitCode      int
	shouldExit    bool
	exitMessage   string
}

// NewBaseFlowController creates a new base flow controller
func NewBaseFlowController() *BaseFlowController {
	return &BaseFlowController{
		width:  80,
		height: 24,
	}
}

// SetSize sets the terminal size for the flow controller
func (fc *BaseFlowController) SetSize(width, height int) {
	fc.width = width
	fc.height = height
}

// Exit sets the flow controller to exit with the given code
func (fc *BaseFlowController) Exit(code int, message string) {
	fc.shouldExit = true
	fc.exitCode = code
	fc.exitMessage = message
}

// ShouldExit returns whether the flow controller should exit
func (fc *BaseFlowController) ShouldExit() bool {
	return fc.shouldExit
}

// GetExitCode returns the exit code
func (fc *BaseFlowController) GetExitCode() int {
	return fc.exitCode
}

// GetExitMessage returns the exit message
func (fc *BaseFlowController) GetExitMessage() string {
	return fc.exitMessage
}

// SetCurrentScreen sets the current active screen
func (fc *BaseFlowController) SetCurrentScreen(screen Screen) {
	fc.currentScreen = screen
}

// GetCurrentScreen returns the current active screen
func (fc *BaseFlowController) GetCurrentScreen() Screen {
	return fc.currentScreen
}

// RunProgram runs a tea program with the current screen
func (fc *BaseFlowController) RunProgram() (tea.Model, error) {
	if fc.currentScreen == nil {
		return nil, fmt.Errorf("no current screen set")
	}

	program := tea.NewProgram(fc.currentScreen, tea.WithAltScreen())
	return program.Run()
}

// ActionData provides type-safe access to action data
type ActionData struct {
	Script     *entities.Script
	ScriptPath string
	Command    string
	Values     map[string]string
}

// ExtractActionData safely extracts typed data from ScreenResult
func ExtractActionData(result ScreenResult) *ActionData {
	switch data := result.Data.(type) {
	case *ActionData:
		return data
	case entities.Script:
		return &ActionData{Script: &data}
	case string:
		return &ActionData{ScriptPath: data}
	case map[string]string:
		return &ActionData{Values: data}
	default:
		return &ActionData{}
	}
}

// NewActionData creates a new ActionData with script
func NewActionDataWithScript(script entities.Script) *ActionData {
	return &ActionData{Script: &script}
}

// NewActionData creates a new ActionData with script path
func NewActionDataWithPath(path string) *ActionData {
	return &ActionData{ScriptPath: path}
}

// NewActionData creates a new ActionData with command
func NewActionDataWithCommand(command string) *ActionData {
	return &ActionData{Command: command}
}
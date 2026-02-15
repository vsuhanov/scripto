package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/entities"
	"scripto/internal/services"
)

type RootModel struct {
	scriptService    *services.ScriptService
	executionService *services.ExecutionService
	terminalService  *services.TerminalService
	currentScreen    tea.Model
	screenStack      []tea.Model
	width            int
	height           int
	shouldExit       bool
	exitCode         int
	exitMessage      string
}

func NewRootModel() (*RootModel, error) {
	scriptService, err := services.NewScriptService()
	if err != nil {
		return nil, fmt.Errorf("failed to create script service: %w", err)
	}

	mainListScreen, err := NewMainListScreen()
	if err != nil {
		return nil, fmt.Errorf("failed to create main list screen: %w", err)
	}

	mainListScreen.SetServices(scriptService)

	return &RootModel{
		scriptService:    scriptService,
		executionService: services.NewExecutionService(),
		terminalService:  services.NewTerminalService(),
		currentScreen:    mainListScreen,
		screenStack:      []tea.Model{},
		width:            80,
		height:           24,
	}, nil
}

func (m RootModel) Init() tea.Cmd {
	if model, ok := m.currentScreen.(tea.Model); ok {
		return model.Init()
	}
	return nil
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if model, ok := m.currentScreen.(tea.Model); ok {
			updatedModel, cmd := model.Update(msg)
			m.currentScreen = updatedModel
			return m, cmd
		}
		return m, nil

	case ExitAppMsg:
		m.shouldExit = true
		m.exitCode = msg.exitCode
		m.exitMessage = msg.message
		return m, tea.Quit

	case ExecuteScriptMsg:
		return m, m.handleExecuteScript(msg.scriptPath)

	case EditScriptExternalMsg:
		return m, m.handleEditScriptExternal(msg.scriptPath)

	case ShowScriptEditorMsg:
		scriptEditor := NewScriptEditorScreen(msg.script, false)
		scriptEditor.SetServices(m.scriptService)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = scriptEditor
		return m, scriptEditor.Init()

	case SaveScriptMsg:
		return m, m.handleSaveScript(msg.script, msg.command, msg.original)

	case ShowHistoryScreenMsg:
		historyScreen := NewHistoryScreen()
		historyScreen.SetServices(m.scriptService)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = historyScreen
		return m, historyScreen.Init()

	case NavigateBackMsg:
		if len(m.screenStack) > 0 {
			m.currentScreen = m.screenStack[len(m.screenStack)-1]
			m.screenStack = m.screenStack[:len(m.screenStack)-1]
			if model, ok := m.currentScreen.(tea.Model); ok {
				return m, model.Init()
			}
			return m, nil
		}
		m.shouldExit = true
		m.exitCode = int(services.ExitCodeBuiltinComplete)
		return m, tea.Quit

	case RefreshScriptsMsg:
		if mainList, ok := m.currentScreen.(*MainListScreen); ok {
			mainList.RefreshScripts()
		}
		if model, ok := m.currentScreen.(tea.Model); ok {
			return m, model.Init()
		}
		return m, nil

	default:
		if model, ok := m.currentScreen.(tea.Model); ok {
			updatedModel, cmd := model.Update(msg)
			m.currentScreen = updatedModel
			return m, cmd
		}
		return m, nil
	}
}

func (m RootModel) View() string {
	if model, ok := m.currentScreen.(tea.Model); ok {
		if v, ok := model.(interface{ View() string }); ok {
			return v.View()
		}
	}
	return ""
}

func (m *RootModel) handleExecuteScript(scriptPath string) tea.Cmd {
	return func() tea.Msg {
		if scriptPath == "" {
			return ErrorMsg(fmt.Errorf("no script path provided for execution"))
		}

		matchResult, err := m.scriptService.FindScriptByFilePath(scriptPath)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to find script: %w", err))
		}

		processingResult, err := m.executionService.ProcessScriptArguments(matchResult, []string{})
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to process script arguments: %w", err))
		}

		if !processingResult.NeedsPlaceholderForm {
			finalCommand, err := m.executionService.PrepareDirectExecution(processingResult)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to prepare script execution: %w", err))
			}
			if err := m.terminalService.ExecuteScript(finalCommand); err != nil {
				return ErrorMsg(err)
			}
		} else {
			formResult, err := RunPlaceholderForm(processingResult.Placeholders)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to collect placeholder values: %w", err))
			}

			if formResult.Cancelled {
				return ErrorMsg(fmt.Errorf("operation cancelled by user"))
			}

			finalCommand, err := m.executionService.PrepareExecution(matchResult, []string{}, formResult.Values)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to prepare script execution: %w", err))
			}
			if err := m.terminalService.ExecuteScript(finalCommand); err != nil {
				return ErrorMsg(err)
			}
		}

		return ExitAppMsg{exitCode: int(services.ExitCodeSuccess), message: scriptPath}
	}
}

func (m *RootModel) handleEditScriptExternal(scriptPath string) tea.Cmd {
	return func() tea.Msg {
		if scriptPath == "" {
			return ErrorMsg(fmt.Errorf("no script path provided for external edit"))
		}

		if err := m.terminalService.EditScriptExternal(scriptPath); err != nil {
			return ErrorMsg(err)
		}

		return ExitAppMsg{exitCode: int(services.ExitCodeExternalEditor), message: scriptPath}
	}
}

func (m *RootModel) handleSaveScript(script entities.Script, command string, original *entities.Script) tea.Cmd {
	return func() tea.Msg {
		if err := m.scriptService.SaveScript(script, command, original); err != nil {
			return ErrorMsg(fmt.Errorf("error saving script: %w", err))
		}

		return NavigateBackMsg{}
	}
}


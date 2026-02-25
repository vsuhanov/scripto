package tui

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/entities"
	"scripto/internal/services"
)

type RootModel struct {
	container      *services.Container
	currentScreen  tea.Model
	screenStack    []tea.Model
	width          int
	height         int
	pendingCommand services.TerminalServiceCommand
}

type StartMode int

const (
	StartAtMainList StartMode = iota
	StartAtAdd
)

type ExecuteAppCommandMsg struct {
	command services.TerminalServiceCommand
}

func NewRootModel(container *services.Container, startMode StartMode) (*RootModel, error) {
	var initialScreen tea.Model
	var err error

	if startMode == StartAtAdd {
		initialScreen = NewHistoryScreen(container)
	} else {
		initialScreen, err = NewMainListScreen(container)
		if err != nil {
			return nil, fmt.Errorf("failed to create main list screen: %w", err)
		}
	}

	return &RootModel{
		container:     container,
		currentScreen: initialScreen,
		screenStack:   []tea.Model{},
		width:         80,
		height:        24,
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
		log.Printf("RootModel WindowSize - Width: %d, Height: %d", msg.Width, msg.Height)
		if model, ok := m.currentScreen.(tea.Model); ok {
			updatedScreen, cmd := model.Update(msg)
			m.currentScreen = updatedScreen
			return m, cmd
		}
		return m, nil

	case ExitAppMsg:
		return m, func() tea.Msg {
			return ExecuteAppCommandMsg{
				command: m.container.TerminalService.PrepareExit(msg.exitCode),
			}
		}

	case ExecuteAppCommandMsg:
		m.pendingCommand = msg.command
		return m, tea.Quit

	case ExecuteScriptMsg:
		return m, m.handleExecuteScript(msg.scriptPath)

	case EditScriptExternalMsg:
		return m, m.handleEditScriptExternal(msg.scriptPath)

	case ShowScriptEditorMsg:
		scriptEditor := NewScriptEditorScreen(msg.script, false, m.container)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = scriptEditor
		return m, scriptEditor.Init()

	case SaveScriptMsg:
		return m, m.handleSaveScript(msg.script, msg.command, msg.original)

	case ShowHistoryScreenMsg:
		historyScreen := NewHistoryScreen(m.container)
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
		m.pendingCommand = m.container.TerminalService.PrepareExit(ExitBuiltinComplete)
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

		matchResult, err := m.container.ScriptService.FindScriptByFilePath(scriptPath)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to find script: %w", err))
		}

		processingResult, err := m.container.ExecutionService.ProcessScriptArguments(matchResult, []string{})
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to process script arguments: %w", err))
		}

		var finalCommand string
		if !processingResult.NeedsPlaceholderForm {
			var err error
			finalCommand, err = m.container.ExecutionService.PrepareDirectExecution(processingResult)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to prepare script execution: %w", err))
			}
		} else {
			formResult, err := RunPlaceholderForm(processingResult.Placeholders)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to collect placeholder values: %w", err))
			}

			if formResult.Cancelled {
				return ErrorMsg(fmt.Errorf("operation cancelled by user"))
			}

			finalCommand, err = m.container.ExecutionService.PrepareExecution(matchResult, []string{}, formResult.Values)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to prepare script execution: %w", err))
			}
		}

		return ExecuteAppCommandMsg{
			command: m.container.TerminalService.PrepareScriptExecution(finalCommand),
		}
	}
}

func (m *RootModel) handleEditScriptExternal(scriptPath string) tea.Cmd {
	return func() tea.Msg {
		if scriptPath == "" {
			return ErrorMsg(fmt.Errorf("no script path provided for external edit"))
		}

		return ExecuteAppCommandMsg{
			command: m.container.TerminalService.PrepareExternalEditing(scriptPath),
		}
	}
}

func (m *RootModel) handleSaveScript(script *entities.Script, command string, original *entities.Script) tea.Cmd {
	return func() tea.Msg {
		if err := m.container.ScriptService.SaveScript(script, command, original); err != nil {
			return ErrorMsg(fmt.Errorf("error saving script: %w", err))
		}

		return NavigateBackMsg{}
	}
}

func (m *RootModel) GetPendingCommand() services.TerminalServiceCommand {
	return m.pendingCommand
}

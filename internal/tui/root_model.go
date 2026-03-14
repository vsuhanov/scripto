package tui

import (
	"fmt"
	"log"
	"os"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"scripto/entities"
	"scripto/internal/services"
)

type TuiRequest interface {
	tuiRequest()
}

type ShowMainListRequest struct{}

func (ShowMainListRequest) tuiRequest() {}

type ShowAddScreenRequest struct{}

func (ShowAddScreenRequest) tuiRequest() {}

type ExecuteScriptRequest struct {
	Script     *entities.Script
	ScriptArgs []string
}

func (ExecuteScriptRequest) tuiRequest() {}

type RootModel struct {
	container               *services.Container
	currentScreen           tea.Model
	screenStack             []tea.Model
	width                   int
	height                  int
	pendingCommand          services.TerminalServiceCommand
	pendingHistoryRecord    *services.ExecutionRecord
	initialRequest          TuiRequest
	pendingPlaceholderScript *entities.Script
	pendingPlaceholderAction string
	pendingPlaceholderOriginalScript string
}

type ExecuteAppCommandMsg struct {
	command       services.TerminalServiceCommand
	historyRecord *services.ExecutionRecord
}

func NewRootModel(container *services.Container, request TuiRequest) (*RootModel, error) {
	var initialScreen tea.Model
	var err error

	switch request.(type) {
	case ShowAddScreenRequest:
		initialScreen = NewHistoryScreen(container)
	default:
		initialScreen, err = NewMainListScreen(container)
		if err != nil {
			return nil, fmt.Errorf("failed to create main list screen: %w", err)
		}
	}

	return &RootModel{
		container:      container,
		currentScreen:  initialScreen,
		screenStack:    []tea.Model{},
		width:          80,
		height:         24,
		initialRequest: request,
	}, nil
}

func (m RootModel) Init() tea.Cmd {
	screenInit := m.currentScreen.Init()
	if req, ok := m.initialRequest.(ExecuteScriptRequest); ok {
		return tea.Batch(screenInit, func() tea.Msg {
			return ExecuteScriptMsg{script: req.Script, scriptArgs: req.ScriptArgs}
		})
	}
	return screenInit
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
		m.pendingHistoryRecord = msg.historyRecord
		return m, tea.Quit

	case ExecuteScriptMsg:
		return m, m.handleExecuteScript(msg.script, msg.scriptArgs)

	case CopyScriptToClipboardMsg:
		return m, m.handleCopyScriptToClipboard(msg.script)

	case ShowPlaceholderFormMsg:
		m.pendingPlaceholderScript = msg.script
		m.pendingPlaceholderAction = msg.action
		m.pendingPlaceholderOriginalScript = msg.originalScript
		form := NewPlaceholderForm(msg.script, msg.placeholders, m.width, m.height)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = form
		return m, form.Init()

	case PlaceholderFormDoneMsg:
		if len(m.screenStack) > 0 {
			m.currentScreen = m.screenStack[len(m.screenStack)-1]
			m.screenStack = m.screenStack[:len(m.screenStack)-1]
		}
		if msg.cancelled {
			m.pendingPlaceholderScript = nil
			m.pendingPlaceholderAction = ""
			m.pendingPlaceholderOriginalScript = ""
			return m, func() tea.Msg { return StatusMsg("Cancelled") }
		}
		script := m.pendingPlaceholderScript
		action := m.pendingPlaceholderAction
		originalScript := m.pendingPlaceholderOriginalScript
		m.pendingPlaceholderScript = nil
		m.pendingPlaceholderAction = ""
		m.pendingPlaceholderOriginalScript = ""
		if action == "execute" {
			return m, m.finalizeExecute(script, msg.values, originalScript)
		}
		return m, m.finalizeCopy(script, msg.values)

	case EditScriptExternalMsg:
		return m, m.handleEditScriptExternal(msg.script)

	case ShowScriptEditorMsg:
		scriptEditor := NewScriptEditorScreen(msg.script, false, m.container)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = scriptEditor
		return m, scriptEditor.Init()

	case SaveScriptMsg:
		return m, m.handleSaveScript(msg.script, msg.command, msg.original)

	case HistoryCommandSelectedMsg:
		script := &entities.Script{}
		editor := NewScriptEditorScreen(script, true, m.container)
		editor.initialCommand = msg.command
		m.currentScreen = editor
		return m, editor.Init()

	case ShowHistoryScreenMsg:
		historyScreen := NewHistoryScreen(m.container)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = historyScreen
		return m, historyScreen.Init()

	case ShowExecutionHistoryMsg:
		execHistoryScreen := NewExecutionHistoryScreen(m.container, msg.scriptID, m.width, m.height)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = execHistoryScreen
		return m, execHistoryScreen.Init()

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

func (m *RootModel) handleExecuteScript(script *entities.Script, scriptArgs []string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("handleExecuteScript: scriptID=%q scriptName=%q", script.ID, script.Name)
		processingResult, err := m.container.ExecutionService.ProcessScriptArguments(script, scriptArgs)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to process script arguments: %w", err))
		}

		log.Printf("handleExecuteScript: NeedsPlaceholderForm=%v", processingResult.NeedsPlaceholderForm)
		if !processingResult.NeedsPlaceholderForm {
			finalCommand, err := m.container.ExecutionService.PrepareDirectExecution(processingResult)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to prepare script execution: %w", err))
			}
			record := m.buildHistoryRecord(script, finalCommand, processingResult.OriginalScript, nil)
			log.Printf("handleExecuteScript: historyRecord=%v", record != nil)
			return ExecuteAppCommandMsg{
				command:       m.container.TerminalService.PrepareScriptExecution(finalCommand),
				historyRecord: record,
			}
		}

		return ShowPlaceholderFormMsg{script: script, action: "execute", placeholders: processingResult.Placeholders, originalScript: processingResult.OriginalScript}
	}
}

func (m *RootModel) handleCopyScriptToClipboard(script *entities.Script) tea.Cmd {
	return func() tea.Msg {
		processingResult, err := m.container.ExecutionService.ProcessScriptArguments(script, []string{})
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to process script arguments: %w", err))
		}

		if !processingResult.NeedsPlaceholderForm {
			finalCommand, err := m.container.ExecutionService.PrepareDirectExecution(processingResult)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to prepare command: %w", err))
			}
			_ = clipboard.WriteAll(finalCommand)
			return StatusMsg("Copied to clipboard")
		}

		return ShowPlaceholderFormMsg{script: script, action: "copy", placeholders: processingResult.Placeholders}
	}
}

func (m *RootModel) finalizeExecute(script *entities.Script, values map[string]string, originalScript string) tea.Cmd {
	return func() tea.Msg {
		finalCommand, err := m.container.ExecutionService.PrepareExecution(script, []string{}, values)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to prepare script execution: %w", err))
		}
		record := m.buildHistoryRecord(script, finalCommand, originalScript, values)
		return ExecuteAppCommandMsg{command: m.container.TerminalService.PrepareScriptExecution(finalCommand), historyRecord: record}
	}
}

func (m *RootModel) finalizeCopy(script *entities.Script, values map[string]string) tea.Cmd {
	return func() tea.Msg {
		finalCommand, err := m.container.ExecutionService.PrepareExecution(script, []string{}, values)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to prepare command: %w", err))
		}
		_ = clipboard.WriteAll(finalCommand)
		return StatusMsg("Copied to clipboard")
	}
}

func (m *RootModel) handleEditScriptExternal(script *entities.Script) tea.Cmd {
	return func() tea.Msg {
		if script.FilePath == "" {
			return ErrorMsg(fmt.Errorf("no script path provided for external edit"))
		}

		return ExecuteAppCommandMsg{
			command: m.container.TerminalService.PrepareExternalEditing(script.FilePath),
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

func (m *RootModel) GetPendingHistoryRecord() *services.ExecutionRecord {
	return m.pendingHistoryRecord
}

func (m *RootModel) buildHistoryRecord(script *entities.Script, executedScript, originalScript string, placeholderValues map[string]string) *services.ExecutionRecord {
	log.Printf("buildHistoryRecord: scriptID=%q scriptName=%q executedScript=%q", script.ID, script.Name, executedScript)
	if script.ID == "" {
		log.Printf("buildHistoryRecord: script.ID is empty, skipping history record (run --migrate to assign IDs)")
		return nil
	}
	cwd, _ := os.Getwd()
	if placeholderValues == nil {
		placeholderValues = map[string]string{}
	}
	record := services.BuildExecutionRecord(script, executedScript, originalScript, placeholderValues, cwd)
	log.Printf("buildHistoryRecord: built record id=%q scriptID=%q", record.ID, record.ScriptID)
	return &record
}

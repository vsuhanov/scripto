package tui

import (
	"fmt"
	"log"
	"os"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/services"
	"github.com/vsuhanov/scripto/internal/utils"
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

type ShowScriptEditorRequest struct {
	Script         *entities.Script
	InitialCommand string
}

func (ShowScriptEditorRequest) tuiRequest() {}

type ShowMainListWithSearchRequest struct {
	SearchText string
}

func (ShowMainListWithSearchRequest) tuiRequest() {}

type RootModel struct {
	container            *services.Container
	currentScreen        tea.Model
	screenStack          []tea.Model
	width                int
	height               int
	pendingCommand       services.TerminalServiceCommand
	pendingHistoryRecord *services.ExecutionRecord
	initialRequest       TuiRequest
	pendingSavedScript   *entities.Script
	pendingSavedCommand  string
}

type ExecuteAppCommandMsg struct {
	command       services.TerminalServiceCommand
	historyRecord *services.ExecutionRecord
}

func NewRootModel(container *services.Container, request TuiRequest) (*RootModel, error) {
	var initialScreen tea.Model
	var err error

	switch req := request.(type) {
	case ShowAddScreenRequest:
		initialScreen = NewHistoryScreen(container)
	case ShowScriptEditorRequest:
		editor := NewScriptEditorScreen(req.Script, true, container)
		editor.initialCommand = req.InitialCommand
		initialScreen = editor
	case ShowMainListWithSearchRequest:
		mainList, err := NewMainListScreen(container)
		if err != nil {
			return nil, fmt.Errorf("failed to create main list screen: %w", err)
		}
		mainList.initialSearch = req.SearchText
		initialScreen = mainList
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
			return ExecuteScriptMsg{script: req.Script, scriptArgs: req.ScriptArgs, fromCLI: true}
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
		realScope := msg.script.Scope
		if msg.script.OriginalScope != "" {
			realScope = msg.script.OriginalScope
		}
		scopeType := getScopeType(realScope)
		if scopeType == "local" || scopeType == "global" {
			return m, m.handleExecuteScript(msg.script, msg.scriptArgs, !msg.fromCLI)
		}
		return m, m.showExecutionForm(msg.script, msg.scriptArgs, msg.fromCLI)

	case ShowScriptExecutionWithWorkingDirMsg:
		realScope := msg.script.Scope
		if msg.script.OriginalScope != "" {
			realScope = msg.script.OriginalScope
		}
		if getScopeType(realScope) == "global" {
			return m, m.handleExecuteScript(msg.script, msg.scriptArgs, true)
		}
		return m, m.showExecutionForm(msg.script, msg.scriptArgs, false)

	case CopyScriptToClipboardMsg:
		return m, m.handleCopyScriptToClipboard(msg.script)

	case ShowRawCommandExecutionMsg:
		return m, func() tea.Msg {
			return ShowPlaceholderFormMsg{
				action:     "execute",
				workingDir: msg.workingDir,
				rawCommand: msg.command,
			}
		}

	case ShowPlaceholderFormMsg:
		form := NewPlaceholderForm(msg.script, msg.action, msg.placeholders, m.width, m.height, m.container, msg.originalScript, msg.workingDir, msg.rawCommand)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = form
		return m, form.Init()

	case PlaceholderFormDoneMsg:
		if len(m.screenStack) > 0 {
			m.currentScreen = m.screenStack[len(m.screenStack)-1]
			m.screenStack = m.screenStack[:len(m.screenStack)-1]
		}
		if msg.cancelled {
			return m, func() tea.Msg { return StatusMsg("Cancelled") }
		}
		if msg.script == nil {
			return m, m.finalizeRawExecute(msg.rawCommand, msg.commandOverride, msg.workingDir)
		}
		if msg.action == "execute" {
			return m, m.finalizeExecute(msg.script, msg.values, msg.originalScript, msg.workingDir, msg.commandOverride)
		}
		return m, m.finalizeCopy(msg.script, msg.values, msg.originalScript)

	case DeleteScriptMsg:
		return m, m.handleDeleteScript(msg.script)

	case ArchiveScriptMsg:
		return m, m.handleArchiveScript(msg.script)

	case UnarchiveScriptMsg:
		return m, m.handleUnarchiveScript(msg.script)

	case EditScriptExternalMsg:
		return m, m.handleEditScriptExternal(msg.script)

	case ShowScopeChoiceForEditMsg:
		choiceScreen := NewScopeEditChoiceScreen(msg.script, m.container)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = choiceScreen
		return m, choiceScreen.Init()

	case ShowScriptEditorMsg:
		scriptEditor := NewScriptEditorScreen(msg.script, msg.isNewScript, m.container)
		if msg.initialCommand != "" {
			scriptEditor.initialCommand = msg.initialCommand
		}
		if _, isChoice := m.currentScreen.(*ScopeEditChoiceScreen); !isChoice {
			m.screenStack = append(m.screenStack, m.currentScreen)
		}
		m.currentScreen = scriptEditor
		return m, scriptEditor.Init()

	case SaveScriptMsg:
		return m, m.handleSaveScript(msg.script, msg.command, msg.original)

	case ScriptSavedMsg:
		if len(m.screenStack) > 0 {
			m.currentScreen = m.screenStack[len(m.screenStack)-1]
			m.screenStack = m.screenStack[:len(m.screenStack)-1]
			if mainList, ok := m.currentScreen.(*MainListScreen); ok {
				mainList.RefreshScripts()
			}
			var markCmd tea.Cmd
			if shellHistory, ok := m.currentScreen.(*ShellHistoryScreen); ok && msg.script != nil {
				markCmd = shellHistory.MarkPendingSaved(msg.script.ID)
			}
			if model, ok := m.currentScreen.(tea.Model); ok {
				return m, tea.Sequence(markCmd, model.Init())
			}
			return m, markCmd
		}
		m.pendingSavedScript = msg.script
		m.pendingSavedCommand = msg.command
		return m, tea.Quit

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

	case ShowShellHistoryMsg:
		shellHistoryScreen := NewShellHistoryScreen(m.container, m.width, m.height)
		m.screenStack = append(m.screenStack, m.currentScreen)
		m.currentScreen = shellHistoryScreen
		return m, shellHistoryScreen.Init()

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

	case CdToDirectoryMsg:
		dir := msg.dir
		return m, func() tea.Msg {
			return ExecuteAppCommandMsg{
				command: m.container.TerminalService.PrepareScriptExecution("cd "+utils.ShellQuote(dir), "", nil, "", false),
			}
		}

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

func scriptDefaultWorkingDir(script *entities.Script) string {
	cwd, _ := os.Getwd()
	if script.OriginalScope != "" {
		return cwd
	}
	if services.IsPatternScope(script.Scope) {
		return cwd
	}
	if script.Scope != "" && script.Scope != "global" && script.Scope != cwd {
		return script.Scope
	}
	return cwd
}

func withWorkingDir(command, workingDir string) string {
	cwd, _ := os.Getwd()
	if workingDir != "" && workingDir != cwd {
		return "cd " + utils.ShellQuote(workingDir) + " && " + command
	}
	return command
}

func (m *RootModel) showExecutionForm(script *entities.Script, scriptArgs []string, fromCLI bool) tea.Cmd {
	return func() tea.Msg {
		workingDirFromArgs, filteredArgs := services.ExtractWorkingDirArg(scriptArgs)
		processingResult, err := m.container.ExecutionService.ProcessScriptArguments(script, filteredArgs)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to process script arguments: %w", err))
		}

		workingDir := scriptDefaultWorkingDir(script)
		if workingDirFromArgs != "" {
			workingDir = workingDirFromArgs
		}

		canSkip := workingDirFromArgs != "" || script.OriginalScope != ""
		if !processingResult.NeedsPlaceholderForm && canSkip {
			finalCommand := withWorkingDir(processingResult.FinalCommand, workingDir)
			record := m.buildHistoryRecord(script, finalCommand, processingResult.OriginalScript, processingResult.ParsedValues, workingDir)
			return ExecuteAppCommandMsg{
				command:       m.container.TerminalService.PrepareScriptExecution(finalCommand, script.Name, processingResult.ParsedValues, workingDir, !fromCLI),
				historyRecord: record,
			}
		}

		return ShowPlaceholderFormMsg{
			script:         script,
			action:         "execute",
			placeholders:   processingResult.Metas,
			originalScript: processingResult.OriginalScript,
			workingDir:     workingDir,
		}
	}
}

func (m *RootModel) handleExecuteScript(script *entities.Script, scriptArgs []string, writeHistory bool) tea.Cmd {
	return func() tea.Msg {
		workingDir, filteredArgs := services.ExtractWorkingDirArg(scriptArgs)
		processingResult, err := m.container.ExecutionService.ProcessScriptArguments(script, filteredArgs)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to process script arguments: %w", err))
		}

		if processingResult.NeedsPlaceholderForm {
			return ShowPlaceholderFormMsg{script: script, action: "execute", placeholders: processingResult.Metas, originalScript: processingResult.OriginalScript, workingDir: workingDir}
		}

		finalCommand := withWorkingDir(processingResult.FinalCommand, workingDir)
		record := m.buildHistoryRecord(script, finalCommand, processingResult.OriginalScript, processingResult.ParsedValues, workingDir)
		return ExecuteAppCommandMsg{
			command:       m.container.TerminalService.PrepareScriptExecution(finalCommand, script.Name, processingResult.ParsedValues, workingDir, writeHistory),
			historyRecord: record,
		}
	}
}

func (m *RootModel) handleCopyScriptToClipboard(script *entities.Script) tea.Cmd {
	return func() tea.Msg {
		processingResult, err := m.container.ExecutionService.ProcessScriptArguments(script, []string{})
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to process script arguments: %w", err))
		}

		if !processingResult.NeedsPlaceholderForm {
			_ = clipboard.WriteAll(processingResult.FinalCommand)
			return StatusMsg("Copied to clipboard")
		}

		return ShowPlaceholderFormMsg{script: script, action: "copy", placeholders: processingResult.Metas, originalScript: processingResult.OriginalScript}
	}
}

func (m *RootModel) renderScript(script *entities.Script, originalScript string, values map[string]string) (string, error) {
	if originalScript != "" {
		return m.container.ExecutionService.Render(originalScript, values)
	}
	return m.container.ExecutionService.PrepareExecution(script, values)
}

func (m *RootModel) finalizeExecute(script *entities.Script, values map[string]string, originalScript string, workingDir string, commandOverride string) tea.Cmd {
	return func() tea.Msg {
		finalCommand := commandOverride
		if finalCommand == "" {
			var err error
			finalCommand, err = m.renderScript(script, originalScript, values)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to prepare script execution: %w", err))
			}
		}
		finalCommand = withWorkingDir(finalCommand, workingDir)
		record := m.buildHistoryRecord(script, finalCommand, originalScript, values, workingDir)
		return ExecuteAppCommandMsg{command: m.container.TerminalService.PrepareScriptExecution(finalCommand, script.Name, values, workingDir, true), historyRecord: record}
	}
}

func (m *RootModel) finalizeRawExecute(rawCommand string, commandOverride string, workingDir string) tea.Cmd {
	return func() tea.Msg {
		finalCommand := rawCommand
		if commandOverride != "" {
			finalCommand = commandOverride
		}
		finalCommand = withWorkingDir(finalCommand, workingDir)
		cwd, _ := os.Getwd()
		if workingDir == "" {
			workingDir = cwd
		}
		return ExecuteAppCommandMsg{
			command:       m.container.TerminalService.PrepareScriptExecution(finalCommand, "", nil, workingDir, true),
			historyRecord: &services.ExecutionRecord{ExecutedScript: finalCommand, WorkingDirectory: workingDir, InvokedFrom: cwd},
		}
	}
}

func (m *RootModel) finalizeCopy(script *entities.Script, values map[string]string, originalScript string) tea.Cmd {
	return func() tea.Msg {
		finalCommand, err := m.renderScript(script, originalScript, values)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to prepare command: %w", err))
		}
		_ = clipboard.WriteAll(finalCommand)
		return StatusMsg("Copied to clipboard")
	}
}

func (m *RootModel) handleDeleteScript(script *entities.Script) tea.Cmd {
	return func() tea.Msg {
		if err := m.container.ScriptService.DeleteScript(script); err != nil {
			return ErrorMsg(fmt.Errorf("error deleting script: %w", err))
		}
		return ScriptDeletedMsg{script: script}
	}
}

func (m *RootModel) handleArchiveScript(script *entities.Script) tea.Cmd {
	return func() tea.Msg {
		if err := m.container.ScriptService.ArchiveScript(script); err != nil {
			return ErrorMsg(fmt.Errorf("error archiving script: %w", err))
		}
		return ScriptArchivedMsg{script: script}
	}
}

func (m *RootModel) handleUnarchiveScript(script *entities.Script) tea.Cmd {
	return func() tea.Msg {
		if err := m.container.ScriptService.UnarchiveScript(script); err != nil {
			return ErrorMsg(fmt.Errorf("error unarchiving script: %w", err))
		}
		return ScriptUnarchivedMsg{script: script}
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

		return ScriptSavedMsg{script: script, command: command}
	}
}

func (m *RootModel) GetPendingCommand() services.TerminalServiceCommand {
	return m.pendingCommand
}

func (m *RootModel) GetPendingHistoryRecord() *services.ExecutionRecord {
	return m.pendingHistoryRecord
}

func (m *RootModel) GetPendingSavedScript() *entities.Script {
	return m.pendingSavedScript
}

func (m *RootModel) GetPendingSavedCommand() string {
	return m.pendingSavedCommand
}

func (m *RootModel) buildHistoryRecord(script *entities.Script, executedScript, originalScript string, placeholderValues map[string]string, workingDir string) *services.ExecutionRecord {
	if script.ID == "" {
		log.Printf("buildHistoryRecord: script.ID is empty, skipping history record (run --migrate to assign IDs)")
		return nil
	}
	cwd, _ := os.Getwd()
	if workingDir == "" {
		workingDir = cwd
	}
	if placeholderValues == nil {
		placeholderValues = map[string]string{}
	}
	record := services.BuildExecutionRecord(script, executedScript, originalScript, placeholderValues, workingDir, cwd)
	return &record
}

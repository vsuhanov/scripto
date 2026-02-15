package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"scripto/entities"
	"scripto/internal/execution"
	"scripto/internal/script"
	"scripto/internal/services"
	"scripto/internal/storage"
)

type RootModel struct {
	scriptService *services.ScriptService
	currentScreen tea.Model
	screenStack   []tea.Model
	width         int
	height        int
	shouldExit    bool
	exitCode      int
	exitMessage   string
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

	m := &RootModel{
		scriptService: scriptService,
		currentScreen: mainListScreen,
		screenStack:   []tea.Model{},
		width:         80,
		height:        24,
	}

	return m, nil
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
		m.exitCode = 3
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

		configPath, err := storage.GetConfigPath()
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to get config path: %w", err))
		}

		config, err := storage.ReadConfig(configPath)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to read config: %w", err))
		}

		matchResult, err := m.findScriptByFilePath(config, scriptPath)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to find script: %w", err))
		}

		if err := m.executeFoundScript(matchResult, []string{}); err != nil {
			return ErrorMsg(fmt.Errorf("error executing script: %w", err))
		}

		return ExitAppMsg{exitCode: 0, message: scriptPath}
	}
}

func (m *RootModel) handleEditScriptExternal(scriptPath string) tea.Cmd {
	return func() tea.Msg {
		if scriptPath == "" {
			return ErrorMsg(fmt.Errorf("no script path provided for external edit"))
		}

		if err := m.writeScriptPathForEditor(scriptPath); err != nil {
			return ErrorMsg(err)
		}

		return ExitAppMsg{exitCode: 4, message: scriptPath}
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

func (m *RootModel) findScriptByFilePath(config storage.Config, filePath string) (*script.MatchResult, error) {
	for scope, scripts := range config {
		for _, scriptEntity := range scripts {
			if scriptEntity.FilePath == filePath {
				matchResult := &script.MatchResult{
					Type:       script.ExactName,
					Script:     scriptEntity,
					Confidence: 1.0,
				}
				matchResult.Script.Scope = scope
				return matchResult, nil
			}
		}
	}
	return nil, fmt.Errorf("script not found with file path: %s", filePath)
}

func (m *RootModel) executeFoundScript(matchResult *script.MatchResult, scriptArgs []string) error {
	executor := execution.NewScriptExecutor()

	processingResult, err := executor.ProcessScriptArguments(matchResult, scriptArgs)
	if err != nil {
		return err
	}

	if !processingResult.NeedsPlaceholderForm {
		return executor.ExecuteScriptDirect(processingResult.FinalCommand)
	}

	formResult, err := RunPlaceholderForm(processingResult.Placeholders)
	if err != nil {
		return fmt.Errorf("failed to collect placeholder values: %w", err)
	}

	if formResult.Cancelled {
		return fmt.Errorf("operation cancelled by user")
	}

	return executor.ExecuteScriptWithPlaceholders(matchResult, scriptArgs, formResult.Values)
}

func (m *RootModel) writeScriptPathForEditor(scriptPath string) error {
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		return execution.WriteScriptPathToFile(scriptPath, cmdFdPath)
	}

	fmt.Print(scriptPath)
	return nil
}

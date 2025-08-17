package tui

import (
	"fmt"

	"scripto/entities"
	"scripto/internal/services"
)

// AddFlowController manages the add script flow
type AddFlowController struct {
	*BaseFlowController
	scriptService *services.ScriptService
	
	// Flow configuration
	initialCommand  string
	initialFilePath string
	scriptName      string
	description     string
	isGlobal        bool
	skipHistory     bool
	
	// Screens
	historyScreen *HistoryScreen
	scriptEditor  *ScriptEditorScreen
	
	// Flow state
	selectedCommand string
}

// AddFlowOptions configures the add flow
type AddFlowOptions struct {
	Command     string
	FilePath    string
	Name        string
	Description string
	IsGlobal    bool
	SkipHistory bool
}

// NewAddFlowController creates a new add flow controller
func NewAddFlowController(options AddFlowOptions) (*AddFlowController, error) {
	scriptService, err := services.NewScriptService()
	if err != nil {
		return nil, fmt.Errorf("failed to create script service: %w", err)
	}

	fc := &AddFlowController{
		BaseFlowController: NewBaseFlowController(),
		scriptService:      scriptService,
		initialCommand:     options.Command,
		initialFilePath:    options.FilePath,
		scriptName:         options.Name,
		description:        options.Description,
		isGlobal:           options.IsGlobal,
		skipHistory:        options.SkipHistory,
	}

	// Determine starting screen
	if options.SkipHistory || options.Command != "" || options.FilePath != "" {
		// Skip history and go directly to script editor
		fc.selectedCommand = options.Command
		if err := fc.showScriptEditor(); err != nil {
			return nil, fmt.Errorf("failed to initialize script editor: %w", err)
		}
	} else {
		// Start with history screen
		if err := fc.showHistoryScreen(); err != nil {
			return nil, fmt.Errorf("failed to initialize history screen: %w", err)
		}
	}

	return fc, nil
}

// Run starts the add flow
func (fc *AddFlowController) Run() (TUIResult, error) {
	for !fc.ShouldExit() {
		// Run the current screen
		finalModel, err := fc.RunProgram()
		if err != nil {
			return TUIResult{ExitCode: 1}, fmt.Errorf("TUI error: %w", err)
		}

		// Get result from the screen
		var result ScreenResult
		if screen, ok := finalModel.(Screen); ok {
			result = screen.GetResult()
		} else {
			// If screen doesn't implement Screen interface, assume normal quit
			fc.Exit(3, "Normal quit")
			break
		}

		// Handle the screen result
		if err := fc.HandleScreenResult(result); err != nil {
			return TUIResult{ExitCode: 1}, err
		}

		// Check if result indicates exit
		if result.ShouldExit {
			fc.Exit(result.ExitCode, result.Message)
		}
	}

	return TUIResult{
		ExitCode: fc.GetExitCode(),
	}, nil
}

// HandleScreenResult processes screen results and performs actions
func (fc *AddFlowController) HandleScreenResult(result ScreenResult) error {
	actionData := ExtractActionData(result)

	switch result.Action {
	case ActionNavigateBack:
		fc.Exit(3, "Cancelled")
		return nil

	case ActionSelectFromHistory:
		fc.selectedCommand = actionData.Command
		return fc.showScriptEditor()

	case ActionScriptEditorSave:
		return fc.handleScriptEditorSave()

	case ActionScriptEditorCancel:
		if fc.historyScreen != nil {
			// Return to history screen
			return fc.showHistoryScreen()
		} else {
			// Exit if no history screen
			fc.Exit(3, "Cancelled")
		}
		return nil

	default:
		// Unknown action, continue
		return nil
	}
}

// showHistoryScreen shows the history selection screen
func (fc *AddFlowController) showHistoryScreen() error {
	fc.historyScreen = NewHistoryScreen()
	fc.SetCurrentScreen(fc.historyScreen)
	return nil
}

// showScriptEditor shows the script editor
func (fc *AddFlowController) showScriptEditor() error {
	// Create script with initial values
	script := fc.createScript()
	
	fc.scriptEditor = NewScriptEditorScreen(script, true)
	fc.scriptEditor.SetServices(fc.scriptService)
	fc.SetCurrentScreen(fc.scriptEditor)
	return nil
}

// createScript creates a script entity with the configured values
func (fc *AddFlowController) createScript() entities.Script {
	script := entities.Script{
		Name:        fc.scriptName,
		Description: fc.description,
	}

	// Set scope
	if fc.isGlobal {
		script.Scope = "global"
	} else {
		script.Scope = fc.scriptService.GetCurrentDirectoryScope()
	}

	// Handle file path or create temp file for command
	if fc.initialFilePath != "" {
		script.FilePath = fc.initialFilePath
	} else if fc.selectedCommand != "" || fc.initialCommand != "" {
		command := fc.selectedCommand
		if command == "" {
			command = fc.initialCommand
		}
		
		// Create a temporary script file for the command
		if tempFilePath, err := fc.scriptService.CreateTempScriptFile(command); err == nil {
			script.FilePath = tempFilePath
		}
	}

	return script
}

// handleScriptEditorSave processes script editor save
func (fc *AddFlowController) handleScriptEditorSave() error {
	if fc.scriptEditor == nil {
		return fmt.Errorf("no script editor active")
	}

	editorResult := fc.scriptEditor.GetEditorResult()
	if editorResult.Cancelled {
		return fc.HandleScreenResult(ScreenResult{Action: ActionScriptEditorCancel})
	}

	// Use the command from the editor (whatever the user typed/edited)
	finalCommand := editorResult.Command

	// Save the script
	if err := fc.scriptService.SaveScript(editorResult.Script, finalCommand, nil); err != nil {
		// Show error and stay in editor
		fc.scriptEditor.SetErrorMessage(fmt.Sprintf("Error saving: %v", err))
		return nil
	}

	// Successfully saved, exit
	fc.Exit(0, "Script added successfully")
	return nil
}

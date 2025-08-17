package tui

import (
	"fmt"
	"os"

	"scripto/entities"
	"scripto/internal/execution"
	"scripto/internal/script"
	"scripto/internal/services"
	"scripto/internal/storage"
)

// RootFlowController manages the main application flow
type RootFlowController struct {
	*BaseFlowController
	scriptService *services.ScriptService
	mainListScreen *MainListScreen
	scriptEditor   *ScriptEditorScreen
	showingEditor  bool
}

// NewRootFlowController creates a new root flow controller
func NewRootFlowController() (*RootFlowController, error) {
	scriptService, err := services.NewScriptService()
	if err != nil {
		return nil, fmt.Errorf("failed to create script service: %w", err)
	}

	mainListScreen, err := NewMainListScreen()
	if err != nil {
		return nil, fmt.Errorf("failed to create main list screen: %w", err)
	}

	fc := &RootFlowController{
		BaseFlowController: NewBaseFlowController(),
		scriptService:      scriptService,
		mainListScreen:     mainListScreen,
	}

	// Inject services into the main list screen
	mainListScreen.SetServices(scriptService)

	// Set main list as current screen
	fc.SetCurrentScreen(mainListScreen)

	return fc, nil
}

// Run starts the root flow
func (fc *RootFlowController) Run() (TUIResult, error) {
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
func (fc *RootFlowController) HandleScreenResult(result ScreenResult) error {
	actionData := ExtractActionData(result)

	switch result.Action {
	case ActionExitApp:
		fc.Exit(result.ExitCode, result.Message)
		return nil

	case ActionExecuteScript:
		return fc.handleExecuteScript(actionData.ScriptPath)

	case ActionEditScriptExternal:
		return fc.handleExternalEdit(actionData.ScriptPath)

	case ActionEditScriptInline:
		return fc.handleInlineEdit(actionData.Script)

	case ActionScriptEditorSave:
		return fc.handleScriptEditorSave(result)

	case ActionScriptEditorCancel:
		return fc.handleScriptEditorCancel()

	case ActionDeleteScript:
		return fc.handleDeleteScript(actionData.Script)

	case ActionRefreshScripts:
		return fc.handleRefreshScripts()

	default:
		// Unknown action, continue
		return nil
	}
}

// handleExecuteScript handles script execution
func (fc *RootFlowController) handleExecuteScript(scriptPath string) error {
	if scriptPath == "" {
		return fmt.Errorf("no script path provided for execution")
	}

	// Load configuration to find the script entity
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	config, err := storage.ReadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Find the script in config by file path
	matchResult, err := fc.findScriptByFilePath(config, scriptPath)
	if err != nil {
		return fmt.Errorf("failed to find script: %w", err)
	}

	// Execute the script - this will write to command FD and exit
	if err := fc.executeFoundScript(matchResult, []string{}); err != nil {
		return fmt.Errorf("error executing script: %w", err)
	}

	// Set exit after successful execution
	fc.Exit(0, "Script executed")
	return nil
}

// handleExternalEdit handles external editor launch
func (fc *RootFlowController) handleExternalEdit(scriptPath string) error {
	if scriptPath == "" {
		return fmt.Errorf("no script path provided for external edit")
	}

	// Write script path for external editor
	if err := fc.writeScriptPathForEditor(scriptPath); err != nil {
		return fmt.Errorf("error writing script path: %w", err)
	}

	// Exit with external edit code
	fc.Exit(4, "External edit")
	return nil
}

// handleInlineEdit shows the script editor
func (fc *RootFlowController) handleInlineEdit(script *entities.Script) error {
	if script == nil {
		return fmt.Errorf("no script provided for inline edit")
	}

	// Create script editor
	scriptEditor := NewScriptEditorScreen(*script, false)
	scriptEditor.SetServices(fc.scriptService)
	fc.scriptEditor = scriptEditor
	fc.showingEditor = true

	// Switch to script editor
	fc.SetCurrentScreen(scriptEditor)
	return nil
}

// handleScriptEditorSave processes script editor save
func (fc *RootFlowController) handleScriptEditorSave(result ScreenResult) error {
	if fc.scriptEditor == nil {
		return fmt.Errorf("no script editor active")
	}

	editorResult := fc.scriptEditor.GetEditorResult()
	if editorResult.Cancelled {
		return fc.handleScriptEditorCancel()
	}

	// Get the original script for update
	originalScript := fc.scriptEditor.GetOriginalScript()

	// Save the script
	if err := fc.scriptService.SaveScript(editorResult.Script, editorResult.Command, &originalScript); err != nil {
		// Stay in editor and show error
		fc.scriptEditor.SetErrorMessage(fmt.Sprintf("Error saving: %v", err))
		return nil
	}

	// Successfully saved, return to main list
	fc.showingEditor = false
	fc.scriptEditor = nil
	fc.SetCurrentScreen(fc.mainListScreen)

	// Refresh the main list
	fc.mainListScreen.SetStatusMessage("Script updated successfully")
	fc.mainListScreen.RefreshScripts()

	return nil
}

// handleScriptEditorCancel cancels script editor
func (fc *RootFlowController) handleScriptEditorCancel() error {
	fc.showingEditor = false
	fc.scriptEditor = nil
	fc.SetCurrentScreen(fc.mainListScreen)
	fc.mainListScreen.SetStatusMessage("Edit cancelled")
	return nil
}

// handleDeleteScript handles script deletion
func (fc *RootFlowController) handleDeleteScript(script *entities.Script) error {
	if script == nil {
		return fmt.Errorf("no script provided for deletion")
	}

	// TODO: Implement script deletion through service
	// For now, just refresh
	fc.mainListScreen.SetStatusMessage("Delete not yet implemented")
	return nil
}

// handleRefreshScripts refreshes the script list
func (fc *RootFlowController) handleRefreshScripts() error {
	fc.mainListScreen.RefreshScripts()
	return nil
}

// Helper methods from root.go

// findScriptByFilePath finds a script entity in the config by its file path
func (fc *RootFlowController) findScriptByFilePath(config storage.Config, filePath string) (*script.MatchResult, error) {
	// Search through all scopes for a script with matching file path
	for scope, scripts := range config {
		for _, scriptEntity := range scripts {
			if scriptEntity.FilePath == filePath {
				// Create a match result for this script
				matchResult := &script.MatchResult{
					Type:       script.ExactName,
					Script:     scriptEntity,
					Confidence: 1.0,
				}
				// Ensure script has correct scope set
				matchResult.Script.Scope = scope
				return matchResult, nil
			}
		}
	}
	return nil, fmt.Errorf("script not found with file path: %s", filePath)
}

// executeFoundScript executes a matched script
func (fc *RootFlowController) executeFoundScript(matchResult *script.MatchResult, scriptArgs []string) error {
	// This is a simplified version - the full implementation would need 
	// the argument processing logic from root.go
	
	// For now, just write the script path to command FD
	return fc.writeScriptPathForEditor(matchResult.Script.FilePath)
}

// writeScriptPathForEditor writes the script path for editor use
func (fc *RootFlowController) writeScriptPathForEditor(scriptPath string) error {
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		return execution.WriteScriptPathToFile(scriptPath, cmdFdPath)
	}

	// Fallback to stdout for backward compatibility
	fmt.Print(scriptPath)
	return nil
}
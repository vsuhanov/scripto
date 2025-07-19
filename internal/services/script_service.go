package services

import (
	"fmt"
	"os"
	"path/filepath"

	"scripto/entities"
	"scripto/internal/storage"
)

// ScriptService handles all script-related business logic
type ScriptService struct {
	configPath string
}

// NewScriptService creates a new script service
func NewScriptService() (*ScriptService, error) {
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	return &ScriptService{
		configPath: configPath,
	}, nil
}

// SaveScript saves a new script or updates an existing one
func (s *ScriptService) SaveScript(script entities.Script, command string, originalScript *entities.Script) error {
	// Load current config
	config, err := storage.ReadConfig(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Remove old script if this is an update
	if originalScript != nil {
		if err := s.removeScriptFromConfig(config, *originalScript); err != nil {
			return fmt.Errorf("failed to remove old script: %w", err)
		}
	}

	// Validate scope
	if script.Scope == "" {
		return fmt.Errorf("scope cannot be empty")
	}

	// Check for duplicate names in the target scope
	if err := s.checkForDuplicateName(config, script); err != nil {
		return err
	}

	// Create or update script file
	var filePath string
	if originalScript != nil && originalScript.FilePath != "" {
		// Update existing file
		filePath = originalScript.FilePath
	} else if script.FilePath != "" {
		// Use existing file path (external file reference)
		filePath = script.FilePath
	} else {
		// Create new script file
		var err error
		filePath, err = storage.SaveScriptToFile(script.Name, command)
		if err != nil {
			return fmt.Errorf("failed to save script to file: %w", err)
		}
	}

	// Update script with file path
	script.FilePath = filePath

	// Add script to config
	if config[script.Scope] == nil {
		config[script.Scope] = []entities.Script{}
	}
	config[script.Scope] = append(config[script.Scope], script)

	// Save config
	if err := storage.WriteConfig(s.configPath, config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Update script file content
	if err := os.WriteFile(script.FilePath, []byte(command), 0644); err != nil {
		return fmt.Errorf("failed to update script file: %w", err)
	}

	return nil
}

// DeleteScript removes a script from the configuration and filesystem
func (s *ScriptService) DeleteScript(script entities.Script) error {
	// Load current config
	config, err := storage.ReadConfig(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Remove script from config
	if err := s.removeScriptFromConfig(config, script); err != nil {
		return fmt.Errorf("failed to remove script from config: %w", err)
	}

	// Save updated config
	if err := storage.WriteConfig(s.configPath, config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Remove script file if it exists
	if script.FilePath != "" {
		if err := os.Remove(script.FilePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove script file: %w", err)
		}
	}

	return nil
}

// CreateEmptyScript creates a new empty script with default values
func (s *ScriptService) CreateEmptyScript() entities.Script {
	// Default to current directory scope
	scope := "global"
	if cwd, err := os.Getwd(); err == nil {
		scope = cwd
	}

	return entities.Script{
		Name:        "",
		Description: "",
		FilePath:    "",
		Scope:       scope,
	}
}

// ValidateScript validates a script's properties
func (s *ScriptService) ValidateScript(script entities.Script) error {
	if script.Scope == "" {
		return fmt.Errorf("scope cannot be empty")
	}

	// Validate scope is either "global" or a valid directory path
	if script.Scope != "global" {
		if !filepath.IsAbs(script.Scope) {
			return fmt.Errorf("scope must be 'global' or an absolute directory path")
		}
	}

	return nil
}

// removeScriptFromConfig removes a script from the configuration
func (s *ScriptService) removeScriptFromConfig(config storage.Config, script entities.Script) error {
	scripts, exists := config[script.Scope]
	if !exists {
		return fmt.Errorf("script scope not found in config")
	}

	// Find and remove the script
	scriptRemoved := false
	for i, configScript := range scripts {
		if s.scriptsMatch(configScript, script) {
			config[script.Scope] = append(scripts[:i], scripts[i+1:]...)
			if len(config[script.Scope]) == 0 {
				delete(config, script.Scope)
			}
			scriptRemoved = true
			break
		}
	}

	if !scriptRemoved {
		return fmt.Errorf("script not found in config")
	}

	return nil
}

// scriptsMatch determines if two scripts are the same
func (s *ScriptService) scriptsMatch(script1, script2 entities.Script) bool {
	return script1.Name == script2.Name &&
		script1.FilePath == script2.FilePath &&
		script1.Description == script2.Description &&
		script1.Scope == script2.Scope
}

// checkForDuplicateName checks if a script with the same name already exists in the target scope
func (s *ScriptService) checkForDuplicateName(config storage.Config, script entities.Script) error {
	if script.Name == "" {
		return nil // Allow unnamed scripts
	}

	if scripts, exists := config[script.Scope]; exists {
		for _, existingScript := range scripts {
			if existingScript.Name == script.Name {
				return fmt.Errorf("script with name '%s' already exists in scope '%s'", script.Name, script.Scope)
			}
		}
	}

	return nil
}

// GetScopeDisplayName returns a user-friendly display name for a scope
func (s *ScriptService) GetScopeDisplayName(scope string) string {
	if scope == "global" {
		return "global"
	}
	return filepath.Base(scope)
}

// GetCurrentDirectoryScope returns the current directory as a scope
func (s *ScriptService) GetCurrentDirectoryScope() string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "global"
}

// CreateTempScriptFile creates a temporary script file with the given command content
func (s *ScriptService) CreateTempScriptFile(command string) (string, error) {
	// Use storage layer to create the script file
	filePath, err := storage.SaveScriptToFile("", command)
	if err != nil {
		return "", fmt.Errorf("failed to create temp script file: %w", err)
	}
	return filePath, nil
}
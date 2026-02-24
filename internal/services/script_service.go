package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"scripto/entities"
	"scripto/internal/script"
	"scripto/internal/storage"
)

// ScriptService handles all script-related business logic
type ScriptService struct {
	configPath string
	config     storage.Config
}

// NewScriptService creates a new script service
func NewScriptService() (*ScriptService, error) {
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	config, err := storage.ReadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	return &ScriptService{
		configPath: configPath,
		config:     config,
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

	// Handle shortcuts for global named scripts
	if script.Scope == "global" && script.Name != "" {
		// Remove old shortcut if this is an update and name changed
		if originalScript != nil && originalScript.Name != "" && originalScript.Name != script.Name {
			if err := storage.RemoveShortcutFunction(originalScript.Name); err != nil {
				// Log error but don't fail the entire operation
				fmt.Fprintf(os.Stderr, "Warning: failed to remove old shortcut for '%s': %v\n", originalScript.Name, err)
			}
		}
		
		// Create new shortcut
		if err := storage.CreateShortcutFunction(script.Name); err != nil {
			// Log error but don't fail the entire operation
			fmt.Fprintf(os.Stderr, "Warning: failed to create shortcut for '%s': %v\n", script.Name, err)
		}
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

	// Remove shortcut for global named scripts
	if script.Scope == "global" && script.Name != "" {
		if err := storage.RemoveShortcutFunction(script.Name); err != nil {
			// Log error but don't fail the entire operation
			fmt.Fprintf(os.Stderr, "Warning: failed to remove shortcut for '%s': %v\n", script.Name, err)
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

// SyncShortcuts synchronizes all shortcuts with the current configuration
func (s *ScriptService) SyncShortcuts() error {
	// Load current config
	config, err := storage.ReadConfig(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Sync shortcuts using storage layer
	if err := storage.SyncShortcuts(config); err != nil {
		return fmt.Errorf("failed to sync shortcuts: %w", err)
	}

	return nil
}

// Reload refreshes the config from disk
func (s *ScriptService) Reload() error {
	config, err := storage.ReadConfig(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}
	s.config = config
	return nil
}

// FIXME: this needs to be change to work with entities.Script, need to simplify the MatchResult, I don't think I need it.
func (s *ScriptService) FindAllScripts() ([]script.MatchResult, error) {
	var results []script.MatchResult

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Track directories we've seen to avoid duplicates
	seen := make(map[string]bool)

	// 1. Local scripts (current directory)
	if scripts, exists := s.config[cwd]; exists {
		for _, scriptEnt := range scripts {
			// Ensure script has correct scope set
			scriptEnt.Scope = cwd
			results = append(results, script.MatchResult{
				Script: scriptEnt,
			})
		}
		seen[cwd] = true
	}

	// 2. Parent directory scripts (walk up the tree)
	dir := cwd
	for {
		parent := filepath.Dir(dir)
		if parent == dir || parent == "/" {
			break // Reached root
		}

		if !seen[parent] {
			if scripts, exists := s.config[parent]; exists {
				for _, scriptEnt := range scripts {
					// Ensure script has correct scope set
					scriptEnt.Scope = parent
					results = append(results, script.MatchResult{
						Script: scriptEnt,
					})
				}
			}
			seen[parent] = true
		}

		dir = parent
	}

	// 3. Global scripts
	if scripts, exists := s.config["global"]; exists {
		for _, scriptEnt := range scripts {
			// Ensure script has correct scope set
			scriptEnt.Scope = "global"
			results = append(results, script.MatchResult{
				Script: scriptEnt,
			})
		}
	}

	return results, nil
}

// Match finds the best matching script for the given input
func (s *ScriptService) Match(input string) (*script.MatchResult, error) {
	allScripts, err := s.FindAllScripts()
	if err != nil {
		return nil, err
	}

	// Try exact name matches first (highest priority)
	for _, result := range allScripts {
		if result.Script.Name != "" && result.Script.Name == input {
			result.Type = script.ExactName
			result.Confidence = 1.0
			return &result, nil
		}
	}

	// Command field removed - partial command matching no longer available
	var candidates []script.MatchResult

	// Sort candidates by confidence (highest first), then by scope priority
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return s.getScopePriority(candidates[i].Script.Scope) < s.getScopePriority(candidates[j].Script.Scope)
	})

	if len(candidates) > 0 {
		return &candidates[0], nil
	}

	// No match found
	return &script.MatchResult{Type: script.NoMatch}, nil
}

// FilterByKeyword filters scripts that contain the given keyword
func (s *ScriptService) FilterByKeyword(keyword string) ([]script.MatchResult, error) {
	allScripts, err := s.FindAllScripts()
	if err != nil {
		return nil, err
	}

	var filtered []script.MatchResult
	keyword = strings.ToLower(keyword)

	for _, result := range allScripts {
		// Check if keyword appears in name or description
		searchText := strings.ToLower(result.Script.Name + " " + result.Script.Description)
		if strings.Contains(searchText, keyword) {
			// Calculate confidence based on keyword match quality
			result.Confidence = s.calculateKeywordConfidence(keyword, result.Script)
			filtered = append(filtered, result)
		}
	}

	// Sort by confidence, then scope priority
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Confidence != filtered[j].Confidence {
			return filtered[i].Confidence > filtered[j].Confidence
		}
		return s.getScopePriority(filtered[i].Script.Scope) < s.getScopePriority(filtered[j].Script.Scope)
	})

	return filtered, nil
}

// FindScriptByFilePath finds a script entity in the config by its file path
func (s *ScriptService) FindScriptByFilePath(filePath string) (*script.MatchResult, error) {
	// Search through all scopes for a script with matching file path
	for scope, scripts := range s.config {
		for _, scriptEnt := range scripts {
			if scriptEnt.FilePath == filePath {
				// Create a match result for this script
				matchResult := &script.MatchResult{
					Type:       script.ExactName,
					Script:     scriptEnt,
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

// getScopePriority returns the priority order for script scopes
func (s *ScriptService) getScopePriority(scope string) int {
	if scope == "global" {
		return 2
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return 3 // Unknown priority if we can't get cwd
	}

	if scope == cwd {
		return 0 // Local (current directory)
	}

	// Check if it's a parent directory
	if strings.HasPrefix(cwd, scope+string(filepath.Separator)) {
		return 1 // Parent directory
	}

	return 3 // Other directory
}

// calculateKeywordConfidence calculates how well the keyword matches the script
func (s *ScriptService) calculateKeywordConfidence(keyword string, scriptEnt entities.Script) float64 {
	confidence := 0.0

	// Exact name match gets highest score
	if strings.ToLower(scriptEnt.Name) == keyword {
		confidence = 1.0
	} else if strings.Contains(strings.ToLower(scriptEnt.Name), keyword) {
		confidence = 0.8
	}

	// Description matches get lower scores
	if strings.Contains(strings.ToLower(scriptEnt.Description), keyword) {
		confidence = max(confidence, 0.4)
	}

	return confidence
}

// max returns the larger of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

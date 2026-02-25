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

type ScriptService struct {
	configPath string
	config     storage.Config
}

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

func (s *ScriptService) SaveScript(script *entities.Script, command string, originalScript *entities.Script) error {
	config, err := storage.ReadConfig(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if originalScript != nil {
		if err := s.removeScriptFromConfig(config, originalScript); err != nil {
			return fmt.Errorf("failed to remove old script: %w", err)
		}
	}

	if script.Scope == "" {
		return fmt.Errorf("scope cannot be empty")
	}

	if err := s.checkForDuplicateName(config, script); err != nil {
		return err
	}

	var filePath string
	if originalScript != nil && originalScript.FilePath != "" {
		filePath = originalScript.FilePath
	} else if script.FilePath != "" {
		filePath = script.FilePath
	} else {
		var err error
		filePath, err = storage.SaveScriptToFile(script.Name, command)
		if err != nil {
			return fmt.Errorf("failed to save script to file: %w", err)
		}
	}

	script.FilePath = filePath

	if config[script.Scope] == nil {
		config[script.Scope] = []*entities.Script{}
	}
	config[script.Scope] = append(config[script.Scope], script)

	if err := storage.WriteConfig(s.configPath, config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if err := os.WriteFile(script.FilePath, []byte(command), 0644); err != nil {
		return fmt.Errorf("failed to update script file: %w", err)
	}

	if script.Scope == "global" && script.Name != "" {
		if originalScript != nil && originalScript.Name != "" && originalScript.Name != script.Name {
			if err := storage.RemoveShortcutFunction(originalScript.Name); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove old shortcut for '%s': %v\n", originalScript.Name, err)
			}
		}
		
		if err := storage.CreateShortcutFunction(script.Name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create shortcut for '%s': %v\n", script.Name, err)
		}
	}

	return nil
}

func (s *ScriptService) DeleteScript(script *entities.Script) error {
	config, err := storage.ReadConfig(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := s.removeScriptFromConfig(config, script); err != nil {
		return fmt.Errorf("failed to remove script from config: %w", err)
	}

	if err := storage.WriteConfig(s.configPath, config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if script.FilePath != "" {
		if err := os.Remove(script.FilePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove script file: %w", err)
		}
	}

	if script.Scope == "global" && script.Name != "" {
		if err := storage.RemoveShortcutFunction(script.Name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove shortcut for '%s': %v\n", script.Name, err)
		}
	}

	return nil
}

func (s *ScriptService) CreateEmptyScript() *entities.Script {
	scope := "global"
	if cwd, err := os.Getwd(); err == nil {
		scope = cwd
	}

	return &entities.Script{
		Name:        "",
		Description: "",
		FilePath:    "",
		Scope:       scope,
	}
}

func (s *ScriptService) ValidateScript(script *entities.Script) error {
	if script.Scope == "" {
		return fmt.Errorf("scope cannot be empty")
	}

	if script.Scope != "global" {
		if !filepath.IsAbs(script.Scope) {
			return fmt.Errorf("scope must be 'global' or an absolute directory path")
		}
	}

	return nil
}

func (s *ScriptService) removeScriptFromConfig(config storage.Config, script *entities.Script) error {
	scripts, exists := config[script.Scope]
	if !exists {
		return fmt.Errorf("script scope not found in config")
	}

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

func (s *ScriptService) scriptsMatch(script1, script2 *entities.Script) bool {
	return script1.Name == script2.Name &&
		script1.FilePath == script2.FilePath &&
		script1.Description == script2.Description &&
		script1.Scope == script2.Scope
}

func (s *ScriptService) checkForDuplicateName(config storage.Config, script *entities.Script) error {
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

func (s *ScriptService) GetScopeDisplayName(scope string) string {
	if scope == "global" {
		return "global"
	}
	return filepath.Base(scope)
}

func (s *ScriptService) GetCurrentDirectoryScope() string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "global"
}

func (s *ScriptService) CreateTempScriptFile(command string) (string, error) {
	filePath, err := storage.SaveScriptToFile("", command)
	if err != nil {
		return "", fmt.Errorf("failed to create temp script file: %w", err)
	}
	return filePath, nil
}

func (s *ScriptService) SyncShortcuts() error {
	config, err := storage.ReadConfig(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := storage.SyncShortcuts(config); err != nil {
		return fmt.Errorf("failed to sync shortcuts: %w", err)
	}

	return nil
}

func (s *ScriptService) Reload() error {
	config, err := storage.ReadConfig(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}
	s.config = config
	return nil
}

func (s *ScriptService) FindAllScripts() ([]script.MatchResult, error) {
	var results []script.MatchResult

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)

	if scripts, exists := s.config[cwd]; exists {
		for _, scriptEnt := range scripts {
			scriptEnt.Scope = cwd
			results = append(results, script.MatchResult{
				Script: scriptEnt,
			})
		}
		seen[cwd] = true
	}

	dir := cwd
	for {
		parent := filepath.Dir(dir)
		if parent == dir || parent == "/" {
			break // Reached root
		}

		if !seen[parent] {
			if scripts, exists := s.config[parent]; exists {
				for _, scriptEnt := range scripts {
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

	if scripts, exists := s.config["global"]; exists {
		for _, scriptEnt := range scripts {
			scriptEnt.Scope = "global"
			results = append(results, script.MatchResult{
				Script: scriptEnt,
			})
		}
	}

	return results, nil
}

func (s *ScriptService) Match(input string) (*script.MatchResult, error) {
	allScripts, err := s.FindAllScripts()
	if err != nil {
		return nil, err
	}

	for _, result := range allScripts {
		if result.Script.Name != "" && result.Script.Name == input {
			result.Type = script.ExactName
			result.Confidence = 1.0
			return &result, nil
		}
	}

	var candidates []script.MatchResult

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return s.getScopePriority(candidates[i].Script.Scope) < s.getScopePriority(candidates[j].Script.Scope)
	})

	if len(candidates) > 0 {
		return &candidates[0], nil
	}

	return &script.MatchResult{Type: script.NoMatch}, nil
}

func (s *ScriptService) FilterByKeyword(keyword string) ([]script.MatchResult, error) {
	allScripts, err := s.FindAllScripts()
	if err != nil {
		return nil, err
	}

	var filtered []script.MatchResult
	keyword = strings.ToLower(keyword)

	for _, result := range allScripts {
		searchText := strings.ToLower(result.Script.Name + " " + result.Script.Description)
		if strings.Contains(searchText, keyword) {
			result.Confidence = s.calculateKeywordConfidence(keyword, result.Script)
			filtered = append(filtered, result)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Confidence != filtered[j].Confidence {
			return filtered[i].Confidence > filtered[j].Confidence
		}
		return s.getScopePriority(filtered[i].Script.Scope) < s.getScopePriority(filtered[j].Script.Scope)
	})

	return filtered, nil
}

func (s *ScriptService) FindScriptByFilePath(filePath string) (*script.MatchResult, error) {
	for scope, scripts := range s.config {
		for _, scriptEnt := range scripts {
			if scriptEnt.FilePath == filePath {
				matchResult := &script.MatchResult{
					Type:       script.ExactName,
					Script:     scriptEnt,
					Confidence: 1.0,
				}
				matchResult.Script.Scope = scope
				return matchResult, nil
			}
		}
	}
	return nil, fmt.Errorf("script not found with file path: %s", filePath)
}

func (s *ScriptService) getScopePriority(scope string) int {
	if scope == "global" {
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		return 3 // Unknown priority if we can't get cwd
	}

	if scope == cwd {
		return 0 // Local (current directory)
	}

	if strings.HasPrefix(cwd, scope+string(filepath.Separator)) {
		return 1 // Parent directory
	}

	return 3 // Other directory
}

func (s *ScriptService) calculateKeywordConfidence(keyword string, scriptEnt *entities.Script) float64 {
	confidence := 0.0

	if strings.ToLower(scriptEnt.Name) == keyword {
		confidence = 1.0
	} else if strings.Contains(strings.ToLower(scriptEnt.Name), keyword) {
		confidence = 0.8
	}

	if strings.Contains(strings.ToLower(scriptEnt.Description), keyword) {
		confidence = max(confidence, 0.4)
	}

	return confidence
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

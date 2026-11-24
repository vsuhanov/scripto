package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/storage"
)

type ScriptService struct {
	configPath string
	mu         sync.Mutex
	config     storage.Config
}

func (s *ScriptService) current() storage.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

func (s *ScriptService) setConfig(config storage.Config) {
	s.mu.Lock()
	s.config = config
	s.mu.Unlock()
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

func (s *ScriptService) update(mutate func(storage.Config) error) error {
	config, err := storage.UpdateConfig(s.configPath, mutate)
	if err != nil {
		return err
	}
	s.setConfig(config)
	return nil
}

func sameScript(a, b *entities.Script) bool {
	if a.ID != "" && b.ID != "" {
		return a.ID == b.ID
	}
	return a.Name == b.Name && a.FilePath == b.FilePath
}

func locateScript(config storage.Config, script *entities.Script) (string, int, bool) {
	var scopes []string
	if script.OriginalScope != "" {
		scopes = append(scopes, script.OriginalScope)
	}
	scopes = append(scopes, script.Scope)
	for _, scope := range scopes {
		for i, candidate := range config[scope] {
			if sameScript(candidate, script) {
				return scope, i, true
			}
		}
	}
	if script.ID != "" {
		for scope, scripts := range config {
			for i, candidate := range scripts {
				if candidate.ID == script.ID {
					return scope, i, true
				}
			}
		}
	}
	return "", -1, false
}

func removeAt(config storage.Config, scope string, index int) {
	scripts := config[scope]
	config[scope] = append(scripts[:index], scripts[index+1:]...)
	if len(config[scope]) == 0 {
		delete(config, scope)
	}
}

func hasShortcut(script *entities.Script) bool {
	return script != nil && script.Scope == "global" && script.Name != "" && !script.Archived
}

func updateShortcuts(before, after *entities.Script) {
	if hasShortcut(before) && (!hasShortcut(after) || storage.ShortcutName(before.Name) != storage.ShortcutName(after.Name)) {
		if err := storage.RemoveShortcutFunction(before.Name); err != nil {
			log.Printf("Warning: failed to remove shortcut for '%s': %v", before.Name, err)
		}
	}
	if hasShortcut(after) {
		if err := storage.CreateShortcutFunction(after.Name); err != nil {
			log.Printf("Warning: failed to create shortcut for '%s': %v", after.Name, err)
		}
	}
}

func (s *ScriptService) SaveScript(script *entities.Script, command string, originalScript *entities.Script) error {
	if script.Scope == "" {
		return fmt.Errorf("scope cannot be empty")
	}
	script.OriginalScope = ""

	filePath := script.FilePath
	if originalScript != nil && originalScript.FilePath != "" {
		filePath = originalScript.FilePath
	}
	newFile := false
	if filePath == "" {
		var err error
		filePath, err = storage.NewScriptFilePath(script.Name, command)
		if err != nil {
			return fmt.Errorf("failed to save script to file: %w", err)
		}
		newFile = true
	}
	script.FilePath = filePath

	var before *entities.Script
	err := s.update(func(config storage.Config) error {
		if originalScript != nil {
			scope, index, ok := locateScript(config, originalScript)
			if !ok {
				return fmt.Errorf("failed to remove old script: script not found in config")
			}
			existing := *config[scope][index]
			existing.Scope = scope
			before = &existing
			script.Archived = existing.Archived
			removeAt(config, scope, index)
		}

		if err := checkForDuplicateName(config, script); err != nil {
			return err
		}

		if err := storage.WriteScriptFile(filePath, command); err != nil {
			return fmt.Errorf("failed to write script file: %w", err)
		}

		config[script.Scope] = append(config[script.Scope], script)
		return nil
	})
	if err != nil {
		if newFile {
			os.Remove(filePath)
		}
		return err
	}

	updateShortcuts(before, script)
	return nil
}

func (s *ScriptService) DeleteScript(script *entities.Script) error {
	var removed *entities.Script
	err := s.update(func(config storage.Config) error {
		scope, index, ok := locateScript(config, script)
		if !ok {
			return fmt.Errorf("failed to remove script from config: script not found in config")
		}
		existing := *config[scope][index]
		existing.Scope = scope
		removed = &existing
		removeAt(config, scope, index)
		return nil
	})
	if err != nil {
		return err
	}

	if removed.FilePath != "" {
		if err := os.Remove(removed.FilePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove script file: %w", err)
		}
	}

	updateShortcuts(removed, nil)
	return nil
}

func (s *ScriptService) setArchived(script *entities.Script, archived bool) error {
	var before, after entities.Script
	err := s.update(func(config storage.Config) error {
		scope, index, ok := locateScript(config, script)
		if !ok {
			return fmt.Errorf("script not found in config")
		}
		entry := config[scope][index]
		before = *entry
		before.Scope = scope
		entry.Archived = archived
		after = *entry
		after.Scope = scope
		return nil
	})
	if err != nil {
		return err
	}
	updateShortcuts(&before, &after)
	return nil
}

func (s *ScriptService) ArchiveScript(script *entities.Script) error {
	return s.setArchived(script, true)
}

func (s *ScriptService) UnarchiveScript(script *entities.Script) error {
	return s.setArchived(script, false)
}

func (s *ScriptService) FindAllScopesScriptsWithArchived() ([]*entities.Script, error) {
	config := s.current()
	var mainScripts []*entities.Script

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)

	if scripts, exists := config[cwd]; exists {
		for _, scriptEnt := range scripts {
			scriptEnt.Scope = cwd
			mainScripts = append(mainScripts, scriptEnt)
		}
		seen[cwd] = true
	}

	var patternScopes []string
	for scope := range config {
		if !seen[scope] && IsPatternScope(scope) && ScopeMatchesDir(scope, cwd) {
			patternScopes = append(patternScopes, scope)
		}
	}
	sort.Strings(patternScopes)
	for _, scope := range patternScopes {
		for _, scriptEnt := range config[scope] {
			scriptEnt.Scope = scope
			mainScripts = append(mainScripts, scriptEnt)
		}
		seen[scope] = true
	}

	dir := cwd
	for {
		parent := filepath.Dir(dir)
		if parent == dir || parent == "/" {
			break
		}
		if !seen[parent] {
			if scripts, exists := config[parent]; exists {
				for _, scriptEnt := range scripts {
					scriptEnt.Scope = parent
					mainScripts = append(mainScripts, scriptEnt)
				}
			}
			seen[parent] = true
		}
		dir = parent
	}

	if scripts, exists := config["global"]; exists {
		for _, scriptEnt := range scripts {
			scriptEnt.Scope = "global"
			mainScripts = append(mainScripts, scriptEnt)
		}
		seen["global"] = true
	}

	var extra []*entities.Script
	for scope, scripts := range config {
		if seen[scope] {
			continue
		}
		for _, scriptEnt := range scripts {
			scriptEnt.Scope = scope
			extra = append(extra, scriptEnt)
		}
	}

	sort.SliceStable(extra, func(i, j int) bool {
		return extra[i].Scope < extra[j].Scope
	})

	return append(mainScripts, extra...), nil
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

	if script.Scope != "global" && !IsPatternScope(script.Scope) {
		if !filepath.IsAbs(script.Scope) {
			return fmt.Errorf("scope must be 'global', a glob pattern, or an absolute directory path")
		}
	}

	return nil
}

func checkForDuplicateName(config storage.Config, script *entities.Script) error {
	if script.Name == "" {
		return nil
	}

	for _, existing := range config[script.Scope] {
		if existing.Name == script.Name {
			return fmt.Errorf("script with name '%s' already exists in scope '%s'", script.Name, script.Scope)
		}
	}

	if script.Scope == "global" {
		shortcut := storage.ShortcutName(script.Name)
		for _, existing := range config["global"] {
			if shortcut != "" && existing.Name != "" && storage.ShortcutName(existing.Name) == shortcut {
				return fmt.Errorf("global shortcut '%s' is already used by script '%s'", shortcut, existing.Name)
			}
		}
	}

	return nil
}

type GlobalShortcutInfo struct {
	Shortcut   string
	Normalized bool
	Shadows    string
}

func (s *ScriptService) GlobalShortcutInfo(name string) GlobalShortcutInfo {
	shortcut := storage.ShortcutName(name)
	info := GlobalShortcutInfo{
		Shortcut:   shortcut,
		Normalized: shortcut != strings.TrimSpace(name),
	}
	if shortcut != "" {
		if path, err := exec.LookPath(shortcut); err == nil {
			info.Shadows = path
		}
	}
	return info
}

func (s *ScriptService) GetScopeDisplayName(scope string) string {
	if scope == "global" {
		return "global"
	}
	if IsPatternScope(scope) {
		return scope
	}
	return filepath.Base(scope)
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
	s.setConfig(config)
	return nil
}

func (s *ScriptService) FindAllScripts() ([]*entities.Script, error) {
	config := s.current()
	var results []*entities.Script

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if scripts, exists := config[cwd]; exists {
		for _, scriptEnt := range scripts {
			if scriptEnt.Archived {
				continue
			}
			scriptEnt.Scope = cwd
			results = append(results, scriptEnt)
		}
	}

	var patternScopes []string
	for scope := range config {
		if scope != cwd && IsPatternScope(scope) && ScopeMatchesDir(scope, cwd) {
			patternScopes = append(patternScopes, scope)
		}
	}
	sort.Strings(patternScopes)
	for _, scope := range patternScopes {
		for _, scriptEnt := range config[scope] {
			if scriptEnt.Archived {
				continue
			}
			scriptEnt.Scope = scope
			results = append(results, scriptEnt)
		}
	}

	if scripts, exists := config["global"]; exists {
		for _, scriptEnt := range scripts {
			if scriptEnt.Archived {
				continue
			}
			scriptEnt.Scope = "global"
			results = append(results, scriptEnt)
		}
	}

	return results, nil
}

func (s *ScriptService) FindContextualScripts(contextualIDs map[string]bool, cwd string, alreadyInHierarchy map[string]bool) ([]*entities.Script, error) {
	config := s.current()
	var results []*entities.Script
	for scope, scripts := range config {
		if scope == "global" || alreadyInHierarchy[scope] || (IsPatternScope(scope) && ScopeMatchesDir(scope, cwd)) {
			continue
		}
		for _, scriptEnt := range scripts {
			if scriptEnt.Archived || !contextualIDs[scriptEnt.ID] {
				continue
			}
			copied := *scriptEnt
			copied.OriginalScope = scope
			copied.Scope = cwd
			results = append(results, &copied)
		}
	}
	return results, nil
}

func (s *ScriptService) FindAllScopesScripts() ([]*entities.Script, error) {
	mainScripts, err := s.FindAllScripts()
	if err != nil {
		return nil, err
	}
	return s.withOtherScopes(mainScripts), nil
}

func (s *ScriptService) withOtherScopes(mainScripts []*entities.Script) []*entities.Script {
	config := s.current()
	seen := make(map[string]bool)
	for _, script := range mainScripts {
		seen[script.Scope] = true
	}

	var extra []*entities.Script
	for scope, scripts := range config {
		if seen[scope] {
			continue
		}
		for _, scriptEnt := range scripts {
			if scriptEnt.Archived {
				continue
			}
			scriptEnt.Scope = scope
			extra = append(extra, scriptEnt)
		}
	}

	sort.SliceStable(extra, func(i, j int) bool {
		return extra[i].Scope < extra[j].Scope
	})

	result := make([]*entities.Script, 0, len(mainScripts)+len(extra))
	result = append(result, mainScripts...)
	return append(result, extra...)
}

type ScriptListings struct {
	Current      []*entities.Script
	AllScopes    []*entities.Script
	WithArchived []*entities.Script
}

func (s *ScriptService) FindScriptListings() (ScriptListings, error) {
	current, err := s.FindAllScripts()
	if err != nil {
		return ScriptListings{}, err
	}
	withArchived, err := s.FindAllScopesScriptsWithArchived()
	if err != nil {
		return ScriptListings{}, err
	}
	return ScriptListings{
		Current:      current,
		AllScopes:    s.withOtherScopes(current),
		WithArchived: withArchived,
	}, nil
}

func (s *ScriptService) MatchAllScopes(input string) ([]*entities.Script, error) {
	allScripts, err := s.FindAllScopesScripts()
	if err != nil {
		return nil, err
	}
	var matches []*entities.Script
	for _, script := range allScripts {
		if script.Name != "" && script.Name == input {
			matches = append(matches, script)
		}
	}
	return matches, nil
}

func (s *ScriptService) Match(input string) (*entities.Script, error) {
	allScripts, err := s.FindAllScripts()
	if err != nil {
		return nil, err
	}

	for _, script := range allScripts {
		if script.Name != "" && script.Name == input {
			return script, nil
		}
	}

	return nil, nil
}

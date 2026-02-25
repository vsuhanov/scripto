package script

import (
	"os"
	"path/filepath"
	"strings"

	"scripto/entities"
	"scripto/internal/storage"
)

type ScriptMatcher struct {
	config storage.Config
}

func NewMatcher(config storage.Config) *ScriptMatcher {
	return &ScriptMatcher{config: config}
}

func (m *ScriptMatcher) FindAllScripts() ([]*entities.Script, error) {
	var results []*entities.Script

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)

	if scripts, exists := m.config[cwd]; exists {
		for _, script := range scripts {
			script.Scope = cwd
			results = append(results, script)
		}
		seen[cwd] = true
	}

	dir := cwd
	for {
		parent := filepath.Dir(dir)
		if parent == dir || parent == "/" {
			break
		}

		if !seen[parent] {
			if scripts, exists := m.config[parent]; exists {
				for _, script := range scripts {
					script.Scope = parent
					results = append(results, script)
				}
			}
			seen[parent] = true
		}

		dir = parent
	}

	return results, nil
}

func (m *ScriptMatcher) Match(input string) (*entities.Script, error) {
	allScripts, err := m.FindAllScripts()
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

func (m *ScriptMatcher) FilterByKeyword(keyword string) ([]*entities.Script, error) {
	allScripts, err := m.FindAllScripts()
	if err != nil {
		return nil, err
	}

	var filtered []*entities.Script
	keyword = strings.ToLower(keyword)

	for _, script := range allScripts {
		searchText := strings.ToLower(script.Name + " " + script.Description)

		if strings.Contains(searchText, keyword) {
			filtered = append(filtered, script)
		}
	}

	return filtered, nil
}


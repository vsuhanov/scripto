package script

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"scripto/entities"
	"scripto/internal/storage"
)

type MatchType int

const (
	NoMatch MatchType = iota
	ExactName
	PartialCommand
)

type MatchResult struct {
	Type       MatchType
	Script     *entities.Script
	Confidence float64
}

type ScriptMatcher struct {
	config storage.Config
}

func NewMatcher(config storage.Config) *ScriptMatcher {
	return &ScriptMatcher{config: config}
}

func (m *ScriptMatcher) FindAllScripts() ([]MatchResult, error) {
	var results []MatchResult

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)

	if scripts, exists := m.config[cwd]; exists {
		for _, script := range scripts {
			script.Scope = cwd
			results = append(results, MatchResult{
				Script: script,
			})
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
					results = append(results, MatchResult{
						Script: script,
					})
				}
			}
			seen[parent] = true
		}

		dir = parent
	}

	return results, nil
}

func (m *ScriptMatcher) Match(input string) (*MatchResult, error) {
	allScripts, err := m.FindAllScripts()
	if err != nil {
		return nil, err
	}

	for _, result := range allScripts {
		if result.Script.Name != "" && result.Script.Name == input {
			result.Type = ExactName
			result.Confidence = 1.0
			return &result, nil
		}
	}

	var candidates []MatchResult

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return getScopePriority(candidates[i].Script.Scope) < getScopePriority(candidates[j].Script.Scope)
	})

	if len(candidates) > 0 {
		return &candidates[0], nil
	}

	return &MatchResult{Type: NoMatch}, nil
}

func (m *ScriptMatcher) FilterByKeyword(keyword string) ([]MatchResult, error) {
	allScripts, err := m.FindAllScripts()
	if err != nil {
		return nil, err
	}

	var filtered []MatchResult
	keyword = strings.ToLower(keyword)

	for _, result := range allScripts {
		searchText := strings.ToLower(result.Script.Name + " " + result.Script.Description)

		if strings.Contains(searchText, keyword) {
			result.Confidence = calculateKeywordConfidence(keyword, result.Script)
			filtered = append(filtered, result)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Confidence != filtered[j].Confidence {
			return filtered[i].Confidence > filtered[j].Confidence
		}
		return getScopePriority(filtered[i].Script.Scope) < getScopePriority(filtered[j].Script.Scope)
	})

	return filtered, nil
}

func calculateCommandConfidence(input, command string) float64 {
	if input == command {
		return 1.0
	}

	matchLength := float64(len(input))
	commandLength := float64(len(command))

	words := strings.Fields(command)
	if len(words) > 0 && strings.HasPrefix(words[0], input) {
		matchLength += 0.2
	}

	return matchLength / commandLength
}

func calculateKeywordConfidence(keyword string, script *entities.Script) float64 {
	confidence := 0.0

	if strings.ToLower(script.Name) == keyword {
		confidence = 1.0
	} else if strings.Contains(strings.ToLower(script.Name), keyword) {
		confidence = 0.8
	}

	if strings.Contains(strings.ToLower(script.Description), keyword) {
		confidence = max(confidence, 0.6)
	}

	if strings.Contains(strings.ToLower(script.Description), keyword) {
		confidence = max(confidence, 0.4)
	}

	return confidence
}

func getScopePriority(scope string) int {
	if scope == "global" {
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
	}

	if scope == cwd {
	}

	if strings.HasPrefix(cwd, scope+string(filepath.Separator)) {
	}

	return 1
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

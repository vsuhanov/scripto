package script

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"scripto/entities"
	"scripto/internal/storage"
)

// MatchType represents the type of match found
type MatchType int

const (
	NoMatch MatchType = iota
	ExactName
	PartialCommand
)

// MatchResult represents a script match with confidence scoring
type MatchResult struct {
	Type       MatchType
	Script     entities.Script
	Confidence float64
}

// ScriptMatcher handles script discovery and matching
type ScriptMatcher struct {
	config storage.Config
}

// NewMatcher creates a new ScriptMatcher
func NewMatcher(config storage.Config) *ScriptMatcher {
	return &ScriptMatcher{config: config}
}

// FindAllScripts discovers all available scripts in order: local -> parent -> global
func (m *ScriptMatcher) FindAllScripts() ([]MatchResult, error) {
	var results []MatchResult

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Track directories we've seen to avoid duplicates
	seen := make(map[string]bool)

	// 1. Local scripts (current directory)
	if scripts, exists := m.config[cwd]; exists {
		for _, script := range scripts {
			// Ensure script has correct scope set
			script.Scope = cwd
			results = append(results, MatchResult{
				Script: script,
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
			if scripts, exists := m.config[parent]; exists {
				for _, script := range scripts {
					// Ensure script has correct scope set
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

	// 3. Global scripts
	if scripts, exists := m.config["global"]; exists {
		for _, script := range scripts {
			// Ensure script has correct scope set
			script.Scope = "global"
			results = append(results, MatchResult{
				Script: script,
			})
		}
	}

	return results, nil
}

// Match finds the best matching script for the given input
func (m *ScriptMatcher) Match(input string) (*MatchResult, error) {
	allScripts, err := m.FindAllScripts()
	if err != nil {
		return nil, err
	}

	// Try exact name matches first (highest priority)
	for _, result := range allScripts {
		if result.Script.Name != "" && result.Script.Name == input {
			result.Type = ExactName
			result.Confidence = 1.0
			return &result, nil
		}
	}

	// Command field removed - partial command matching no longer available
	var candidates []MatchResult

	// Sort candidates by confidence (highest first), then by scope priority
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return getScopePriority(candidates[i].Script.Scope) < getScopePriority(candidates[j].Script.Scope)
	})

	if len(candidates) > 0 {
		return &candidates[0], nil
	}

	// No match found
	return &MatchResult{Type: NoMatch}, nil
}

// FilterByKeyword filters scripts that contain the given keyword
func (m *ScriptMatcher) FilterByKeyword(keyword string) ([]MatchResult, error) {
	allScripts, err := m.FindAllScripts()
	if err != nil {
		return nil, err
	}

	var filtered []MatchResult
	keyword = strings.ToLower(keyword)

	for _, result := range allScripts {
		// Check if keyword appears in name or command
		searchText := strings.ToLower(result.Script.Name + " " + result.Script.Description)
		if strings.Contains(searchText, keyword) {
			// Calculate confidence based on keyword match quality
			result.Confidence = calculateKeywordConfidence(keyword, result.Script)
			filtered = append(filtered, result)
		}
	}

	// Sort by confidence, then scope priority
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Confidence != filtered[j].Confidence {
			return filtered[i].Confidence > filtered[j].Confidence
		}
		return getScopePriority(filtered[i].Script.Scope) < getScopePriority(filtered[j].Script.Scope)
	})

	return filtered, nil
}

// calculateCommandConfidence calculates how well the input matches the command
func calculateCommandConfidence(input, command string) float64 {
	if input == command {
		return 1.0
	}

	// Higher confidence for longer matches
	matchLength := float64(len(input))
	commandLength := float64(len(command))

	// Bonus for exact word boundary matches
	words := strings.Fields(command)
	if len(words) > 0 && strings.HasPrefix(words[0], input) {
		matchLength += 0.2
	}

	return matchLength / commandLength
}

// calculateKeywordConfidence calculates how well the keyword matches the script
func calculateKeywordConfidence(keyword string, script entities.Script) float64 {
	confidence := 0.0

	// Exact name match gets highest score
	if strings.ToLower(script.Name) == keyword {
		confidence = 1.0
	} else if strings.Contains(strings.ToLower(script.Name), keyword) {
		confidence = 0.8
	}

	// Command matches get lower scores
	if strings.Contains(strings.ToLower(script.Description), keyword) {
		confidence = max(confidence, 0.6)
	}

	// Description matches get lowest scores
	if strings.Contains(strings.ToLower(script.Description), keyword) {
		confidence = max(confidence, 0.4)
	}

	return confidence
}

// getScopePriority returns the priority order for script scopes
func getScopePriority(scope string) int {
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

// max returns the larger of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

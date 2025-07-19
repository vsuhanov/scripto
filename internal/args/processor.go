package args

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"scripto/entities"
)

// PlaceholderValue represents a placeholder and its resolved value
type PlaceholderValue struct {
	Name        string
	Description string
	Value       string
	Provided    bool
}

// ProcessResult contains the result of argument processing
type ProcessResult struct {
	Placeholders map[string]PlaceholderValue
	FinalCommand string
	MissingArgs  []PlaceholderValue
}

// ArgumentProcessor handles parsing and processing of script arguments
type ArgumentProcessor struct {
	script entities.Script
}

// NewArgumentProcessor creates a new ArgumentProcessor for a script
func NewArgumentProcessor(script entities.Script) *ArgumentProcessor {
	return &ArgumentProcessor{script: script}
}

// getCommandContent reads the command content from the script's FilePath
func (p *ArgumentProcessor) getCommandContent() (string, error) {
	if p.script.FilePath == "" {
		return "", fmt.Errorf("script has no file path")
	}
	
	content, err := os.ReadFile(p.script.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read script file %s: %w", p.script.FilePath, err)
	}
	
	return strings.TrimSpace(string(content)), nil
}

// ProcessArguments processes the provided arguments against the script's placeholders
func (p *ArgumentProcessor) ProcessArguments(args []string) (*ProcessResult, error) {
	// Extract placeholder information from the command
	placeholders, err := p.extractPlaceholderInfo()
	if err != nil {
		return nil, err
	}

	// Process the provided arguments
	providedValues := p.parseProvidedArguments(args)

	// Match provided values to placeholders
	result := &ProcessResult{
		Placeholders: make(map[string]PlaceholderValue),
	}

	// First, handle named arguments
	for name, value := range providedValues.Named {
		if placeholder, exists := placeholders[name]; exists {
			placeholder.Value = value
			placeholder.Provided = true
			result.Placeholders[name] = placeholder
		}
	}

	// Then, handle positional arguments for remaining placeholders
	positionalIndex := 0
	placeholderOrder := p.getPlaceholderOrder()

	for _, name := range placeholderOrder {
		if placeholder, exists := result.Placeholders[name]; !exists || !placeholder.Provided {
			if positionalIndex < len(providedValues.Positional) {
				placeholder := placeholders[name]
				placeholder.Value = providedValues.Positional[positionalIndex]
				placeholder.Provided = true
				result.Placeholders[name] = placeholder
				positionalIndex++
			} else {
				// Missing argument
				result.Placeholders[name] = placeholders[name]
			}
		}
	}

	// Identify missing arguments
	for _, placeholder := range result.Placeholders {
		if !placeholder.Provided {
			result.MissingArgs = append(result.MissingArgs, placeholder)
		}
	}

	// Generate final command if all placeholders are provided
	if len(result.MissingArgs) == 0 {
		result.FinalCommand = p.substitutePlaceholders(result.Placeholders)
	}

	return result, nil
}

// ProvidedArguments represents parsed user input
type ProvidedArguments struct {
	Positional []string
	Named      map[string]string
}

// parseProvidedArguments parses the user's arguments into positional and named arguments
func (p *ArgumentProcessor) parseProvidedArguments(args []string) ProvidedArguments {
	result := ProvidedArguments{
		Named: make(map[string]string),
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Check for named argument patterns: --name=value or --name value
		if strings.HasPrefix(arg, "--") {
			if strings.Contains(arg, "=") {
				// --name=value format
				parts := strings.SplitN(arg[2:], "=", 2)
				if len(parts) == 2 {
					result.Named[parts[0]] = parts[1]
				}
			} else {
				// --name value format (check next argument)
				name := arg[2:]
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					result.Named[name] = args[i+1]
					// Skip the next argument since we consumed it
					i++
				}
			}
		} else {
			// Positional argument
			result.Positional = append(result.Positional, arg)
		}
	}

	return result
}

// extractPlaceholderInfo extracts placeholder information from the script command
func (p *ArgumentProcessor) extractPlaceholderInfo() (map[string]PlaceholderValue, error) {
	placeholders := make(map[string]PlaceholderValue)

	// Regex to match %name:description% placeholders
	re := regexp.MustCompile(`%([^:%]+):([^%]*)%`)
	command, err := p.getCommandContent()
	if err != nil {
		return nil, err
	}
	
	matches := re.FindAllStringSubmatch(command, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			name := match[1]
			description := match[2]
			placeholders[name] = PlaceholderValue{
				Name:        name,
				Description: description,
				Provided:    false,
			}
		}
	}

	return placeholders, nil
}

// getPlaceholderOrder returns the order of placeholders as they appear in the command
func (p *ArgumentProcessor) getPlaceholderOrder() []string {
	var order []string

	re := regexp.MustCompile(`%([^:%]+):[^%]*%`)
	command, err := p.getCommandContent()
	if err != nil {
		return nil // Return empty slice on error
	}
	
	matches := re.FindAllStringSubmatch(command, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 2 {
			name := match[1]
			if !seen[name] {
				order = append(order, name)
				seen[name] = true
			}
		}
	}

	return order
}

// substitutePlaceholders replaces placeholders in the command with provided values
func (p *ArgumentProcessor) substitutePlaceholders(placeholders map[string]PlaceholderValue) string {
	command, err := p.getCommandContent()
	if err != nil {
		return "" // Return empty string on error
	}

	// Replace each placeholder with its value
	for name, placeholder := range placeholders {
		if placeholder.Provided {
			// Create the placeholder pattern
			pattern := fmt.Sprintf(`%%%s:[^%%]*%%`, regexp.QuoteMeta(name))
			re := regexp.MustCompile(pattern)

			// Replace with the value, properly quoted if it contains spaces
			value := placeholder.Value
			if strings.Contains(value, " ") && !strings.HasPrefix(value, "\"") {
				value = fmt.Sprintf("\"%s\"", value)
			}

			command = re.ReplaceAllString(command, value)
		}
	}

	return command
}

// GetCompletionSuggestions returns completion suggestions for the given partial input
func (p *ArgumentProcessor) GetCompletionSuggestions(args []string) []string {
	placeholders, _ := p.extractPlaceholderInfo()
	var suggestions []string

	// If no arguments provided, suggest all placeholder flags
	if len(args) == 0 {
		for name, placeholder := range placeholders {
			suggestion := fmt.Sprintf("--%s=", name)
			if placeholder.Description != "" {
				suggestion += fmt.Sprintf("\t%s", placeholder.Description)
			}
			suggestions = append(suggestions, suggestion)
		}
		return suggestions
	}

	// Check if the last argument is an incomplete named argument
	lastArg := args[len(args)-1]
	if strings.HasPrefix(lastArg, "--") && !strings.Contains(lastArg, "=") {
		name := lastArg[2:]
		if placeholder, exists := placeholders[name]; exists {
			suggestion := fmt.Sprintf("--%s=", name)
			if placeholder.Description != "" {
				suggestion += fmt.Sprintf("\t%s", placeholder.Description)
			}
			suggestions = append(suggestions, suggestion)
		}
	}

	return suggestions
}

// ValidateArguments checks if the provided arguments are valid for the script
func (p *ArgumentProcessor) ValidateArguments(args []string) error {
	placeholders, _ := p.extractPlaceholderInfo()
	providedArgs := p.parseProvidedArguments(args)

	// Check for unknown named arguments
	for name := range providedArgs.Named {
		if _, exists := placeholders[name]; !exists {
			return fmt.Errorf("unknown argument: --%s", name)
		}
	}

	// Check if too many positional arguments provided
	if len(providedArgs.Positional) > len(placeholders) {
		return fmt.Errorf("too many arguments provided: expected %d, got %d",
			len(placeholders), len(providedArgs.Positional))
	}

	return nil
}

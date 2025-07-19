package args

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"scripto/entities"
)

// PlaceholderValue represents a placeholder and its resolved value
type PlaceholderValue struct {
	Name         string
	Description  string
	DefaultValue string
	Value        string
	Provided     bool
	IsPositional bool
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

// hasPositionalPlaceholders checks if the script has any positional placeholders
func (p *ArgumentProcessor) hasPositionalPlaceholders() (bool, error) {
	placeholders, err := p.extractPlaceholderInfo()
	if err != nil {
		return false, err
	}
	
	for _, placeholder := range placeholders {
		if placeholder.IsPositional {
			return true, nil
		}
	}
	
	return false, nil
}

// ProcessArguments processes the provided arguments against the script's placeholders
func (p *ArgumentProcessor) ProcessArguments(args []string) (*ProcessResult, error) {
	// Extract placeholder information from the command
	placeholders, err := p.extractPlaceholderInfo()
	if err != nil {
		return nil, err
	}

	// Check if this script has positional placeholders (disables named arguments)
	hasPositional, err := p.hasPositionalPlaceholders()
	if err != nil {
		return nil, err
	}

	// Process the provided arguments
	providedValues := p.parseProvidedArguments(args)

	// If script has positional placeholders, reject any named arguments
	if hasPositional && len(providedValues.Named) > 0 {
		return nil, fmt.Errorf("named arguments not allowed when script contains positional placeholders")
	}

	// Match provided values to placeholders
	result := &ProcessResult{
		Placeholders: make(map[string]PlaceholderValue),
	}

	// If no positional placeholders, handle named arguments first
	if !hasPositional {
		for name, value := range providedValues.Named {
			if placeholder, exists := placeholders[name]; exists {
				placeholder.Value = value
				placeholder.Provided = true
				result.Placeholders[name] = placeholder
			}
		}
	}

	// Handle positional arguments for remaining placeholders
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
				// Missing argument - use default value if available
				placeholder := placeholders[name]
				if placeholder.DefaultValue != "" {
					placeholder.Value = placeholder.DefaultValue
					placeholder.Provided = true
				}
				result.Placeholders[name] = placeholder
			}
		}
	}

	// Identify missing arguments (those without values and no defaults)
	for _, placeholder := range result.Placeholders {
		if !placeholder.Provided && placeholder.DefaultValue == "" {
			result.MissingArgs = append(result.MissingArgs, placeholder)
		}
	}

	// Generate final command if all placeholders are provided or have defaults
	if len(result.MissingArgs) == 0 {
		result.FinalCommand = p.substitutePlaceholders(result.Placeholders)
    log.Printf("%s", result.FinalCommand)
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
	positionalCounter := 0

	// Regex to match various placeholder formats:
	// %name:description:default% or %name::default% or %name:description:% or %:description:default% or %::default% or %%
	re := regexp.MustCompile(`%%|%([^:%]*):?([^:%]*):?([^%]*)%`)
	command, err := p.getCommandContent()
	if err != nil {
		return nil, err
	}
	
	matches := re.FindAllStringSubmatch(command, -1)

	for _, match := range matches {
		// Check if this is the simple %% case
		if match[0] == "%%" {
			positionalCounter++
			name := fmt.Sprintf("arg%d", positionalCounter)
			
			// Skip if already processed (avoid duplicates)
			if _, exists := placeholders[name]; exists {
				continue
			}
			
			placeholders[name] = PlaceholderValue{
				Name:         name,
				Description:  "",
				DefaultValue: "",
				Provided:     false,
				IsPositional: true,
			}
		} else if len(match) >= 4 {
			rawName := match[1]
			rawDescription := match[2] 
			rawDefault := match[3]
			
			// Handle escaped colons
			description := strings.ReplaceAll(rawDescription, "\\:", ":")
			defaultValue := strings.ReplaceAll(rawDefault, "\\:", ":")
			
			var name string
			var isPositional bool
			
			if rawName == "" {
				// Positional placeholder
				positionalCounter++
				name = fmt.Sprintf("arg%d", positionalCounter)
				isPositional = true
			} else {
				name = rawName
				isPositional = false
			}
			
			// Skip if already processed (avoid duplicates)
			if _, exists := placeholders[name]; exists {
				continue
			}
			
			placeholders[name] = PlaceholderValue{
				Name:         name,
				Description:  description,
				DefaultValue: defaultValue,
				Provided:     false,
				IsPositional: isPositional,
			}
		}
	}

	return placeholders, nil
}

// getPlaceholderOrder returns the order of placeholders as they appear in the command
func (p *ArgumentProcessor) getPlaceholderOrder() []string {
	var order []string

	re := regexp.MustCompile(`%%|%([^:%]*):?([^:%]*):?([^%]*)%`)
	command, err := p.getCommandContent()
	if err != nil {
		return nil // Return empty slice on error
	}
	
	matches := re.FindAllStringSubmatch(command, -1)
	positionalCounter := 0

	seen := make(map[string]bool)
	for _, match := range matches {
		var name string
		
		if match[0] == "%%" {
			// Simple positional placeholder
			positionalCounter++
			name = fmt.Sprintf("arg%d", positionalCounter)
		} else if len(match) >= 4 {
			rawName := match[1]
			
			if rawName == "" {
				// Positional placeholder with description/default
				positionalCounter++
				name = fmt.Sprintf("arg%d", positionalCounter)
			} else {
				// Named placeholder
				name = rawName
			}
		}
		
		if name != "" && !seen[name] {
			order = append(order, name)
			seen[name] = true
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

	// Get all placeholder matches to replace them in order
	re := regexp.MustCompile(`%%|%([^:%]*):?([^:%]*):?([^%]*)%`)
	matches := re.FindAllStringSubmatch(command, -1)
	
	positionalCounter := 0
	
	// Create a replacement map for each specific placeholder occurrence
	replacements := make(map[string]string)
	
	for _, match := range matches {
		var placeholderKey string
		var value string
		
		if match[0] == "%%" {
			// Simple positional placeholder
			positionalCounter++
			placeholderKey = fmt.Sprintf("arg%d", positionalCounter)
		} else if len(match) >= 4 {
			rawName := match[1]
			
			if rawName == "" {
				// Positional placeholder with description/default
				positionalCounter++
				placeholderKey = fmt.Sprintf("arg%d", positionalCounter)
			} else {
				// Named placeholder
				placeholderKey = rawName
			}
		}
		
		// Get the value for this placeholder
		if placeholder, exists := placeholders[placeholderKey]; exists && placeholder.Provided {
			value = placeholder.Value
		} else if placeholder, exists := placeholders[placeholderKey]; exists && placeholder.DefaultValue != "" {
			// Use default value if no value provided
			value = placeholder.DefaultValue
		} else {
			// Keep original placeholder if no value available
			continue
		}
		
		// Properly quote if it contains spaces
		if strings.Contains(value, " ") && !strings.HasPrefix(value, "\"") {
			value = fmt.Sprintf("\"%s\"", value)
		}
		
		// Store the replacement
		replacements[match[0]] = value
	}
	
	// Apply all replacements
	result := command
	for placeholder, value := range replacements {
		result = strings.Replace(result, placeholder, value, 1)
	}

	return result
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

// isExecutableScript checks if the script is an executable (starts with shebang)
func (p *ArgumentProcessor) isExecutableScript() (bool, error) {
	if p.script.FilePath == "" {
		return false, nil
	}
	
	content, err := os.ReadFile(p.script.FilePath)
	if err != nil {
		return false, err
	}
	
	return strings.HasPrefix(string(content), "#!"), nil
}

// ValidateArguments checks if the provided arguments are valid for the script
func (p *ArgumentProcessor) ValidateArguments(args []string) error {
	// Check if this is an executable script
	isExecutable, err := p.isExecutableScript()
	if err != nil {
		log.Printf("DEBUG ValidateArguments: failed to check if executable: %v", err)
		// Continue with validation anyway
	}
	
	if isExecutable {
		// Executable scripts accept any arguments, no validation needed
		log.Printf("DEBUG ValidateArguments: script is executable, skipping validation")
		return nil
	}

	placeholders, _ := p.extractPlaceholderInfo()
	providedArgs := p.parseProvidedArguments(args)

	// Log placeholder information for debugging
	log.Printf("DEBUG ValidateArguments: script.FilePath=%s", p.script.FilePath)
	log.Printf("DEBUG ValidateArguments: found %d placeholders", len(placeholders))
	for name, ph := range placeholders {
		log.Printf("DEBUG ValidateArguments: placeholder %s: %+v", name, ph)
	}
	log.Printf("DEBUG ValidateArguments: provided args: positional=%v, named=%v", providedArgs.Positional, providedArgs.Named)

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

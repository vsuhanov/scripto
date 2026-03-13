package args

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"scripto/entities"
)

type PlaceholderValue struct {
	Name         string
	Description  string
	Descriptions []string
	DefaultValue string
	Value        string
	Provided     bool
	IsPositional bool
}

type ProcessResult struct {
	Placeholders map[string]PlaceholderValue
	FinalCommand string
	MissingArgs  []PlaceholderValue
}

type ArgumentProcessor struct {
	script *entities.Script
}

func NewArgumentProcessor(script *entities.Script) *ArgumentProcessor {
	return &ArgumentProcessor{script: script}
}

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

func (p *ArgumentProcessor) HasPositionalPlaceholders() (bool, error) {
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

func (p *ArgumentProcessor) ProcessArguments(args []string) (*ProcessResult, error) {
	placeholders, err := p.extractPlaceholderInfo()
	if err != nil {
		return nil, err
	}

	hasPositional, err := p.HasPositionalPlaceholders()
	if err != nil {
		return nil, err
	}

	providedValues := p.parseProvidedArguments(args)
  log.Printf("args: %s", args)
  log.Printf("providedValues: %s", providedValues)
	if hasPositional && len(providedValues.Named) > 0 {
		return nil, fmt.Errorf("named arguments not allowed when script contains positional placeholders")
	}

	result := &ProcessResult{
		Placeholders: make(map[string]PlaceholderValue),
	}

	if !hasPositional {
		for name, value := range providedValues.Named {
			if placeholder, exists := placeholders[name]; exists {
				placeholder.Value = value
				placeholder.Provided = true
				result.Placeholders[name] = placeholder
			}
		}
	}

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
				placeholder := placeholders[name]
				if placeholder.DefaultValue != "" {
					placeholder.Value = placeholder.DefaultValue
					placeholder.Provided = true
				}
				result.Placeholders[name] = placeholder
			}
		}
	}

	for _, placeholder := range result.Placeholders {
		if !placeholder.Provided && placeholder.DefaultValue == "" {
			result.MissingArgs = append(result.MissingArgs, placeholder)
		}
	}

	if len(result.MissingArgs) == 0 {
		result.FinalCommand = p.substitutePlaceholders(result.Placeholders)
    log.Printf("%s", result.FinalCommand)
	}

	return result, nil
}

type ProvidedArguments struct {
	Positional []string
	Named      map[string]string
}

func (p *ArgumentProcessor) parseProvidedArguments(args []string) ProvidedArguments {
	result := ProvidedArguments{
		Named: make(map[string]string),
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg[2:], "=", 2)
				if len(parts) == 2 {
					result.Named[parts[0]] = parts[1]
				}
			} else {
				name := arg[2:]
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					result.Named[name] = args[i+1]
					i++
				}
			}
		} else {
			result.Positional = append(result.Positional, arg)
		}
	}

	return result
}

func (p *ArgumentProcessor) getDemarcators() (string, string) {
	start := p.script.PlaceholderStartDemarcator
	end := p.script.PlaceholderEndDemarcator
	if start == "" {
		start = "%"
	}
	if end == "" {
		end = "%"
	}
	return start, end
}

func (p *ArgumentProcessor) buildPlaceholderRegex() *regexp.Regexp {
	start, end := p.getDemarcators()
	if start == "%" && end == "%" {
		return regexp.MustCompile(`%%|%([^:%\n]*):?([^:%\n]*):?([^%\n]*)%`)
	}
	escapedStart := regexp.QuoteMeta(start)
	escapedEnd := regexp.QuoteMeta(end)
	endChar := regexp.QuoteMeta(string([]rune(end)[0]))
	pattern := escapedStart + `([^:` + endChar + `\n]*):?([^:` + endChar + `\n]*):?([^` + endChar + `\n]*)` + escapedEnd
	return regexp.MustCompile(pattern)
}

func (p *ArgumentProcessor) extractPlaceholderInfo() (map[string]PlaceholderValue, error) {
	placeholders := make(map[string]PlaceholderValue)
	positionalCounter := 0

	re := p.buildPlaceholderRegex()
	command, err := p.getCommandContent()
	if err != nil {
		return nil, err
	}

	matches := re.FindAllStringSubmatch(command, -1)

	for _, match := range matches {
		if match[0] == "%%" {
			positionalCounter++
			name := fmt.Sprintf("arg%d", positionalCounter)

			if _, exists := placeholders[name]; exists {
				continue
			}

			placeholders[name] = PlaceholderValue{
				Name:         name,
				Descriptions: []string{},
				DefaultValue: "",
				Provided:     false,
				IsPositional: true,
			}
		} else if len(match) >= 4 {
			rawName := match[1]
			rawDescription := match[2]
			rawDefault := match[3]

			description := strings.ReplaceAll(rawDescription, "\\:", ":")
			defaultValue := strings.ReplaceAll(rawDefault, "\\:", ":")

			var name string
			var isPositional bool

			if rawName == "" {
				positionalCounter++
				name = fmt.Sprintf("arg%d", positionalCounter)
				isPositional = true
			} else {
				name = rawName
				isPositional = false
			}

			if existing, exists := placeholders[name]; exists {
				if description != "" {
					alreadyPresent := false
					for _, d := range existing.Descriptions {
						if d == description {
							alreadyPresent = true
							break
						}
					}
					if !alreadyPresent {
						existing.Descriptions = append(existing.Descriptions, description)
						placeholders[name] = existing
					}
				}
				continue
			}

			placeholders[name] = PlaceholderValue{
				Name:         name,
				Description:  description,
				Descriptions: []string{description},
				DefaultValue: defaultValue,
				Provided:     false,
				IsPositional: isPositional,
			}
		}
	}

	return placeholders, nil
}

func (p *ArgumentProcessor) GetPlaceholderOrder() []string {
	return p.getPlaceholderOrder()
}

func (p *ArgumentProcessor) getPlaceholderOrder() []string {
	var order []string

	re := p.buildPlaceholderRegex()
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
			positionalCounter++
			name = fmt.Sprintf("arg%d", positionalCounter)
		} else if len(match) >= 4 {
			rawName := match[1]
			
			if rawName == "" {
				positionalCounter++
				name = fmt.Sprintf("arg%d", positionalCounter)
			} else {
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

func (p *ArgumentProcessor) substitutePlaceholders(placeholders map[string]PlaceholderValue) string {
	command, err := p.getCommandContent()
	if err != nil {
		return "" // Return empty string on error
	}

	re := p.buildPlaceholderRegex()
	matches := re.FindAllStringSubmatch(command, -1)
	
	positionalCounter := 0
	
	replacements := make(map[string]string)
	
	for _, match := range matches {
		var placeholderKey string
		var value string
		
		if match[0] == "%%" {
			positionalCounter++
			placeholderKey = fmt.Sprintf("arg%d", positionalCounter)
		} else if len(match) >= 4 {
			rawName := match[1]
			
			if rawName == "" {
				positionalCounter++
				placeholderKey = fmt.Sprintf("arg%d", positionalCounter)
			} else {
				placeholderKey = rawName
			}
		}
		
		if placeholder, exists := placeholders[placeholderKey]; exists && placeholder.Provided {
			value = placeholder.Value
		} else if placeholder, exists := placeholders[placeholderKey]; exists && placeholder.DefaultValue != "" {
			value = placeholder.DefaultValue
		} else {
			continue
		}
		
		isInQuotes := strings.Contains(command, `"`+match[0]+`"`)
		if !isInQuotes && strings.Contains(value, " ") && !strings.HasPrefix(value, "\"") {
			value = fmt.Sprintf("\"%s\"", value)
		}

		replacements[match[0]] = value
	}

	result := command
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

func (p *ArgumentProcessor) BuildPreviewCommand(values map[string]string) string {
	command, err := p.getCommandContent()
	if err != nil {
		return ""
	}

	re := p.buildPlaceholderRegex()
	positionalCounter := 0

	result := re.ReplaceAllStringFunc(command, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if submatches == nil {
			return match
		}

		var key string
		if match == "%%" {
			positionalCounter++
			key = fmt.Sprintf("arg%d", positionalCounter)
		} else if len(submatches) >= 4 {
			rawName := submatches[1]
			if rawName == "" {
				positionalCounter++
				key = fmt.Sprintf("arg%d", positionalCounter)
			} else {
				key = rawName
			}
		}

		if val, ok := values[key]; ok && val != "" {
			isInQuotes := strings.Contains(command, `"`+match+`"`)
			if !isInQuotes && strings.Contains(val, " ") && !strings.HasPrefix(val, "\"") {
				return fmt.Sprintf("\"%s\"", val)
			}
			return val
		}
		return match
	})

	return result
}

func (p *ArgumentProcessor) GetCompletionSuggestions(args []string) []string {
	placeholders, _ := p.extractPlaceholderInfo()
	var suggestions []string

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

func (p *ArgumentProcessor) ValidateArguments(args []string) error {
	isExecutable, err := p.isExecutableScript()
	if err != nil {
		log.Printf("DEBUG ValidateArguments: failed to check if executable: %v", err)
	}
	
	if isExecutable {
		log.Printf("DEBUG ValidateArguments: script is executable, skipping validation")
		return nil
	}

	placeholders, _ := p.extractPlaceholderInfo()
	providedArgs := p.parseProvidedArguments(args)

	log.Printf("DEBUG ValidateArguments: script.FilePath=%s", p.script.FilePath)
	log.Printf("DEBUG ValidateArguments: found %d placeholders", len(placeholders))
	for name, ph := range placeholders {
		log.Printf("DEBUG ValidateArguments: placeholder %s: %+v", name, ph)
	}
	log.Printf("DEBUG ValidateArguments: provided args: positional=%v, named=%v", providedArgs.Positional, providedArgs.Named)

	for name := range providedArgs.Named {
		if _, exists := placeholders[name]; !exists {
			return fmt.Errorf("unknown argument: --%s", name)
		}
	}

	if len(providedArgs.Positional) > len(placeholders) {
		return fmt.Errorf("too many arguments provided: expected %d, got %d",
			len(placeholders), len(providedArgs.Positional))
	}

	return nil
}

package args

import (
	"fmt"
	"log"
	"os"
	"strings"

	"scripto/entities"
	"scripto/internal/templatex"
)

type ProcessResult struct {
	Metas        []templatex.VariableMeta
	FinalCommand string
	MissingArgs  []string
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

func (p *ArgumentProcessor) ProcessArguments(args []string) (*ProcessResult, error) {
	content, err := p.getCommandContent()
	if err != nil {
		return nil, err
	}

	metas, err := templatex.ExtractVariables(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	provided := p.parseProvidedArguments(args)
	log.Printf("args: %s", args)
	log.Printf("providedValues: %s", provided)

	values := make(map[string]string)
	for _, meta := range metas {
		if meta.DefaultValue != "" {
			values[meta.Name] = meta.DefaultValue
		}
	}
	for name, value := range provided.Named {
		values[name] = value
	}

	var missingArgs []string
	for _, meta := range metas {
		if values[meta.Name] == "" {
			missingArgs = append(missingArgs, meta.Name)
		}
	}

	result := &ProcessResult{
		Metas:       metas,
		MissingArgs: missingArgs,
	}

	if len(missingArgs) == 0 {
		finalCommand, err := templatex.Execute(content, values)
		if err != nil {
			return nil, fmt.Errorf("failed to execute template: %w", err)
		}
		result.FinalCommand = finalCommand
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

func (p *ArgumentProcessor) BuildPreviewCommand(values map[string]string) string {
	content, err := p.getCommandContent()
	if err != nil {
		return ""
	}
	result, err := templatex.Execute(content, values)
	if err != nil {
		return content
	}
	return result
}

func (p *ArgumentProcessor) GetCompletionSuggestions(args []string) []string {
	content, _ := p.getCommandContent()
	metas, _ := templatex.ExtractVariables(content)
	var suggestions []string

	if len(args) == 0 {
		for _, meta := range metas {
			suggestion := fmt.Sprintf("--%s=", meta.Name)
			if meta.Label != meta.Name {
				suggestion += fmt.Sprintf("\t%s", meta.Label)
			}
			suggestions = append(suggestions, suggestion)
		}
		return suggestions
	}

	lastArg := args[len(args)-1]
	if strings.HasPrefix(lastArg, "--") && !strings.Contains(lastArg, "=") {
		name := lastArg[2:]
		for _, meta := range metas {
			if meta.Name == name {
				suggestion := fmt.Sprintf("--%s=", name)
				if meta.Label != meta.Name {
					suggestion += fmt.Sprintf("\t%s", meta.Label)
				}
				suggestions = append(suggestions, suggestion)
				break
			}
		}
	}

	return suggestions
}

package services

import (
	"fmt"
	"os"
	"scripto/internal/args"
	"scripto/internal/script"
	"strings"
)

type ArgumentProcessingResult struct {
	NeedsPlaceholderForm bool
	Placeholders         []args.PlaceholderValue
	FinalCommand         string
}

type ExecutionService struct{}

func NewExecutionService() *ExecutionService {
	return &ExecutionService{}
}

func (es *ExecutionService) ProcessScriptArguments(matchResult *script.MatchResult, scriptArgs []string) (*ArgumentProcessingResult, error) {
	if matchResult.Script.FilePath == "" {
		return nil, fmt.Errorf("script has no file path or command content")
	}

	content, err := os.ReadFile(matchResult.Script.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script file %s: %w", matchResult.Script.FilePath, err)
	}

	contentStr := string(content)

	if strings.HasPrefix(contentStr, "#!") {
		finalCommand := matchResult.Script.FilePath
		for _, arg := range scriptArgs {
			if strings.Contains(arg, " ") && !strings.HasPrefix(arg, "\"") {
				finalCommand += fmt.Sprintf(" \"%s\"", arg)
			} else {
				finalCommand += " " + arg
			}
		}
		return &ArgumentProcessingResult{
			NeedsPlaceholderForm: false,
			FinalCommand:         finalCommand,
		}, nil
	}

	processor := args.NewArgumentProcessor(matchResult.Script)

	if err := processor.ValidateArguments(scriptArgs); err != nil {
		return nil, err
	}

	result, err := processor.ProcessArguments(scriptArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to process arguments: %w", err)
	}

	hasPlaceholders := len(result.Placeholders) > 0

	if !hasPlaceholders {
		return &ArgumentProcessingResult{
			NeedsPlaceholderForm: false,
			FinalCommand:         result.FinalCommand,
		}, nil
	}

	var allPlaceholders []args.PlaceholderValue
	placeholderOrder := processor.GetPlaceholderOrder()

	for _, name := range placeholderOrder {
		if placeholder, exists := result.Placeholders[name]; exists {
			if placeholder.Provided && placeholder.Value != "" {
				placeholder.DefaultValue = placeholder.Value
			}
			allPlaceholders = append(allPlaceholders, placeholder)
		}
	}

	if len(allPlaceholders) == 0 {
		for _, placeholder := range result.Placeholders {
			if placeholder.Provided && placeholder.Value != "" {
				placeholder.DefaultValue = placeholder.Value
			}
			allPlaceholders = append(allPlaceholders, placeholder)
		}
	}

	return &ArgumentProcessingResult{
		NeedsPlaceholderForm: true,
		Placeholders:         allPlaceholders,
	}, nil
}

func (es *ExecutionService) PrepareExecution(matchResult *script.MatchResult, scriptArgs []string, placeholderValues map[string]string) (string, error) {
	processor := args.NewArgumentProcessor(matchResult.Script)

	result, err := processor.ProcessArguments(scriptArgs)
	if err != nil {
		return "", fmt.Errorf("failed to process arguments: %w", err)
	}

	for name, value := range placeholderValues {
		if placeholder, exists := result.Placeholders[name]; exists {
			placeholder.Value = value
			placeholder.Provided = true
			result.Placeholders[name] = placeholder
		}
	}

	hasPositional, err := processor.HasPositionalPlaceholders()
	if err != nil {
		return "", fmt.Errorf("failed to check placeholder types: %w", err)
	}

	var additionalArgs []string
	if hasPositional {
		additionalArgs = es.convertToPositionalArgs(placeholderValues, result.Placeholders)
	} else {
		additionalArgs = es.convertToArgs(placeholderValues)
	}

	newResult, err := processor.ProcessArguments(append(scriptArgs, additionalArgs...))
	if err != nil {
		return "", err
	}

	return newResult.FinalCommand, nil
}

func (es *ExecutionService) PrepareDirectExecution(processingResult *ArgumentProcessingResult) (string, error) {
	if processingResult == nil {
		return "", fmt.Errorf("processing result is nil")
	}
	return processingResult.FinalCommand, nil
}

func (es *ExecutionService) convertToArgs(values map[string]string) []string {
	var arguments []string
	for name, value := range values {
		arguments = append(arguments, fmt.Sprintf("--%s=%s", name, value))
	}
	return arguments
}

func (es *ExecutionService) convertToPositionalArgs(values map[string]string, _ map[string]args.PlaceholderValue) []string {
	var arguments []string
	for _, value := range values {
		arguments = append(arguments, value)
	}
	return arguments
}

package services

import (
	"fmt"
	"os"
	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/templatex"
	"strings"
)

type ArgumentProcessingResult struct {
	NeedsPlaceholderForm bool
	Metas                []templatex.VariableMeta
	FinalCommand         string
	OriginalScript       string
	ParsedValues         map[string]string
}

type ExecutionService struct{}

func NewExecutionService() *ExecutionService {
	return &ExecutionService{}
}

func (es *ExecutionService) ProcessScriptArguments(s *entities.Script, scriptArgs []string) (*ArgumentProcessingResult, error) {
	if s.FilePath == "" {
		return nil, fmt.Errorf("script has no file path or command content")
	}

	content, err := os.ReadFile(s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script file %s: %w", s.FilePath, err)
	}

	contentStr := string(content)

	if strings.HasPrefix(contentStr, "#!") {
		finalCommand := s.FilePath
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
			OriginalScript:       contentStr,
		}, nil
	}

	trimmed := strings.TrimSpace(contentStr)
	metas, err := templatex.ExtractVariables(trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	if len(metas) == 0 {
		return &ArgumentProcessingResult{
			NeedsPlaceholderForm: false,
			FinalCommand:         trimmed,
			OriginalScript:       contentStr,
		}, nil
	}

	parsedValues := parseNamedArgs(scriptArgs)

	allSatisfied := true
	for _, meta := range metas {
		if parsedValues[meta.Name] == "" {
			allSatisfied = false
			break
		}
	}

	if allSatisfied {
		values := make(map[string]string)
		for _, meta := range metas {
			if meta.DefaultValue != "" {
				values[meta.Name] = meta.DefaultValue
			}
		}
		for name, val := range parsedValues {
			values[name] = val
		}
		finalCommand, err := templatex.Execute(trimmed, values)
		if err != nil {
			return nil, fmt.Errorf("failed to render template: %w", err)
		}
		return &ArgumentProcessingResult{
			NeedsPlaceholderForm: false,
			Metas:                metas,
			FinalCommand:         finalCommand,
			OriginalScript:       contentStr,
			ParsedValues:         parsedValues,
		}, nil
	}

	return &ArgumentProcessingResult{
		NeedsPlaceholderForm: true,
		Metas:                metas,
		OriginalScript:       contentStr,
		ParsedValues:         parsedValues,
	}, nil
}

func parseNamedArgs(args []string) map[string]string {
	result := make(map[string]string)
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		trimmed := strings.TrimPrefix(arg, "--")
		idx := strings.Index(trimmed, "=")
		if idx < 0 {
			continue
		}
		result[trimmed[:idx]] = trimmed[idx+1:]
	}
	return result
}

func (es *ExecutionService) PrepareExecution(s *entities.Script, _ []string, placeholderValues map[string]string) (string, error) {
	content, err := os.ReadFile(s.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read script file %s: %w", s.FilePath, err)
	}

	contentStr := strings.TrimSpace(string(content))

	metas, err := templatex.ExtractVariables(contentStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	values := make(map[string]string)
	for _, meta := range metas {
		if meta.DefaultValue != "" {
			values[meta.Name] = meta.DefaultValue
		}
	}
	for name, value := range placeholderValues {
		if value != "" {
			values[name] = value
		}
	}

	return templatex.Execute(contentStr, values)
}

func (es *ExecutionService) PrepareDirectExecution(processingResult *ArgumentProcessingResult) (string, error) {
	if processingResult == nil {
		return "", fmt.Errorf("processing result is nil")
	}
	return processingResult.FinalCommand, nil
}

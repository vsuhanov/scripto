package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/templatex"
	"github.com/vsuhanov/scripto/internal/utils"
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
		if err := ensureExecutable(s.FilePath); err != nil {
			return nil, fmt.Errorf("failed to make script executable: %w", err)
		}
		parts := []string{utils.ShellQuoteIfNeeded(s.FilePath)}
		for _, arg := range scriptArgs {
			parts = append(parts, utils.ShellQuoteIfNeeded(arg))
		}
		return &ArgumentProcessingResult{
			NeedsPlaceholderForm: false,
			FinalCommand:         strings.Join(parts, " "),
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

	parsedValues := ParseNamedArgs(scriptArgs)

	satisfied := true
	for _, meta := range metas {
		if parsedValues[meta.Name] != "" {
			continue
		}
		if len(scriptArgs) > 0 && meta.DefaultValue != "" {
			continue
		}
		satisfied = false
		break
	}

	if !satisfied {
		return &ArgumentProcessingResult{
			NeedsPlaceholderForm: true,
			Metas:                metas,
			OriginalScript:       contentStr,
			ParsedValues:         parsedValues,
		}, nil
	}

	finalCommand, err := renderWithMetas(trimmed, metas, parsedValues)
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

func ensureExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()
	if perm&0o100 != 0 {
		return nil
	}
	return os.Chmod(path, perm|0o111)
}

func ParseNamedArgs(args []string) map[string]string {
	result := make(map[string]string)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			continue
		}
		trimmed := strings.TrimPrefix(arg, "--")
		if name, value, ok := strings.Cut(trimmed, "="); ok {
			result[name] = value
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			result[trimmed] = args[i+1]
			i++
		}
	}
	return result
}

func ExtractWorkingDirArg(args []string) (string, []string) {
	var workingDir string
	var remaining []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--working-dir="):
			workingDir = strings.TrimPrefix(arg, "--working-dir=")
		case arg == "--working-dir" && i+1 < len(args):
			workingDir = args[i+1]
			i++
		default:
			remaining = append(remaining, arg)
		}
	}
	return workingDir, remaining
}

func renderWithMetas(content string, metas []templatex.VariableMeta, placeholderValues map[string]string) (string, error) {
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
	return templatex.Execute(content, values)
}

func (es *ExecutionService) Render(content string, placeholderValues map[string]string) (string, error) {
	trimmed := strings.TrimSpace(content)
	metas, err := templatex.ExtractVariables(trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}
	return renderWithMetas(trimmed, metas, placeholderValues)
}

func (es *ExecutionService) RenderPreview(content string, placeholderValues map[string]string) string {
	trimmed := strings.TrimSpace(content)
	rendered, err := templatex.Execute(trimmed, placeholderValues)
	if err != nil {
		return trimmed
	}
	return rendered
}

func (es *ExecutionService) PrepareExecution(s *entities.Script, placeholderValues map[string]string) (string, error) {
	content, err := os.ReadFile(s.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read script file %s: %w", s.FilePath, err)
	}
	return es.Render(string(content), placeholderValues)
}

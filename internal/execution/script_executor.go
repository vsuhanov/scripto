package execution

import (
	"fmt"
	"os"
	"scripto/internal/args"
	"scripto/internal/script"
	"strings"
)

// ArgumentProcessingResult contains the result of argument processing
type ArgumentProcessingResult struct {
	NeedsPlaceholderForm bool
	Placeholders         []args.PlaceholderValue
	FinalCommand         string
}

// ScriptExecutor handles script execution logic
type ScriptExecutor struct{}

// NewScriptExecutor creates a new script executor
func NewScriptExecutor() *ScriptExecutor {
	return &ScriptExecutor{}
}

// ProcessScriptArguments determines if argument processing is needed and returns placeholders
func (se *ScriptExecutor) ProcessScriptArguments(matchResult *script.MatchResult, scriptArgs []string) (*ArgumentProcessingResult, error) {
	// Check if script has a file path (stored as file) or is a command
	if matchResult.Script.FilePath == "" {
		return nil, fmt.Errorf("script has no file path or command content")
	}

	// Read the script file to determine execution type
	content, err := os.ReadFile(matchResult.Script.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script file %s: %w", matchResult.Script.FilePath, err)
	}

	contentStr := string(content)

	// Check if this is an executable script (starts with shebang)
	if strings.HasPrefix(contentStr, "#!") {
		// Executable script - no placeholder processing needed
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

	// Shell command script - check if placeholder processing is needed
	processor := args.NewArgumentProcessor(matchResult.Script)

	// Validate arguments
	if err := processor.ValidateArguments(scriptArgs); err != nil {
		return nil, err
	}

	// Process arguments
	result, err := processor.ProcessArguments(scriptArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to process arguments: %w", err)
	}

	// Check if script has any placeholders that need user input
	hasPlaceholders := len(result.Placeholders) > 0

	if !hasPlaceholders {
		// No placeholders needed, return final command
		return &ArgumentProcessingResult{
			NeedsPlaceholderForm: false,
			FinalCommand:         result.FinalCommand,
		}, nil
	}

	// Prepare placeholders for form
	var allPlaceholders []args.PlaceholderValue
	placeholderOrder := processor.GetPlaceholderOrder()

	for _, name := range placeholderOrder {
		if placeholder, exists := result.Placeholders[name]; exists {
			// Set the default value to the provided value if available
			if placeholder.Provided && placeholder.Value != "" {
				placeholder.DefaultValue = placeholder.Value
			}
			allPlaceholders = append(allPlaceholders, placeholder)
		}
	}

	// If no order found, use placeholders from result
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
		FinalCommand:         result.FinalCommand, // Initial command before form values
	}, nil
}

// ExecuteScriptWithPlaceholders executes a script with provided placeholder values
func (se *ScriptExecutor) ExecuteScriptWithPlaceholders(matchResult *script.MatchResult, scriptArgs []string, placeholderValues map[string]string) error {
	processor := args.NewArgumentProcessor(matchResult.Script)

	// Process initial arguments
	result, err := processor.ProcessArguments(scriptArgs)
	if err != nil {
		return fmt.Errorf("failed to process arguments: %w", err)
	}

	// Update result with placeholder values
	for name, value := range placeholderValues {
		if placeholder, exists := result.Placeholders[name]; exists {
			placeholder.Value = value
			placeholder.Provided = true
			result.Placeholders[name] = placeholder
		}
	}

	// Check if script has positional placeholders
	hasPositional, err := processor.HasPositionalPlaceholders()
	if err != nil {
		return fmt.Errorf("failed to check placeholder types: %w", err)
	}

	// Convert values to appropriate argument format and regenerate final command
	var additionalArgs []string
	if hasPositional {
		// For positional scripts, convert named values to positional arguments
		additionalArgs = se.convertToPositionalArgs(placeholderValues, result.Placeholders)
	} else {
		// For named scripts, convert to named arguments
		additionalArgs = se.convertToArgs(placeholderValues)
	}

	newResult, err := processor.ProcessArguments(append(scriptArgs, additionalArgs...))
	if err != nil {
		return err
	}

	// Execute the final command
	return se.executeFinalCommand(newResult.FinalCommand)
}

// ExecuteScriptDirect executes a script directly without placeholder processing
func (se *ScriptExecutor) ExecuteScriptDirect(finalCommand string) error {
	return se.executeFinalCommand(finalCommand)
}


// convertToArgs converts a map of values to argument format
func (se *ScriptExecutor) convertToArgs(values map[string]string) []string {
	var arguments []string
	for name, value := range values {
		arguments = append(arguments, fmt.Sprintf("--%s=%s", name, value))
	}
	return arguments
}

// convertToPositionalArgs converts named values back to positional arguments based on order
func (se *ScriptExecutor) convertToPositionalArgs(values map[string]string, placeholders map[string]args.PlaceholderValue) []string {
	var arguments []string
	// Convert based on the order of placeholders
	for _, value := range values {
		arguments = append(arguments, value)
	}
	return arguments
}

// executeFinalCommand executes a script file with the given placeholders
func (se *ScriptExecutor) executeFinalCommand(finalCommand string) error {
	// Check if we have a custom file descriptor for command output
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		// Write command to custom descriptor file
		err := os.WriteFile(cmdFdPath, []byte(finalCommand), 0600)
		if err != nil {
			return fmt.Errorf("failed to write command to descriptor: %w", err)
		}
		return nil
	}

	// Fallback to stdout for backward compatibility
	fmt.Print(finalCommand)
	return nil
}
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"scripto/internal/args"
	"scripto/internal/prompt"
	"scripto/internal/script"
	"scripto/internal/storage"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scripto [script-name-or-command...]",
	Short: "Execute stored scripts with placeholder substitution",
	Long: `Scripto allows you to store and execute command scripts with placeholders.

Examples:
  scripto                           # Show TUI (not implemented yet)
  scripto echo hello               # Execute script matching "echo hello"
  scripto deploy myapp 8080        # Execute "deploy" script with positional args
  scripto backup --host=localhost  # Execute "backup" script with named args`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// No arguments - show TUI or help
			fmt.Println("TUI not implemented yet")
			fmt.Println("Use 'scripto --help' for usage information")
			return
		}

		// Execute script matching logic
		if err := executeScript(args); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// executeScript handles the main script execution logic
func executeScript(userArgs []string) error {
	// Load configuration
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	config, err := storage.ReadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Create matcher and find best match
	matcher := script.NewMatcher(config)
	input := strings.Join(userArgs, " ")
	matchResult, err := matcher.Match(input)
	if err != nil {
		return fmt.Errorf("failed to match script: %w", err)
	}

	switch matchResult.Type {
	case script.ExactName:
		return executeNamedScript(matchResult, userArgs[1:]) // Skip the script name
	case script.PartialCommand:
		return executePartialCommand(matchResult, userArgs)
	case script.NoMatch:
		return handleNoMatch(input, config, configPath)
	default:
		return fmt.Errorf("unknown match type")
	}
}

// executeNamedScript executes a script found by exact name match
func executeNamedScript(matchResult *script.MatchResult, remainingArgs []string) error {
	processor := args.NewArgumentProcessor(matchResult.Script)

	// Validate arguments
	if err := processor.ValidateArguments(remainingArgs); err != nil {
		return err
	}

	// Process arguments
	result, err := processor.ProcessArguments(remainingArgs)
	if err != nil {
		return fmt.Errorf("failed to process arguments: %w", err)
	}

	// Handle missing arguments
	if len(result.MissingArgs) > 0 {
		prompter := prompt.NewPlaceholderPrompter(prompt.NewConsolePrompter())
		missingValues, err := prompter.PromptForMissingPlaceholders(result.MissingArgs)
		if err != nil {
			return fmt.Errorf("failed to prompt for missing arguments: %w", err)
		}

		// Update result with prompted values
		for name, value := range missingValues {
			if placeholder, exists := result.Placeholders[name]; exists {
				placeholder.Value = value
				placeholder.Provided = true
				result.Placeholders[name] = placeholder
			}
		}

		// Regenerate final command
		processor := args.NewArgumentProcessor(matchResult.Script)
		newResult, err := processor.ProcessArguments(append(remainingArgs, convertToArgs(missingValues)...))
		if err != nil {
			return err
		}
		result.FinalCommand = newResult.FinalCommand
	}

	// Execute the command
	return executeCommand(result.FinalCommand)
}

// executePartialCommand executes a script found by partial command match
func executePartialCommand(matchResult *script.MatchResult, userArgs []string) error {
	processor := args.NewArgumentProcessor(matchResult.Script)
	result, err := processor.ProcessArguments([]string{}) // No args provided yet
	if err != nil {
		return err
	}

	// If the script has placeholders, prompt for them
	if len(result.MissingArgs) > 0 {
		prompter := prompt.NewPlaceholderPrompter(prompt.NewConsolePrompter())
		missingValues, err := prompter.PromptForMissingPlaceholders(result.MissingArgs)
		if err != nil {
			return fmt.Errorf("failed to prompt for missing arguments: %w", err)
		}

		// Create final command with substituted values
		finalArgs := convertToArgs(missingValues)
		newResult, err := processor.ProcessArguments(finalArgs)
		if err != nil {
			return err
		}
		result.FinalCommand = newResult.FinalCommand
	} else {
		result.FinalCommand = matchResult.Script.Command
	}

	return executeCommand(result.FinalCommand)
}

// handleNoMatch handles the case when no script matches
func handleNoMatch(input string, config storage.Config, configPath string) error {
	prompter := prompt.NewPlaceholderPrompter(prompt.NewConsolePrompter())

	// Ask if user wants to save the command
	save, name, description, err := prompter.PromptToSaveCommand(input)
	if err != nil {
		return err
	}

	if !save {
		return fmt.Errorf("command not found: %s", input)
	}

	// Prompt for scope (global or local)
	global, err := prompter.PromptForScope()
	if err != nil {
		return fmt.Errorf("failed to prompt for scope: %w", err)
	}

	// Use the shared function to store the script
	if err := StoreScript(config, configPath, name, input, description, global); err != nil {
		return err
	}

	fmt.Printf("Saved script: %s\n", input)

	// Parse placeholders for execution
	placeholders := ParsePlaceholders(input)

	// Now execute the saved script
	if len(placeholders) > 0 {
		// Create a script object for processing
		savedScript := storage.Script{
			Name:         name,
			Command:      input,
			Placeholders: placeholders,
			Description:  description,
		}

		processor := args.NewArgumentProcessor(savedScript)
		result, err := processor.ProcessArguments([]string{})
		if err != nil {
			return err
		}

		missingValues, err := prompter.PromptForMissingPlaceholders(result.MissingArgs)
		if err != nil {
			return fmt.Errorf("failed to prompt for missing arguments: %w", err)
		}

		// Substitute placeholders
		finalCommand := input
		for name, value := range missingValues {
			pattern := fmt.Sprintf("%%%s:", name)
			if strings.Contains(finalCommand, pattern) {
				// Find and replace the placeholder
				start := strings.Index(finalCommand, pattern)
				if start != -1 {
					// Find the next % that closes the placeholder
					endSearch := finalCommand[start+len(pattern):]
					endIdx := strings.Index(endSearch, "%")
					if endIdx != -1 {
						end := start + len(pattern) + endIdx + 1
						placeholder := finalCommand[start:end]
						finalCommand = strings.Replace(finalCommand, placeholder, value, 1)
					}
				}
			}
		}

		return executeCommand(finalCommand)
	}

	return executeCommand(input)
}

// convertToArgs converts a map of values to argument format
func convertToArgs(values map[string]string) []string {
	var arguments []string
	for name, value := range values {
		arguments = append(arguments, fmt.Sprintf("--%s=%s", name, value))
	}
	return arguments
}

// executeCommand executes the final command using the system shell
func executeCommand(command string) error {
	fmt.Printf("Executing: %s\n", command)

	// Execute using shell
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// convertScriptResultsToSuggestions converts script matcher results to completion suggestions
func convertScriptResultsToSuggestions(results []script.MatchResult, separator string) []string {
	var suggestions []string
	for _, result := range results {
		if result.Script.Name != "" {
			// Named script - use scope as group name
			description := result.Script.Description
			if description == "" {
				description = result.Script.Command
			}
			suggestions = append(suggestions, result.Directory+separator+result.Script.Name+separator+description)
		} else {
			// Unnamed script - show command, use scope as group name
			command := result.Script.Command
			if len(command) > 50 {
				command = command[:47] + "..."
			}
			suggestions = append(suggestions, result.Directory+separator+command+separator+result.Script.Command)
		}
	}
	return suggestions
}

// getCompletionSuggestions provides completion suggestions for zsh
func getCompletionSuggestions(cmdArgs []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	separator := "\x1F"

	// Load configuration
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	config, err := storage.ReadConfig(configPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	matcher := script.NewMatcher(config)

	// If no args provided, show all available scripts
	if len(cmdArgs) == 0 {
		allScripts, err := matcher.FindAllScripts()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		suggestions := convertScriptResultsToSuggestions(allScripts, separator)
		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	// If we have args, try to provide context-aware completion
	if len(cmdArgs) == 1 {
		// Check if this might be a named script
		firstArg := cmdArgs[0]
		matchResult, err := matcher.Match(firstArg)
		if err == nil && matchResult.Type == script.ExactName {
			// This is a named script, provide argument completions
			processor := args.NewArgumentProcessor(matchResult.Script)
			return processor.GetCompletionSuggestions([]string{}), cobra.ShellCompDirectiveNoFileComp
		}

		// Otherwise, filter scripts by keyword
		filtered, err := matcher.FilterByKeyword(toComplete)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		suggestions := convertScriptResultsToSuggestions(filtered, separator)
		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	return nil, cobra.ShellCompDirectiveNoFileComp
}

func Execute() {
	// Get command line arguments
	cmdArgs := os.Args[1:]

	// If no arguments, run root command normally
	if len(cmdArgs) == 0 {
		if err := rootCmd.Execute(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return
	}

	firstArg := cmdArgs[0]

	// Handle completion specially to avoid Cobra's built-in suggestions
	if firstArg == "__complete" {
		handleCompletion(cmdArgs[1:])
		return
	}

	// Check if the first argument is a known subcommand
	knownSubcommands := []string{"add", "completion", "help", "--help", "-h"}

	for _, subcmd := range knownSubcommands {
		if firstArg == subcmd {
			// This is a known subcommand, delegate to Cobra
			if err := rootCmd.Execute(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			return
		}
	}

	// Not a known subcommand, treat as script execution
	if err := executeScript(cmdArgs); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// handleCompletion handles the __complete command directly, bypassing Cobra's built-in completion
func handleCompletion(args []string) {
	// Parse the completion arguments to extract the command being completed
	var cmdArgs []string
	var toComplete string

	// The args should be the command parts, with the last one being the part to complete
	// If the last argument is empty (""), it means we're completing after a space
	if len(args) > 0 && args[len(args)-1] == "" {
		// Remove the empty string and use the previous args
		cmdArgs = args[:len(args)-1]
		toComplete = ""
	} else if len(args) > 0 {
		// The last argument is what we're trying to complete
		cmdArgs = args[:len(args)-1]
		toComplete = args[len(args)-1]
	}

	// Get completion suggestions using the existing logic
	suggestions, _ := getCompletionSuggestions(cmdArgs, toComplete)

	// Print suggestions in the format expected by shell completion
	for _, suggestion := range suggestions {
		fmt.Println(suggestion)
	}

	// End with the completion directive
	fmt.Println(":36") // ShellCompDirectiveNoFileComp | ShellCompDirectiveKeepOrder
}

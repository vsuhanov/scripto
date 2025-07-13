package commands

import (
	"fmt"
	"os"
	"strings"

	"scripto/internal/args"
	"scripto/internal/prompt"
	"scripto/internal/script"
	"scripto/internal/storage"
	"scripto/internal/tui"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scripto [script-name-or-command...]",
	Short: "Execute stored scripts with placeholder substitution",
	Long: `Scripto allows you to store and execute command scripts with placeholders.

Examples:
  scripto                           # Launch interactive TUI
  scripto echo hello               # Execute script matching "echo hello"
  scripto deploy myapp 8080        # Execute "deploy" script with positional args
  scripto backup --host=localhost  # Execute "backup" script with named args`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// No arguments - launch TUI
			result, err := tui.RunWithResult()
			if err != nil {
				fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
				os.Exit(1)
			}

			switch result.Action {
			case tui.ActionExecute:
				// Execute the selected script
				if err := executeCommand(result.ScriptPath); err != nil {
					fmt.Fprintf(os.Stderr, "Error executing script: %v\n", err)
					os.Exit(1)
				}
			case tui.ActionEdit:
				// Write script path for editor and exit with special code
				if err := executeCommand(result.ScriptPath); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing script path: %v\n", err)
					os.Exit(1)
				}
			}

			os.Exit(result.ExitCode)
		}

		// Execute script matching logic
		if err := executeScript(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
	return executeScriptFile(matchResult, result.FinalCommand)
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

	return executeScriptFile(matchResult, result.FinalCommand)
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

// executeScriptFile executes a script, preferring file path if available
func executeScriptFile(matchResult *script.MatchResult, finalCommand string) error {
	// If the script has a file path, use that for execution
	if matchResult.Script.FilePath != "" {
		return executeCommand(matchResult.Script.FilePath)
	}

	// Fallback to direct command execution for scripts without file paths
	return executeCommand(finalCommand)
}

// executeCommand outputs the command for shell function to evaluate
func executeCommand(command string) error {
	// Check if we have a custom file descriptor for command output
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		// Write command to custom descriptor file
		err := os.WriteFile(cmdFdPath, []byte(command), 0600)
		if err != nil {
			return fmt.Errorf("failed to write command to descriptor: %w", err)
		}
		return nil
	}

	// Fallback to stdout for backward compatibility
	fmt.Print(command)
	return nil
}

// convertScriptResultsToSuggestions converts script matcher results to completion suggestions
func convertScriptResultsToSuggestions(results []script.MatchResult, separator string, toComplete string) []string {
	// log.Printf("convertScriptResultsToSuggestions: toComplete=%s, separator=%s", toComplete, separator)
	var suggestions []string
	for _, result := range results {
		if result.Script.Name != "" {
			// Named script - use scope as group name
			description := result.Script.Description
			if description == "" {
				description = result.Script.Command
			}

			// Filter and strip prefix if needed
			name := result.Script.Name
			if toComplete != "" {
				if !strings.HasPrefix(name, toComplete) {
					continue // Skip if doesn't match prefix
				}
				// //log.Printf("Prefix matched: %s", toComplete)
				// name = strings.TrimPrefix(name, toComplete)
			}

			suggestions = append(suggestions, result.Directory+separator+name+separator+description)
		} else {
			// Unnamed script - show command, use scope as group name
			command := result.Script.Command
			displayCommand := command
			// if len(displayCommand) > 50 {
			// 	displayCommand = displayCommand[:47] + "..."
			// }

			// Filter and strip prefix if needed
			if toComplete != "" {
				if !strings.HasPrefix(command, toComplete) {
					continue // Skip if doesn't match prefix
				}
				// command = strings.TrimPrefix(command, toComplete)
				// //log.Printf("Prefix matched: %s", toComplete)

				// Update display command too
				displayCommand = command
				// if len(displayCommand) > 50 {
				// 	displayCommand = displayCommand[:47] + "..."
				// }
			}

			suggestions = append(suggestions, result.Directory+separator+displayCommand+separator+result.Script.Command)
		}
	}
	return suggestions
}

// getCompletionSuggestions provides completion suggestions for zsh
func getCompletionSuggestions(toComplete string) ([]string, cobra.ShellCompDirective) {
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

	// Always find all scripts and filter by prefix
	allScripts, err := matcher.FindAllScripts()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// log.Printf("getCompletionSuggestions: toComplete=%s, allScripts count=%d", toComplete, len(allScripts))
	suggestions := convertScriptResultsToSuggestions(allScripts, separator, toComplete)
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func Execute() {
	// Get command line arguments
	cmdArgs := os.Args[1:]

	// If no arguments, run root command normally
	if len(cmdArgs) == 0 {
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		// Built-in command completed successfully - exit with code 3
		os.Exit(3)
	}

	firstArg := cmdArgs[0]

	// Handle completion specially to avoid Cobra's built-in suggestions
	if firstArg == "__complete" {
		handleCompletion(cmdArgs[1:])
		// Completion completed successfully - exit with code 3
		os.Exit(3)
	}

	// Check if the first argument is a known subcommand
	knownSubcommands := []string{"add", "completion", "install", "help", "--help", "-h"}

	for _, subcmd := range knownSubcommands {
		if firstArg == subcmd {
			// This is a known subcommand, delegate to Cobra
			if err := rootCmd.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			// Built-in command completed successfully - exit with code 3
			os.Exit(3)
		}
	}

	// Not a known subcommand, treat as script execution
	if err := executeScript(cmdArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// handleCompletion handles the __complete command directly, bypassing Cobra's built-in completion
func handleCompletion(args []string) {
	// Parse the completion arguments to extract the command being completed
	var toComplete string
	// log.Printf("handleCompletion: args=%v", args)

	// If the last argument is empty (""), it means we're completing after a space
	if len(args) > 0 && args[len(args)-1] == "" {
		// Remove the empty string and use the previous args as full command
		// log.Printf("remove empty string")
		toComplete = strings.Join(args[:len(args)-1], " ")
	} else if len(args) > 0 {
		// Use all arguments as the full command being completed
		toComplete = strings.Join(args, " ")
	}

	// Strip leading quotes from toComplete for matching (handle both escaped and unescaped quotes)
	cleanToComplete := toComplete
	if strings.HasPrefix(cleanToComplete, "\\\"") {
		cleanToComplete = strings.TrimPrefix(cleanToComplete, "\\\"")
	} else if strings.HasPrefix(cleanToComplete, "\"") {
		cleanToComplete = strings.TrimPrefix(cleanToComplete, "\"")
	}

	// log.Printf("handleCompletion: toComplete=%s, cleanToComplete=%s", toComplete, cleanToComplete)
	// Get completion suggestions using the cleaned string
	suggestions, _ := getCompletionSuggestions(cleanToComplete)

	// Print suggestions in the format expected by shell completion
	for _, suggestion := range suggestions {
		fmt.Println(suggestion)
	}

	// End with the completion directive
	fmt.Println(":36") // ShellCompDirectiveNoFileComp | ShellCompDirectiveKeepOrder
}

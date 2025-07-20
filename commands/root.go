package commands

import (
	"fmt"
	"os"
	"strings"

	// "scripto/entities"
	"scripto/internal/args"
	"scripto/internal/execution"
	"scripto/internal/script"
	"scripto/internal/services"
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
				// Load configuration to find the script entity
				configPath, err := storage.GetConfigPath()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to get config path: %v\n", err)
					os.Exit(1)
				}

				config, err := storage.ReadConfig(configPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to read config: %v\n", err)
					os.Exit(1)
				}

				// Find the script in config by file path
				matchResult, err := findScriptByFilePath(config, result.ScriptPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to find script: %v\n", err)
					os.Exit(1)
				}

				// Execute using the unified flow with no arguments
				if err := executeFoundScript(matchResult, []string{}); err != nil {
					fmt.Fprintf(os.Stderr, "Error executing script: %v\n", err)
					os.Exit(1)
				}
			case tui.ActionEdit:
				if err := writeScriptPathForEditor(result.ScriptPath); err != nil {
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

	// Parse script name and arguments, handling -- separator
	scriptName, scriptArgs := parseScriptNameAndArgs(userArgs)

	// Create matcher and find best match
	matcher := script.NewMatcher(config)
	matchResult, err := matcher.Match(scriptName)
	if err != nil {
		return fmt.Errorf("failed to match script: %w", err)
	}

	switch matchResult.Type {
	case script.ExactName, script.PartialCommand:
		return executeFoundScript(matchResult, scriptArgs)
	case script.NoMatch:
		// For no match, use the original full command for backward compatibility
		fullInput := strings.Join(userArgs, " ")
		return handleNoMatch(fullInput, config, configPath)
	default:
		return fmt.Errorf("unknown match type")
	}
}

// parseScriptNameAndArgs separates script name from arguments, handling -- separator
func parseScriptNameAndArgs(userArgs []string) (string, []string) {
	if len(userArgs) == 0 {
		return "", []string{}
	}

	// Find the first -- separator
	separatorIndex := -1
	for i, arg := range userArgs {
		if arg == "--" {
			separatorIndex = i
			break
		}
	}

	if separatorIndex == -1 {
		// No -- separator found, treat first argument as script name and rest as args
		if len(userArgs) == 1 {
			return userArgs[0], []string{}
		}
		return userArgs[0], userArgs[1:]
	}

	// -- separator found
	if separatorIndex == 0 {
		// -- is the first argument, no script name
		return "", userArgs[1:]
	}

	// Script name is everything before --, args are everything after --
	scriptNameParts := userArgs[:separatorIndex]
	scriptArgs := userArgs[separatorIndex+1:]
	
	// Join script name parts with spaces (in case the script name itself has spaces)
	scriptName := strings.Join(scriptNameParts, " ")
	
	return scriptName, scriptArgs
}

// findScriptByFilePath finds a script entity in the config by its file path
func findScriptByFilePath(config storage.Config, filePath string) (*script.MatchResult, error) {
	// Search through all scopes for a script with matching file path
	for scope, scripts := range config {
		for _, scriptEntity := range scripts {
			if scriptEntity.FilePath == filePath {
				// Create a match result for this script
				matchResult := &script.MatchResult{
					Type:       script.ExactName,
					Script:     scriptEntity,
					Confidence: 1.0,
				}
				// Ensure script has correct scope set
				matchResult.Script.Scope = scope
				return matchResult, nil
			}
		}
	}
	return nil, fmt.Errorf("script not found with file path: %s", filePath)
}

// executeFoundScript is the unified executor for all matched scripts
func executeFoundScript(matchResult *script.MatchResult, scriptArgs []string) error {
	// Check if script has a file path (stored as file) or is a command
	if matchResult.Script.FilePath == "" {
		return fmt.Errorf("script has no file path or command content")
	}

	// Read the script file to determine execution type
	content, err := os.ReadFile(matchResult.Script.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read script file %s: %w", matchResult.Script.FilePath, err)
	}

	contentStr := string(content)
	
	// Check if this is an executable script (starts with shebang)
	if strings.HasPrefix(contentStr, "#!") {
		// Executable script - pass arguments directly, no placeholder processing
		return executeExecutableScript(matchResult.Script.FilePath, scriptArgs)
	}

	// Shell command script - process placeholders
	return executeShellCommandScript(matchResult, scriptArgs)
}

// executeExecutableScript handles scripts that start with shebang
func executeExecutableScript(filePath string, args []string) error {
	finalCommand := filePath
	for _, arg := range args {
		if strings.Contains(arg, " ") && !strings.HasPrefix(arg, "\"") {
			finalCommand += fmt.Sprintf(" \"%s\"", arg)
		} else {
			finalCommand += " " + arg
		}
	}
	return executeFinalCommand(finalCommand)
}

// executeShellCommandScript handles shell command scripts with placeholder processing
func executeShellCommandScript(matchResult *script.MatchResult, scriptArgs []string) error {
	processor := args.NewArgumentProcessor(matchResult.Script)

	// Validate arguments (only for shell command scripts)
	if err := processor.ValidateArguments(scriptArgs); err != nil {
		return err
	}

	// Process arguments
	result, err := processor.ProcessArguments(scriptArgs)
	if err != nil {
		return fmt.Errorf("failed to process arguments: %w", err)
	}

	// Handle missing arguments by prompting user
	if len(result.MissingArgs) > 0 {
		formResult, err := tui.RunPlaceholderForm(result.MissingArgs)
		if err != nil {
			return fmt.Errorf("failed to collect placeholder values: %w", err)
		}
		
		if formResult.Cancelled {
			return fmt.Errorf("operation cancelled by user")
		}
		
		missingValues := formResult.Values

		// Update result with prompted values
		for name, value := range missingValues {
			if placeholder, exists := result.Placeholders[name]; exists {
				placeholder.Value = value
				placeholder.Provided = true
				result.Placeholders[name] = placeholder
			}
		}

		// Check if script has positional placeholders
		processor := args.NewArgumentProcessor(matchResult.Script)
		hasPositional, err := processor.HasPositionalPlaceholders()
		if err != nil {
			return fmt.Errorf("failed to check placeholder types: %w", err)
		}

		// Convert values to appropriate argument format and regenerate final command
		var additionalArgs []string
		if hasPositional {
			// For positional scripts, convert named values to positional arguments
			additionalArgs = convertToPositionalArgs(missingValues, result.MissingArgs)
		} else {
			// For named scripts, convert to named arguments
			additionalArgs = convertToArgs(missingValues)
		}

		newResult, err := processor.ProcessArguments(append(scriptArgs, additionalArgs...))
		if err != nil {
			return err
		}
		result.FinalCommand = newResult.FinalCommand
	}

	// Execute the final command
	return executeFinalCommand(result.FinalCommand)
}


// handleNoMatch handles the case when no script matches
func handleNoMatch(input string, config storage.Config, configPath string) error {
	// Use TUI to create and save new script
	service, err := services.NewScriptService()
	if err != nil {
		return fmt.Errorf("failed to create script service: %w", err)
	}

	// Create new scriptObj with command pre-filled
	scriptObj := service.CreateEmptyScript()
	scriptObj.Scope = service.GetCurrentDirectoryScope() // Default to local scope

	// Create a temporary script file with the command
	tempFilePath, err := service.CreateTempScriptFile(input)
	if err != nil {
		return fmt.Errorf("failed to create temp script file: %w", err)
	}
	scriptObj.FilePath = tempFilePath

	// Launch the script editor with a custom prompt message
	fmt.Printf("Command '%s' not found. Create new script?\n", input)
	
	result, err := tui.RunScriptEditor(scriptObj, true)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if result.Cancelled {
		return fmt.Errorf("command not found: %s", input)
	}

	// Save the script using the service
	finalCommand := result.Command
	if finalCommand == "" {
		finalCommand = input
	}

	if err := service.SaveScript(result.Script, finalCommand, nil); err != nil {
		return fmt.Errorf("failed to save script: %w", err)
	}

	fmt.Printf("Saved script successfully\n")

	// Now execute the saved script using the unified flow
	// Create a match result for the newly saved script
	matchResult := &script.MatchResult{
		Type:       script.ExactName,
		Script:     result.Script,
		Confidence: 1.0,
	}

	return executeFoundScript(matchResult, []string{})
}

// convertToArgs converts a map of values to argument format
func convertToArgs(values map[string]string) []string {
	var arguments []string
	for name, value := range values {
		arguments = append(arguments, fmt.Sprintf("--%s=%s", name, value))
	}
	return arguments
}

// convertToPositionalArgs converts named values back to positional arguments based on order
func convertToPositionalArgs(values map[string]string, missingArgs []args.PlaceholderValue) []string {
	var arguments []string
	// Convert based on the order of missing arguments (which preserves original placeholder order)
	for _, placeholder := range missingArgs {
		if value, exists := values[placeholder.Name]; exists {
			arguments = append(arguments, value)
		}
	}
	return arguments
}


// writeScriptPathForEditor writes the script path for editor use
func writeScriptPathForEditor(scriptPath string) error {
	cmdFdPath := os.Getenv("SCRIPTO_CMD_FD")
	if cmdFdPath != "" {
		return execution.WriteScriptPathToFile(scriptPath, cmdFdPath)
	}

	// Fallback to stdout for backward compatibility
	fmt.Print(scriptPath)
	return nil
}

// executeCommand executes a script file with placeholder processing
func executeCommand(finalCommand string) error {
	return executeFinalCommand(finalCommand)
}

// executeFinalCommand executes a script file with the given placeholders
func executeFinalCommand(finalCommand string) error {
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

// convertScriptResultsToSuggestions converts script matcher results to completion suggestions
func convertScriptResultsToSuggestions(results []script.MatchResult, separator string, toComplete string) []string {
	// log.Printf("convertScriptResultsToSuggestions: toComplete=%s, separator=%s", toComplete, separator)
	var suggestions []string
	for _, result := range results {
		if result.Script.Name != "" {
			// Named script - use scope as group name
			description := result.Script.Description
			if description == "" {
				description = result.Script.Description
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

			suggestions = append(suggestions, result.Script.Scope+separator+name+separator+description)
		} else {
			// Unnamed script - show command, use scope as group name
			command := result.Script.FilePath
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

			suggestions = append(suggestions, result.Script.Scope+separator+displayCommand+separator+result.Script.FilePath)
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

package commands

import (
	"fmt"
	"os"
	"strings"

	"scripto/internal/script"
	"scripto/internal/services"
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
		terminalService := services.NewTerminalService()

		if len(args) == 0 {
			// No arguments - launch TUI with main list screen
			result, err := tui.RunApp(tui.StartAtMainList)
			if err != nil {
				fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
				os.Exit(int(services.ExitCodeError))
			}

			if result.ExitCode == int(services.ExitCodeExternalEditor) {
				// Exit code 4 means external edit was requested
				if err := terminalService.EditScriptExternal(result.ScriptPath); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(int(services.ExitCodeError))
				}
			} else if result.ExitCode == int(services.ExitCodeSuccess) {
				// Exit code 0 means script execution was requested
				// The script path is in result.ScriptPath
				// This is handled by the shell wrapper
			}

			os.Exit(result.ExitCode)
		}

		// Check if first arg is "add" for history screen shortcut
		if args[0] == "add" {
			result, err := tui.RunApp(tui.StartAtHistory)
			if err != nil {
				fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
				os.Exit(int(services.ExitCodeError))
			}

			if result.ExitCode == int(services.ExitCodeExternalEditor) {
				// Exit code 4 means external edit was requested
				if err := terminalService.EditScriptExternal(result.ScriptPath); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(int(services.ExitCodeError))
				}
			}

			os.Exit(result.ExitCode)
		}

		// Execute script matching logic
		if err := executeScript(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(int(services.ExitCodeError))
		}
	},
}

// executeScript handles the main script execution logic
func executeScript(userArgs []string) error {
	// Create script service
	scriptService, err := services.NewScriptService()
	if err != nil {
		return fmt.Errorf("failed to create script service: %w", err)
	}

	// Parse script name and arguments, handling -- separator
	scriptName, scriptArgs := parseScriptNameAndArgs(userArgs)

	// Create matcher and find best match
	matchResult, err := scriptService.Match(scriptName)
	if err != nil {
		return fmt.Errorf("failed to match script: %w", err)
	}

	switch matchResult.Type {
	case script.ExactName, script.PartialCommand:
		return executeFoundScript(scriptService, matchResult, scriptArgs)
	case script.NoMatch:
		// For no match, use the original full command for backward compatibility
		fullInput := strings.Join(userArgs, " ")
		return handleNoMatch(fullInput, scriptService)
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

// executeFoundScript is the unified executor for all matched scripts
func executeFoundScript(_ *services.ScriptService, matchResult *script.MatchResult, scriptArgs []string) error {
	executionService := services.NewExecutionService()
	terminalService := services.NewTerminalService()

	processingResult, err := executionService.ProcessScriptArguments(matchResult, scriptArgs)
	if err != nil {
		return err
	}

	if !processingResult.NeedsPlaceholderForm {
		finalCommand, err := executionService.PrepareDirectExecution(processingResult)
		if err != nil {
			return err
		}
		return terminalService.ExecuteScript(finalCommand)
	}

	// Show placeholder form
	formResult, err := tui.RunPlaceholderForm(processingResult.Placeholders)
	if err != nil {
		return fmt.Errorf("failed to collect placeholder values: %w", err)
	}

	if formResult.Cancelled {
		return fmt.Errorf("operation cancelled by user")
	}

	// Execute with placeholder values
	finalCommand, err := executionService.PrepareExecution(matchResult, scriptArgs, formResult.Values)
	if err != nil {
		return err
	}
	return terminalService.ExecuteScript(finalCommand)
}


// handleNoMatch handles the case when no script matches
func handleNoMatch(input string, scriptService *services.ScriptService) error {
	// Create new scriptObj with command pre-filled
	scriptObj := scriptService.CreateEmptyScript()
	scriptObj.Scope = scriptService.GetCurrentDirectoryScope() // Default to local scope

	// Create a temporary script file with the command
	tempFilePath, err := scriptService.CreateTempScriptFile(input)
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

	if err := scriptService.SaveScript(result.Script, finalCommand, nil); err != nil {
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

	return executeFoundScript(scriptService, matchResult, []string{})
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

	// Create script service
	scriptService, err := services.NewScriptService()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Always find all scripts and filter by prefix
	allScripts, err := scriptService.FindAllScripts()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// log.Printf("getCompletionSuggestions: toComplete=%s, allScripts count=%d", toComplete, len(allScripts))
	suggestions := convertScriptResultsToSuggestions(allScripts, separator, toComplete)
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func Execute() {
	// Sync shortcuts before executing any command
	syncShortcutsQuietly()

	// Get command line arguments
	cmdArgs := os.Args[1:]

	// If no arguments, run root command normally
	if len(cmdArgs) == 0 {
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(int(services.ExitCodeError))
		}
		// Built-in command completed successfully - exit with code 3
		os.Exit(int(services.ExitCodeBuiltinComplete))
	}

	firstArg := cmdArgs[0]

	// Handle completion specially to avoid Cobra's built-in suggestions
	if firstArg == "__complete" {
		handleCompletion(cmdArgs[1:])
		// Completion completed successfully - exit with code 3
		os.Exit(int(services.ExitCodeBuiltinComplete))
	}

	// Check if the first argument is a known subcommand
	knownSubcommands := []string{"add", "completion", "install", "help", "--help", "-h"}

	for _, subcmd := range knownSubcommands {
		if firstArg == subcmd {
			// This is a known subcommand, delegate to Cobra
			if err := rootCmd.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(int(services.ExitCodeError))
			}
			// Built-in command completed successfully - exit with code 3
			os.Exit(int(services.ExitCodeBuiltinComplete))
		}
	}

	// Not a known subcommand, treat as script execution
	if err := executeScript(cmdArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(int(services.ExitCodeError))
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

// syncShortcutsQuietly synchronizes shortcuts without printing errors to avoid interfering with completion
func syncShortcutsQuietly() {
	service, err := services.NewScriptService()
	if err != nil {
		return // Silently ignore initialization errors
	}

	// Silently sync shortcuts - errors are ignored to avoid interfering with normal operation
	_ = service.SyncShortcuts()
}

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

		if err := executeScript(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(int(services.ExitCodeError))
		}
	},
}

func executeScript(userArgs []string) error {
	scriptService, err := services.NewScriptService()
	if err != nil {
		return fmt.Errorf("failed to create script service: %w", err)
	}

	scriptName, scriptArgs := parseScriptNameAndArgs(userArgs)

	matchResult, err := scriptService.Match(scriptName)
	if err != nil {
		return fmt.Errorf("failed to match script: %w", err)
	}

	switch matchResult.Type {
	case script.ExactName, script.PartialCommand:
		return executeFoundScript(scriptService, matchResult, scriptArgs)
	case script.NoMatch:
		fullInput := strings.Join(userArgs, " ")
		return handleNoMatch(fullInput, scriptService)
	default:
		return fmt.Errorf("unknown match type")
	}
}

func parseScriptNameAndArgs(userArgs []string) (string, []string) {
	if len(userArgs) == 0 {
		return "", []string{}
	}

	separatorIndex := -1
	for i, arg := range userArgs {
		if arg == "--" {
			separatorIndex = i
			break
		}
	}

	if separatorIndex == -1 {
		if len(userArgs) == 1 {
			return userArgs[0], []string{}
		}
		return userArgs[0], userArgs[1:]
	}

	if separatorIndex == 0 {
		return "", userArgs[1:]
	}

	scriptNameParts := userArgs[:separatorIndex]
	scriptArgs := userArgs[separatorIndex+1:]

	scriptName := strings.Join(scriptNameParts, " ")

	return scriptName, scriptArgs
}

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

	formResult, err := tui.RunPlaceholderForm(processingResult.Placeholders)
	if err != nil {
		return fmt.Errorf("failed to collect placeholder values: %w", err)
	}

	if formResult.Cancelled {
		return fmt.Errorf("operation cancelled by user")
	}

	finalCommand, err := executionService.PrepareExecution(matchResult, scriptArgs, formResult.Values)
	if err != nil {
		return err
	}
	return terminalService.ExecuteScript(finalCommand)
}


func handleNoMatch(input string, scriptService *services.ScriptService) error {
	scriptObj := scriptService.CreateEmptyScript()

	tempFilePath, err := scriptService.CreateTempScriptFile(input)
	if err != nil {
		return fmt.Errorf("failed to create temp script file: %w", err)
	}
	scriptObj.FilePath = tempFilePath

	fmt.Printf("Command '%s' not found. Create new script?\n", input)

	result, err := tui.RunScriptEditor(scriptObj, true)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if result.Cancelled {
		return fmt.Errorf("command not found: %s", input)
	}

	finalCommand := result.Command
	if finalCommand == "" {
		finalCommand = input
	}

	if err := scriptService.SaveScript(result.Script, finalCommand, nil); err != nil {
		return fmt.Errorf("failed to save script: %w", err)
	}

	fmt.Printf("Saved script successfully\n")

	matchResult := &script.MatchResult{
		Type:       script.ExactName,
		Script:     result.Script,
		Confidence: 1.0,
	}

	return executeFoundScript(scriptService, matchResult, []string{})
}


func convertScriptResultsToSuggestions(results []script.MatchResult, separator string, toComplete string) []string {
	var suggestions []string
	for _, result := range results {
		if result.Script.Name != "" {
			description := result.Script.Description
			if description == "" {
				description = result.Script.Description
			}

			name := result.Script.Name
			if toComplete != "" {
				if !strings.HasPrefix(name, toComplete) {
				}
			}

			suggestions = append(suggestions, result.Script.Scope+separator+name+separator+description)
		} else {
			command := result.Script.FilePath
			displayCommand := command

			if toComplete != "" {
				if !strings.HasPrefix(command, toComplete) {
				}

				displayCommand = command
			}

			suggestions = append(suggestions, result.Script.Scope+separator+displayCommand+separator+result.Script.FilePath)
		}
	}
	return suggestions
}

func getCompletionSuggestions(toComplete string) ([]string, cobra.ShellCompDirective) {
	separator := "\x1F"

	scriptService, err := services.NewScriptService()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	allScripts, err := scriptService.FindAllScripts()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	suggestions := convertScriptResultsToSuggestions(allScripts, separator, toComplete)
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func Execute() {
	syncShortcutsQuietly()

	cmdArgs := os.Args[1:]

	if len(cmdArgs) == 0 {
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(int(services.ExitCodeError))
		}
		os.Exit(int(services.ExitCodeBuiltinComplete))
	}

	firstArg := cmdArgs[0]

	if firstArg == "__complete" {
		handleCompletion(cmdArgs[1:])
		os.Exit(int(services.ExitCodeBuiltinComplete))
	}

	knownSubcommands := []string{"add", "completion", "install", "help", "--help", "-h"}

	for _, subcmd := range knownSubcommands {
		if firstArg == subcmd {
			if err := rootCmd.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(int(services.ExitCodeError))
			}
			os.Exit(int(services.ExitCodeBuiltinComplete))
		}
	}

	if err := executeScript(cmdArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(int(services.ExitCodeError))
	}
}

func handleCompletion(args []string) {
	var toComplete string

	if len(args) > 0 && args[len(args)-1] == "" {
		toComplete = strings.Join(args[:len(args)-1], " ")
	} else if len(args) > 0 {
		toComplete = strings.Join(args, " ")
	}

	cleanToComplete := toComplete
	if strings.HasPrefix(cleanToComplete, "\\\"") {
		cleanToComplete = strings.TrimPrefix(cleanToComplete, "\\\"")
	} else if strings.HasPrefix(cleanToComplete, "\"") {
		cleanToComplete = strings.TrimPrefix(cleanToComplete, "\"")
	}

	suggestions, _ := getCompletionSuggestions(cleanToComplete)

	for _, suggestion := range suggestions {
		fmt.Println(suggestion)
	}

}

func syncShortcutsQuietly() {
	service, err := services.NewScriptService()
	if err != nil {
	}

	_ = service.SyncShortcuts()
}

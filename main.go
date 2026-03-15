package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"scripto/entities"
	"scripto/internal/services"
	"scripto/internal/storage"
	"scripto/internal/tui"
)

//go:embed commands/scripts/completion.zsh
var completionZsh string

var version = "dev"

func configureLogger() {
	logFilePath := "/tmp/scripto.log"
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating log file: %v\n", err)
		os.Exit(1)
	}
	log.SetOutput(logFile)
}

func main() {
	configureLogger()

	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("scripto version %s\n", version)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--migrate" {
		if err := runMigrate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	container, err := services.NewContainer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	handleCommand(container, os.Args[1:])
}

func handleCommand(container *services.Container, args []string) {
	if len(args) > 0 && args[0] == "completion" {
		fmt.Print(completionZsh)
		return
	}

	if len(args) > 0 && args[0] == "__complete" {
		handleCompletion(container, args[1:])
		container.TerminalService.ExecuteCommand(container.TerminalService.PrepareExit(3))
	}

	if len(args) > 0 && args[0] == "install" {
		if err := handleInstall(container); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err.Error())
			container.TerminalService.ExecuteCommand(container.TerminalService.PrepareExit(1))
		}
		return
	}

	if len(args) == 0 {
		if err := tui.RunApp(container, tui.ShowMainListRequest{}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err.Error())
			container.TerminalService.ExecuteCommand(container.TerminalService.PrepareExit(1))
		}
		return
	}

	if args[0] == "add" {
		if err := tui.RunApp(container, tui.ShowAddScreenRequest{}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err.Error())
			container.TerminalService.ExecuteCommand(container.TerminalService.PrepareExit(1))
		}
		return
	}

	if err := executeScript(container, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err.Error())
		container.TerminalService.ExecuteCommand(container.TerminalService.PrepareExit(1))
		return
	}
}

func runMigrate() error {
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	config, err := storage.ReadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := storage.WriteConfig(configPath, config); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Println("Migration complete")
	return nil
}

func handleInstall(container *services.Container) error {
	return container.ScriptService.SyncShortcuts()
}

func executeScript(container *services.Container, userArgs []string) error {
	scriptName, scriptArgs := parseScriptNameAndArgs(userArgs)

	matchResult, err := container.ScriptService.Match(scriptName)
	if err != nil {
		return fmt.Errorf("failed to match script: %w", err)
	}

	if matchResult != nil {
		return executeFoundScript(container, matchResult, scriptArgs)
	}

	fullInput := strings.Join(userArgs, " ")
	return handleNoMatch(container, fullInput)
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

func executeFoundScript(container *services.Container, scriptEnt *entities.Script, scriptArgs []string) error {
	return tui.RunApp(container, tui.ExecuteScriptRequest{Script: scriptEnt, ScriptArgs: scriptArgs})
}

func handleNoMatch(container *services.Container, input string) error {
	scriptObj := container.ScriptService.CreateEmptyScript()

	tempFilePath, err := container.ScriptService.CreateTempScriptFile(input)
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

	if err := container.ScriptService.SaveScript(result.Script, finalCommand, nil); err != nil {
		return fmt.Errorf("failed to save script: %w", err)
	}

	fmt.Printf("Saved script successfully\n")

	return executeFoundScript(container, result.Script, []string{})
}

func handleCompletion(container *services.Container, args []string) {
	showAll := false
	for _, arg := range args {
		if arg == "--more" {
			showAll = true
			break
		}
	}
	// var toComplete string

	// if len(args) > 0 && args[len(args)-1] == "" {
	// 	toComplete = strings.Join(args[:len(args)-1], " ")
	// } else if len(args) > 0 {
	// 	toComplete = strings.Join(args, " ")
	// }

	// cleanToComplete := toComplete
	// if strings.HasPrefix(cleanToComplete, "\\\"") {
	// 	cleanToComplete = strings.TrimPrefix(cleanToComplete, "\\\"")
	// } else if strings.HasPrefix(cleanToComplete, "\"") {
	// 	cleanToComplete = strings.TrimPrefix(cleanToComplete, "\"")
	// }

	suggestions := getCompletionSuggestions(container, showAll)

	for _, suggestion := range suggestions {
		fmt.Println(suggestion)
	}
}

func getCompletionSuggestions(container *services.Container, showAll bool) []string {
	separator := "\x1F"

	var allScripts []*entities.Script
	var err error
	if showAll {
		allScripts, err = container.ScriptService.FindAllScopesScripts()
	} else {
		allScripts, err = container.ScriptService.FindAllScripts()
	}
	if err != nil {
		return nil
	}

	var frecencyScores map[string]float64
	if container.ExecutionHistoryService != nil {
		frecencyScores = container.ExecutionHistoryService.GetFrecencyScores()
	}
	if frecencyScores == nil {
		frecencyScores = map[string]float64{}
	}

	var scopeOrder []string
	scopeScripts := make(map[string][]*entities.Script)
	for _, script := range allScripts {
		if _, exists := scopeScripts[script.Scope]; !exists {
			scopeOrder = append(scopeOrder, script.Scope)
		}
		scopeScripts[script.Scope] = append(scopeScripts[script.Scope], script)
	}

	for scope := range scopeScripts {
		sort.Slice(scopeScripts[scope], func(i, j int) bool {
			return frecencyScores[scopeScripts[scope][i].ID] > frecencyScores[scopeScripts[scope][j].ID]
		})
	}

	var sorted []*entities.Script
	for _, scope := range scopeOrder {
		sorted = append(sorted, scopeScripts[scope]...)
	}

	return convertScriptResultsToSuggestions(sorted, separator)
}

func convertScriptResultsToSuggestions(results []*entities.Script, separator string) []string {
	var suggestions []string
	for _, script := range results {
		if script.Name != "" {
			description := script.Description
			if description == "" {
				description = script.Description
			}

			name := script.Name
			scopeColor := tui.GetScopeColorHex(script.Scope)
			suggestions = append(suggestions, script.Scope+separator+name+separator+description+separator+scopeColor)
		} else {
			command := script.FilePath
			displayCommand := command
			scopeColor := tui.GetScopeColorHex(script.Scope)
			suggestions = append(suggestions, script.Scope+separator+displayCommand+separator+script.FilePath+separator+scopeColor)
		}
	}
	return suggestions
}

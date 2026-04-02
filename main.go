package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"scripto/entities"
	"scripto/internal/services"
	"scripto/internal/storage"
	"scripto/internal/tui"
	"scripto/internal/tui/colors"

	"github.com/charmbracelet/lipgloss"
	xterm "github.com/charmbracelet/x/term"
)

//go:embed commands/scripts/completion.zsh
var completionZsh string

//go:embed commands/scripts/scripto.zsh
var zshFunctionContent string

//go:embed commands/scripts/completion-alias.zsh
var aliasCompletionTemplate string

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
		if err := handleInstall(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err.Error())
			os.Exit(1)
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

func terminalWidth() int {
	width, _, err := xterm.GetSize(os.Stderr.Fd())
	if err != nil || width <= 0 {
		return 80
	}
	return width
}

func printInstallHeader() {
	width := terminalWidth()
	titleStyle := lipgloss.NewStyle().Foreground(colors.Primary).Bold(true)
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(false).
		BorderRight(false).
		BorderForeground(colors.Primary).
		PaddingLeft(1).
		Width(width - 2)
	fmt.Fprintln(os.Stderr, boxStyle.Render(titleStyle.Render("Installing Scripto")))
}

func printInstallStep(action, detail string) {
	arrowStyle := lipgloss.NewStyle().Foreground(colors.Primary).Bold(true)
	actionStyle := lipgloss.NewStyle().Foreground(colors.Primary)
	detailStyle := lipgloss.NewStyle().Foreground(colors.MutedText)
	fmt.Fprintf(os.Stderr, "  %s %s %s\n",
		arrowStyle.Render("→"),
		actionStyle.Render(action),
		detailStyle.Render(detail),
	)
}

func printInstallNote(note string) {
	noteStyle := lipgloss.NewStyle().Foreground(colors.MutedText)
	fmt.Fprintln(os.Stderr, noteStyle.Render("  "+note))
}

func handleInstall(args []string) error {
	turbo := false
	alias := ""
	for i, arg := range args {
		if arg == "--turbo" {
			turbo = true
		} else if arg == "--alias" && i+1 < len(args) {
			alias = args[i+1]
		}
	}

	printInstallHeader()

	if err := installShellIntegration(); err != nil {
		return err
	}

	if turbo {
		return installAlias("sc")
	}
	if alias != "" {
		return installAlias(alias)
	}

	fmt.Fprintln(os.Stderr)
	printInstallNote("Restart your shell or run: source ~/.zshrc")
	return nil
}

func installShellIntegration() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	scriptoDir := filepath.Join(homeDir, ".scripto")
	if err := os.MkdirAll(scriptoDir, 0755); err != nil {
		return fmt.Errorf("failed to create .scripto directory: %w", err)
	}
	printInstallStep("Created directory", scriptoDir)

	zshFile := filepath.Join(scriptoDir, "scripto.zsh")
	if err := os.WriteFile(zshFile, []byte(zshFunctionContent), 0644); err != nil {
		return fmt.Errorf("failed to write scripto.zsh: %w", err)
	}
	printInstallStep("Wrote shell function", zshFile)

	zshrcPath := filepath.Join(homeDir, ".zshrc")
	sourceLine := "source ~/.scripto/scripto.zsh"
	if err := addLineToZshrc(zshrcPath, sourceLine); err != nil {
		return fmt.Errorf("failed to update .zshrc: %w", err)
	}
	printInstallStep("Updated", zshrcPath)

	return nil
}

func installAlias(aliasName string) error {
	if !isValidAliasName(aliasName) {
		return fmt.Errorf("invalid alias name: %s (must be alphanumeric with underscores, no reserved words)", aliasName)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	scriptoDir := filepath.Join(homeDir, ".scripto")
	completionFile := filepath.Join(scriptoDir, fmt.Sprintf("%s_completion.zsh", aliasName))
	if err := generateAliasCompletion(aliasName, completionFile); err != nil {
		return fmt.Errorf("failed to generate completion file: %w", err)
	}
	printInstallStep("Wrote alias completion", completionFile)

	zshrcPath := filepath.Join(homeDir, ".zshrc")
	if err := addLineToZshrc(zshrcPath, fmt.Sprintf("alias %s='scripto'", aliasName)); err != nil {
		return fmt.Errorf("failed to add alias to .zshrc: %w", err)
	}
	printInstallStep(fmt.Sprintf("Added alias '%s'", aliasName), "→ scripto")

	if err := addLineToZshrc(zshrcPath, fmt.Sprintf("source ~/.scripto/%s_completion.zsh", aliasName)); err != nil {
		return fmt.Errorf("failed to add completion source to .zshrc: %w", err)
	}
	printInstallStep("Added alias completion source", zshrcPath)

	fmt.Fprintln(os.Stderr)
	printInstallNote("Restart your shell or run: source ~/.zshrc")
	return nil
}

func generateAliasCompletion(aliasName, outputPath string) error {
	tmpl, err := template.New("completion").Parse(aliasCompletionTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create completion file: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, struct{ Alias string }{Alias: aliasName})
}

func addLineToZshrc(zshrcPath, line string) error {
	content, err := os.ReadFile(zshrcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read .zshrc: %w", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, line) {
		return nil
	}

	if len(contentStr) > 0 && !strings.HasSuffix(contentStr, "\n") {
		contentStr += "\n"
	}
	contentStr += line + "\n"

	if err := os.WriteFile(zshrcPath, []byte(contentStr), 0644); err != nil {
		return fmt.Errorf("failed to write .zshrc: %w", err)
	}
	return nil
}

func isValidAliasName(name string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	if !matched {
		return false
	}

	reserved := []string{
		"if", "then", "else", "elif", "fi", "case", "esac", "for", "while", "until", "do", "done",
		"function", "select", "time", "coproc", "in", "return", "exit", "break", "continue",
		"alias", "unalias", "export", "readonly", "local", "declare", "typeset", "let", "eval",
		"exec", "source", "builtin", "command", "type", "which", "where", "whence",
	}
	for _, word := range reserved {
		if name == word {
			return false
		}
	}
	return true
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
	return tui.RunApp(container, tui.ShowScriptEditorRequest{
		Script:         scriptObj,
		InitialCommand: input,
	})
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

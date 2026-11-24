package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/services"
	"github.com/vsuhanov/scripto/internal/storage"
	"github.com/vsuhanov/scripto/internal/templatex"
	"github.com/vsuhanov/scripto/internal/tui"
	"github.com/vsuhanov/scripto/internal/tui/colors"
	"github.com/vsuhanov/scripto/internal/utils"

	"github.com/charmbracelet/lipgloss"
	xterm "github.com/charmbracelet/x/term"
)

//go:embed commands/scripts/completion.zsh
var completionZsh string

//go:embed commands/scripts/scripto.zsh
var zshFunctionContent string

//go:embed commands/scripts/completion-alias.zsh
var aliasCompletionTemplate string

//go:embed commands/skill/SKILL.md
var skillMD string

var version = "dev"
var commit = "unknown"

func configureLogger() {
	logFilePath := "/tmp/scripto.log"
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.SetOutput(io.Discard)
		return
	}
	_ = os.Chmod(logFilePath, 0600)
	log.SetOutput(logFile)
}

func main() {
	configureLogger()

	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		binaryPath, err := os.Executable()
		if err != nil {
			binaryPath = "unknown"
		}
		fmt.Printf("scripto version %s\n", version)
		fmt.Printf("binary: %s\n", binaryPath)
		fmt.Printf("commit: %s\n", commit)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--migrate" {
		if err := runMigrate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "__record-history" {
		os.Exit(handleRecordHistory(os.Args[2:]))
	}

	if len(os.Args) > 1 && os.Args[1] == "__record-execution" {
		os.Exit(handleRecordExecution(os.Args[2:]))
	}

	container, err := services.NewContainer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	code := handleCommand(container, os.Args[1:])
	container.Close()
	os.Exit(code)
}

func runTui(container *services.Container, request tui.TuiRequest) int {
	code, err := tui.RunApp(container, request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return code
}

func handleCommand(container *services.Container, args []string) int {
	if len(args) > 0 && args[0] == "completion" {
		fmt.Print(completionZsh)
		return 0
	}

	if len(args) > 0 && args[0] == "__complete" {
		handleCompletion(container, args[1:])
		return 3
	}

	if len(args) > 0 && args[0] == "install" {
		if err := handleInstall(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	if len(args) > 0 && args[0] == "cli" {
		return handleCli(container, args[1:])
	}

	if len(args) == 0 {
		return runTui(container, tui.ShowMainListRequest{})
	}

	if args[0] == "add" {
		return runTui(container, tui.ShowAddScreenRequest{})
	}

	return executeScript(container, args)
}

func runMigrate() error {
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	config, err := storage.UpdateConfig(configPath, func(storage.Config) error { return nil })
	if err != nil {
		return fmt.Errorf("failed to migrate config: %w", err)
	}
	if err := storage.SyncShortcuts(config); err != nil {
		return fmt.Errorf("failed to sync shortcuts: %w", err)
	}
	fmt.Println("Migration complete")
	return nil
}

func syncShortcuts() error {
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	config, err := storage.ReadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	return storage.SyncShortcuts(config)
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
	if len(args) > 0 && args[0] == "skill" {
		return handleInstallSkill(args[1:])
	}

	turbo := false
	alias := ""
	trackHistory := false
	noTrackHistory := false
	for i, arg := range args {
		if arg == "--turbo" {
			turbo = true
		} else if arg == "--alias" && i+1 < len(args) {
			alias = args[i+1]
		} else if arg == "--track-history" {
			trackHistory = true
		} else if arg == "--no-track-history" {
			noTrackHistory = true
		}
	}

	printInstallHeader()

	if err := installShellIntegration(); err != nil {
		return err
	}

	if !noTrackHistory {
		if !trackHistory && utils.IsStdinTerminal() {
			trackHistory = promptYesNo("Track shell command history? Records every command you run (and where) into scripto.")
		}
		if trackHistory {
			if err := installHistoryTracking(); err != nil {
				return err
			}
		}
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

	if err := syncShortcuts(); err != nil {
		return fmt.Errorf("failed to sync global shortcuts: %w", err)
	}
	printInstallStep("Synced global shortcuts", fmt.Sprintf("separator '%s'", storage.ShortcutSeparator()))

	return nil
}

func promptYesNo(question string) bool {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %s [y/N]: ", question)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func installHistoryTracking() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	zshrcPath := filepath.Join(homeDir, ".zshrc")
	if err := addLineToZshrc(zshrcPath, "export SCRIPTO_TRACK_HISTORY=1"); err != nil {
		return fmt.Errorf("failed to enable history tracking: %w", err)
	}
	printInstallStep("Enabled shell history tracking", zshrcPath)
	printInstallNote("Browse it with `h` in the scripto TUI")
	printInstallNote("To turn it off, remove `export SCRIPTO_TRACK_HISTORY=1` from ~/.zshrc")
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

func executeScript(container *services.Container, userArgs []string) int {
	scriptName, scriptArgs := parseScriptNameAndArgs(userArgs)

	matchResult, err := container.ScriptService.Match(scriptName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to match script: %v\n", err)
		return 1
	}

	if matchResult != nil {
		return runTui(container, tui.ExecuteScriptRequest{Script: matchResult, ScriptArgs: scriptArgs})
	}

	allScopeMatches, err := container.ScriptService.MatchAllScopes(scriptName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to match script across scopes: %v\n", err)
		return 1
	}

	if len(allScopeMatches) == 0 {
		return runTui(container, tui.ShowScriptEditorRequest{
			Script:         container.ScriptService.CreateEmptyScript(),
			InitialCommand: strings.Join(userArgs, " "),
		})
	}

	if len(allScopeMatches) == 1 {
		script := allScopeMatches[0]
		markContextualIfApplicable(container, script)
		return runTui(container, tui.ExecuteScriptRequest{Script: script, ScriptArgs: scriptArgs})
	}

	return runTui(container, tui.ShowMainListWithSearchRequest{SearchText: scriptName})
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
		return strings.Join(userArgs, " "), []string{}
	}

	if separatorIndex == 0 {
		return "", userArgs[1:]
	}

	scriptNameParts := userArgs[:separatorIndex]
	scriptArgs := userArgs[separatorIndex+1:]

	scriptName := strings.Join(scriptNameParts, " ")

	return scriptName, scriptArgs
}

func markContextualIfApplicable(container *services.Container, script *entities.Script) {
	if container.ExecutionHistoryService == nil || script.ID == "" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	ids, err := container.ExecutionHistoryService.GetScriptIDsRunFromDirectory(cwd)
	if err != nil {
		return
	}
	for _, id := range ids {
		if id == script.ID {
			script.OriginalScope = script.Scope
			script.Scope = cwd
			return
		}
	}
}


func handleCompletion(container *services.Container, args []string) {
	if len(args) > 0 && args[0] == "--args" {
		for _, suggestion := range getArgsCompletionSuggestions(container, strings.Join(args[1:], " ")) {
			fmt.Println(suggestion)
		}
		return
	}

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
		sort.SliceStable(scopeScripts[scope], func(i, j int) bool {
			return frecencyScores[scopeScripts[scope][i].ID] > frecencyScores[scopeScripts[scope][j].ID]
		})
	}

	var sorted []*entities.Script
	for _, scope := range scopeOrder {
		sorted = append(sorted, scopeScripts[scope]...)
	}

	return convertScriptResultsToSuggestions(sorted, separator)
}

func getArgsCompletionSuggestions(container *services.Container, scriptName string) []string {
	separator := "\x1F"
	if scriptName == "" || container.ExecutionHistoryService == nil {
		return nil
	}

	script, err := container.ScriptService.Match(scriptName)
	if err != nil || script == nil {
		matches, mErr := container.ScriptService.MatchAllScopes(scriptName)
		if mErr != nil || len(matches) != 1 {
			return nil
		}
		script = matches[0]
	}
	if script.ID == "" {
		return nil
	}

	records, err := container.ExecutionHistoryService.GetScriptHistory(script.ID, 100)
	if err != nil {
		return nil
	}

	var keyOrder []string
	if script.FilePath != "" {
		if content, rErr := os.ReadFile(script.FilePath); rErr == nil {
			if metas, mErr := templatex.ExtractVariables(strings.TrimSpace(string(content))); mErr == nil {
				for _, meta := range metas {
					keyOrder = append(keyOrder, meta.Name)
				}
			}
		}
	}

	seen := map[string]bool{}
	var suggestions []string
	for _, record := range records {
		argsStr := buildArgsString(record.PlaceholderValues, keyOrder)
		if argsStr == "" || seen[argsStr] {
			continue
		}
		seen[argsStr] = true
		desc := time.Unix(record.ExecutionTimestamp, 0).Format("2006-01-02 15:04")
		suggestions = append(suggestions, argsStr+separator+desc)
	}
	return suggestions
}

func buildArgsString(values map[string]string, keyOrder []string) string {
	var parts []string
	used := map[string]bool{}
	for _, key := range keyOrder {
		if val, ok := values[key]; ok && val != "" {
			parts = append(parts, utils.ShellQuoteIfNeeded(fmt.Sprintf("--%s=%s", key, val)))
			used[key] = true
		}
	}
	var rest []string
	for key, val := range values {
		if !used[key] && val != "" {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		parts = append(parts, utils.ShellQuoteIfNeeded(fmt.Sprintf("--%s=%s", key, values[key])))
	}
	return strings.Join(parts, " ")
}

func convertScriptResultsToSuggestions(results []*entities.Script, separator string) []string {
	var suggestions []string
	for _, script := range results {
		scopeColor := tui.GetScopeColorHex(script.Scope)
		if script.Name != "" {
			suggestions = append(suggestions, script.Scope+separator+script.Name+separator+script.Description+separator+scopeColor)
		} else {
			suggestions = append(suggestions, script.Scope+separator+script.FilePath+separator+script.FilePath+separator+scopeColor)
		}
	}
	return suggestions
}

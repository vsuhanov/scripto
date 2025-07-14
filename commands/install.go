package commands

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed scripts/scripto.zsh
var zshFunctionContent string

//go:embed scripts/completion-alias.zsh
var aliasCompletionTemplate string

var (
	turboFlag bool
	aliasFlag string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install scripto shell integration",
	Long:  "Install shell functions to enable scripto to run commands in the current shell context",
	RunE: func(cmd *cobra.Command, args []string) error {
		// First, always install the main scripto integration
		if err := installShellIntegration(); err != nil {
			return err
		}

		// Handle alias installation
		if turboFlag {
			return installAlias("sc")
		}
		if aliasFlag != "" {
			return installAlias(aliasFlag)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().BoolVar(&turboFlag, "turbo", false, "Install with 'sc' alias for faster access")
	installCmd.Flags().StringVar(&aliasFlag, "alias", "", "Install with custom alias name")
}

func installShellIntegration() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	scriptoDir := filepath.Join(homeDir, ".scripto")

	// Create ~/.scripto directory if it doesn't exist
	if err := os.MkdirAll(scriptoDir, 0755); err != nil {
		return fmt.Errorf("failed to create .scripto directory: %w", err)
	}

	// Write scripto.zsh file
	zshFile := filepath.Join(scriptoDir, "scripto.zsh")
	if err := os.WriteFile(zshFile, []byte(zshFunctionContent), 0644); err != nil {
		return fmt.Errorf("failed to write scripto.zsh: %w", err)
	}

	// Add source line to ~/.zshrc
	zshrcPath := filepath.Join(homeDir, ".zshrc")
	sourceLine := "source ~/.scripto/scripto.zsh"

	if err := addSourceLineToZshrc(zshrcPath, sourceLine); err != nil {
		return fmt.Errorf("failed to update .zshrc: %w", err)
	}

	fmt.Println("Shell integration installed successfully!")
	fmt.Println("Please restart your shell or run: source ~/.zshrc")

	return nil
}

func addSourceLineToZshrc(zshrcPath, sourceLine string) error {
	// Read existing .zshrc content (if it exists)
	var content []byte
	var err error

	content, err = os.ReadFile(zshrcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read .zshrc: %w", err)
	}

	contentStr := string(content)

	// Check if the source line already exists
	if strings.Contains(contentStr, sourceLine) {
		fmt.Println("Source line already exists in .zshrc")
		return nil
	}

	// Append the source line
	if len(contentStr) > 0 && !strings.HasSuffix(contentStr, "\n") {
		contentStr += "\n"
	}
	contentStr += sourceLine + "\n"

	// Write back to .zshrc
	if err := os.WriteFile(zshrcPath, []byte(contentStr), 0644); err != nil {
		return fmt.Errorf("failed to write .zshrc: %w", err)
	}

	return nil
}

func installAlias(aliasName string) error {
	// Validate alias name
	if !isValidAliasName(aliasName) {
		return fmt.Errorf("invalid alias name: %s (must be alphanumeric with underscores, no reserved words)", aliasName)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	scriptoDir := filepath.Join(homeDir, ".scripto")
	zshrcPath := filepath.Join(homeDir, ".zshrc")

	// Generate completion file for the alias
	completionFile := filepath.Join(scriptoDir, fmt.Sprintf("%s_completion.zsh", aliasName))
	if err := generateAliasCompletion(aliasName, completionFile); err != nil {
		return fmt.Errorf("failed to generate completion file: %w", err)
	}

	// Add alias and completion sourcing to .zshrc
	aliasLine := fmt.Sprintf("alias %s='scripto'", aliasName)
	sourceLine := fmt.Sprintf("source ~/.scripto/%s_completion.zsh", aliasName)

	if err := addLineToZshrc(zshrcPath, aliasLine); err != nil {
		return fmt.Errorf("failed to add alias to .zshrc: %w", err)
	}

	if err := addLineToZshrc(zshrcPath, sourceLine); err != nil {
		return fmt.Errorf("failed to add completion source to .zshrc: %w", err)
	}

	fmt.Printf("Alias '%s' installed successfully!\n", aliasName)
	fmt.Println("Please restart your shell or run: source ~/.zshrc")

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

	data := struct {
		Alias string
	}{
		Alias: aliasName,
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func isValidAliasName(name string) bool {
	// Check for basic format: alphanumeric + underscore, starting with letter or underscore
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	if !matched {
		return false
	}

	// Check for shell reserved words
	reservedWords := []string{
		"if", "then", "else", "elif", "fi", "case", "esac", "for", "while", "until", "do", "done",
		"function", "select", "time", "coproc", "in", "return", "exit", "break", "continue",
		"alias", "unalias", "export", "readonly", "local", "declare", "typeset", "let", "eval",
		"exec", "source", "builtin", "command", "type", "which", "where", "whence",
	}

	for _, word := range reservedWords {
		if name == word {
			return false
		}
	}

	return true
}

func addLineToZshrc(zshrcPath, line string) error {
	// Read existing .zshrc content (if it exists)
	var content []byte
	var err error

	content, err = os.ReadFile(zshrcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read .zshrc: %w", err)
	}

	contentStr := string(content)

	// Check if the line already exists
	if strings.Contains(contentStr, line) {
		fmt.Printf("Line already exists in .zshrc: %s\n", line)
		return nil
	}

	// Append the line
	if len(contentStr) > 0 && !strings.HasSuffix(contentStr, "\n") {
		contentStr += "\n"
	}
	contentStr += line + "\n"

	// Write back to .zshrc
	if err := os.WriteFile(zshrcPath, []byte(contentStr), 0644); err != nil {
		return fmt.Errorf("failed to write .zshrc: %w", err)
	}

	return nil
}

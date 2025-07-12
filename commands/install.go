package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const zshFunctionContent = `scripto() {
    # Create a temporary file for command communication
    local cmd_file=$(mktemp)
    
    # Run scripto with custom descriptor, allow normal interaction
    SCRIPTO_CMD_FD="$cmd_file" command scripto "$@"
    local exit_code=$?
    
    # Check if a command was written to the file
    if [ $exit_code -eq 0 ] && [ -s "$cmd_file" ]; then
        # Script execution - read and evaluate the command
        local cmd_to_exec=$(cat "$cmd_file")
        eval "$cmd_to_exec"
        local eval_exit=$?
        rm -f "$cmd_file"
        return $eval_exit
    elif [ $exit_code -eq 3 ]; then
        # Built-in command completed - cleanup and return success
        rm -f "$cmd_file"
        return 0
    else
        # Error occurred - cleanup and return error
        rm -f "$cmd_file"
        return $exit_code
    fi
}
`

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install scripto shell integration",
	Long:  "Install shell functions to enable scripto to run commands in the current shell context",
	RunE: func(cmd *cobra.Command, args []string) error {
		return installShellIntegration()
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
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

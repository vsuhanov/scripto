package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"scripto/internal/services"
	"scripto/internal/tui"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [flags] -- <command...>",
	Short: "Add a new script",
	Long:  `Add a new script to the scripto store. Use -- to separate flags from the command.
	
You can also add a script from an existing file using the --file flag:
  scripto add --file /path/to/script.sh --name "deploy"`,
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		filePath := cmd.Flag("file").Value.String()

		// Check if both file and command arguments are provided
		// When --file is specified, it provides the script content, but other flags like --name, --global are still valid
		if filePath != "" && len(args) > 0 {
			fmt.Printf("Error: Cannot specify both --file and command arguments\n")
			os.Exit(1)
		}

		// Check if we have any command arguments or file
		if len(args) == 0 && filePath == "" {
			// No arguments - show history popup first, then launch ScriptEditor
			historyResult, err := tui.RunHistoryPopup()
			if err != nil {
				fmt.Printf("Error running history popup: %v\n", err)
				os.Exit(1)
			}
			
			if historyResult.Cancelled {
				return
			}
			
			// Launch ScriptEditor with selected command from history
			if err := launchScriptEditor(historyResult.Command, "", cmd); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		}


		var command string
		var sourceFilePath string

		if filePath != "" {
			// Read command from file
			var err error
			command, sourceFilePath, _, err = readCommandFromFile(filePath)
			if err != nil {
				fmt.Printf("Error reading file: %v\n", err)
				os.Exit(1)
			}

			// Launch ScriptEditor with pre-filled content
			if err := launchScriptEditor(command, sourceFilePath, cmd); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		} else {
			// Use command from arguments
			command = strings.Join(args, " ")
		}

		// Launch ScriptEditor with the command arguments
		if err := launchScriptEditor(command, "", cmd); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	},
}

// ParsePlaceholders extracts placeholders in the format %variable:description% from a command
func ParsePlaceholders(command string) []string {
	re := regexp.MustCompile(`%([^:%]+):[^%]*%`)
	matches := re.FindAllStringSubmatch(command, -1)

	placeholders := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			placeholders = append(placeholders, match[1])
		}
	}

	return placeholders
}

// readCommandFromFile reads a command from a file and returns the command content, absolute file path, and suggested name
func readCommandFromFile(filePath string) (string, string, string, error) {
	// Expand tilde to home directory if present
	if strings.HasPrefix(filePath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get home directory: %w", err)
		}
		filePath = filepath.Join(homeDir, filePath[2:])
	}
	
	// Convert to absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", "", "", fmt.Errorf("file does not exist: %s", absPath)
	}

	// Read file content
	file, err := os.Open(absPath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read file: %w", err)
	}

	// Trim whitespace and check if file is empty
	command := strings.TrimSpace(string(content))
	if command == "" {
		return "", "", "", fmt.Errorf("file is empty: %s", absPath)
	}

	// Extract filename without extension as suggested name
	filename := filepath.Base(absPath)
	suggestedName := strings.TrimSuffix(filename, filepath.Ext(filename))

	return command, absPath, suggestedName, nil
}


// launchScriptEditor launches the ScriptEditor for adding a new script
func launchScriptEditor(command, filePath string, cmd *cobra.Command) error {
	// Create script service
	service, err := services.NewScriptService()
	if err != nil {
		return fmt.Errorf("failed to create script service: %w", err)
	}

	// Create new script with defaults
	script := service.CreateEmptyScript()

	// Apply command-line values
	if name := cmd.Flag("name").Value.String(); name != "" {
		script.Name = name
	}
	if desc := cmd.Flag("description").Value.String(); desc != "" {
		script.Description = desc
	}
	// Set scope based on global flag
	if cmd.Flag("global").Changed {
		script.Scope = "global"
	} else {
		// Use current directory scope if not global
		script.Scope = service.GetCurrentDirectoryScope()
	}

	if filePath != "" {
		script.FilePath = filePath
	}

	// If we have a command but no file, create a temporary file for editing
	if command != "" && filePath == "" {
		// Create a temporary script file for the command
		tempFilePath, err := service.CreateTempScriptFile(command)
		if err != nil {
			return fmt.Errorf("failed to create temp script file: %w", err)
		}
		script.FilePath = tempFilePath
	}

	// Run the script editor
	result, err := tui.RunScriptEditor(script, true)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if result.Cancelled {
		return nil
	}

	// Use provided command or command from editor
	finalCommand := command
	if finalCommand == "" {
		finalCommand = result.Command
	}

	// Save the script using the service
	if err := service.SaveScript(result.Script, finalCommand, nil); err != nil {
		return fmt.Errorf("failed to save script: %w", err)
	}

	fmt.Printf("Script added successfully\n")
	return nil
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().Bool("global", false, "Store the script globally")
	addCmd.Flags().String("name", "", "Custom name for the script")
	addCmd.Flags().String("description", "", "Description for the script")
	addCmd.Flags().String("file", "", "Add script from file")
}

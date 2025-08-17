package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
		if filePath != "" && len(args) > 0 {
			fmt.Printf("Error: Cannot specify both --file and command arguments\n")
			os.Exit(1)
		}

		// Prepare flow options
		options := tui.AddFlowOptions{
			Name:        cmd.Flag("name").Value.String(),
			Description: cmd.Flag("description").Value.String(),
			IsGlobal:    cmd.Flag("global").Changed,
		}

		if filePath != "" {
			// Read command from file
			command, sourceFilePath, _, err := readCommandFromFile(filePath)
			if err != nil {
				fmt.Printf("Error reading file: %v\n", err)
				os.Exit(1)
			}
			options.Command = command
			options.FilePath = sourceFilePath
			options.SkipHistory = true
		} else if len(args) > 0 {
			// Use command from arguments
			options.Command = strings.Join(args, " ")
			options.SkipHistory = true
		} else {
			// No arguments - start with history selection
			options.SkipHistory = false
		}

		// Create and run the add flow controller
		flowController, err := tui.NewAddFlowController(options)
		if err != nil {
			fmt.Printf("Failed to create flow controller: %v\n", err)
			os.Exit(1)
		}

		result, err := flowController.Run()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		if result.ExitCode == 0 {
			fmt.Printf("Script added successfully\n")
		}

		os.Exit(result.ExitCode)
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



func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().Bool("global", false, "Store the script globally")
	addCmd.Flags().String("name", "", "Custom name for the script")
	addCmd.Flags().String("description", "", "Description for the script")
	addCmd.Flags().String("file", "", "Add script from file")
}

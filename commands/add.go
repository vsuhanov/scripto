package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"scripto/entities"
	"scripto/internal/storage"
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
		// Get file flag
		filePath := cmd.Flag("file").Value.String()

		// Check if both file and command arguments are provided
		if filePath != "" && len(args) > 0 {
			fmt.Printf("Error: Cannot specify both --file and command arguments\n")
			os.Exit(1)
		}

		// Check if we have any command arguments or file
		if len(args) == 0 && filePath == "" {
			// No arguments - launch TUI for command selection
			if err := launchAddTUI(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		configPath, err := storage.GetConfigPath()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		config, err := storage.ReadConfig(configPath)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var command string
		var sourceFilePath string

		if filePath != "" {
			// Read command from file
			var suggestedName string
			command, sourceFilePath, suggestedName, err = readCommandFromFile(filePath)
			if err != nil {
				fmt.Printf("Error reading file: %v\n", err)
				os.Exit(1)
			}

			// Get global flag
			isGlobal := cmd.Flag("global").Changed
			
			// Launch TUI with pre-filled content
			if err := launchFileEditTUI(command, sourceFilePath, suggestedName, isGlobal); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		} else {
			// Use command from arguments
			command = strings.Join(args, " ")
		}

		// Get script name from flag (optional)
		scriptName := cmd.Flag("name").Value.String()

		// Get description from flag
		description := cmd.Flag("description").Value.String()

		// Get global flag
		isGlobal := cmd.Flag("global").Changed

		// Use the shared function to store the script
		if err := StoreScript(config, configPath, scriptName, command, description, isGlobal, ""); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		if scriptName != "" {
			fmt.Printf("Added script '%s'\n", scriptName)
		} else {
			fmt.Printf("Added script: %s\n", command)
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

// StoreScript stores a script with the given parameters, checking for duplicates
func StoreScript(config storage.Config, configPath string, name, command, description string, isGlobal bool, sourceFilePath string) error {
	// Parse placeholders from command
	// placeholders := ParsePlaceholders(command)

	var filePath string
	var err error

	// If source file path is provided, use it; otherwise create a new file
	if sourceFilePath != "" {
		filePath = sourceFilePath
	} else {
		// Save script to file
		filePath, err = storage.SaveScriptToFile(name, command)
		if err != nil {
			return fmt.Errorf("failed to save script to file: %w", err)
		}
	}

	script := entities.Script{
		Name:         name,
		// Placeholders: placeholders,
		Description:  description,
		FilePath:     filePath,
	}

	// Determine scope
	key := "global"
	if !isGlobal {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		key = wd
	}

	// Check if script with same name already exists in this scope
	if name != "" {
		for _, existingScript := range config[key] {
			if existingScript.Name == name {
				return fmt.Errorf("script with name '%s' already exists in this scope", name)
			}
		}
	}

	// Add script to config
	config[key] = append(config[key], script)

	// Save configuration
	if err := storage.WriteConfig(configPath, config); err != nil {
		return fmt.Errorf("failed to save script: %w", err)
	}

	return nil
}

// launchAddTUI launches the TUI for adding a new script with command history selection
func launchAddTUI() error {
	result, err := tui.RunAddTUI()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if result.Cancelled {
		return nil
	}

	// Script was added successfully via TUI
	fmt.Printf("Script added successfully\n")
	return nil
}

// launchFileEditTUI launches the TUI for editing a script loaded from a file
func launchFileEditTUI(command, filePath, suggestedName string, isGlobal bool) error {
	result, err := tui.RunFileEditTUI(command, filePath, suggestedName, isGlobal)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if result.Cancelled {
		return nil
	}

	// Script was added successfully via TUI
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

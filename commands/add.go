package commands

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"scripto/internal/storage"
	"scripto/internal/tui"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [flags] -- <command...>",
	Short: "Add a new script",
	Long:  `Add a new script to the scripto store. Use -- to separate flags from the command.`,
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if we have any command arguments
		if len(args) == 0 {
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

		command := strings.Join(args, " ")

		// Get script name from flag (optional)
		scriptName := cmd.Flag("name").Value.String()

		// Get description from flag
		description := cmd.Flag("description").Value.String()

		// Get global flag
		isGlobal := cmd.Flag("global").Changed

		// Use the shared function to store the script
		if err := StoreScript(config, configPath, scriptName, command, description, isGlobal); err != nil {
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

// StoreScript stores a script with the given parameters, checking for duplicates
func StoreScript(config storage.Config, configPath string, name, command, description string, isGlobal bool) error {
	// Parse placeholders from command
	placeholders := ParsePlaceholders(command)

	// Save script to file
	filePath, err := storage.SaveScriptToFile(name, command)
	if err != nil {
		return fmt.Errorf("failed to save script to file: %w", err)
	}

	script := storage.Script{
		Name:         name,
		Command:      command,
		Placeholders: placeholders,
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

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().Bool("global", false, "Store the script globally")
	addCmd.Flags().String("name", "", "Custom name for the script")
	addCmd.Flags().String("description", "", "Description for the script")
}

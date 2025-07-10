package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"scripto/internal/storage"

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
			fmt.Println("Error: command is required")
			os.Exit(1)
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

		// Parse placeholders from command
		placeholders := parsePlaceholders(command)

		// Get script name from flag (optional)
		scriptName := cmd.Flag("name").Value.String()

		// Get description from flag
		description := cmd.Flag("description").Value.String()

		script := storage.Script{
			Name:         scriptName,
			Command:      command,
			Placeholders: placeholders,
			Description:  description,
		}

		key := "global"
		if !cmd.Flag("global").Changed {
			wd, err := os.Getwd()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			key = wd
		}

		config[key] = append(config[key], script)

		if err := storage.WriteConfig(configPath, config); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if script.Name != "" {
			fmt.Printf("Added script '%s'\n", script.Name)
		} else {
			fmt.Printf("Added script: %s\n", script.Command)
		}

	},
}

// parsePlaceholders extracts placeholders in the format {variable:description} from a command
func parsePlaceholders(command string) []string {
	re := regexp.MustCompile(`\{([^:}]+):[^}]*\}`)
	matches := re.FindAllStringSubmatch(command, -1)

	placeholders := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			placeholders = append(placeholders, match[1])
		}
	}

	return placeholders
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().Bool("global", false, "Store the script globally")
	addCmd.Flags().String("name", "", "Custom name for the script")
	addCmd.Flags().String("description", "", "Description for the script")
}

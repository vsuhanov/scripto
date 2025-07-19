package storage

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"scripto/entities"
)

const (
	configDir  = ".scripto"
	configFile = "scripts.json"
	scriptsDir = "scripts"
)

// Config represents the entire configuration file.
type Config map[string][]entities.Script

// GetConfigPath returns the absolute path to the configuration file.
// It checks for SCRIPTO_CONFIG environment variable first, then falls back to default.

func GetConfigPath() (string, error) {
	// Check for custom config path via environment variable
	if customPath := os.Getenv("SCRIPTO_CONFIG"); customPath != "" {
		return customPath, nil
	}

	// Default to home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, configFile), nil
}

// ReadConfig reads the configuration from the file.

func ReadConfig(path string) (Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(Config), nil
		}
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// WriteConfig writes the configuration to the file.

func WriteConfig(path string, config Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, 0644)
}

// GetShellExtension returns the file extension for the current shell
func GetShellExtension() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ".sh" // default fallback
	}

	// Extract shell name from path
	shellName := filepath.Base(shell)

	switch shellName {
	case "zsh":
		return ".zsh"
	case "bash":
		return ".sh"
	case "fish":
		return ".fish"
	default:
		return ".sh"
	}
}

// GenerateRandomPrefix creates a random alphanumeric prefix
func GenerateRandomPrefix() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 6

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to simple method if crypto/rand fails
		return "script"
	}

	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}

	return string(bytes)
}

// SanitizeForFilename sanitizes a string to be safe for use in filenames
func SanitizeForFilename(input string) string {
	// Replace spaces with underscores
	sanitized := strings.ReplaceAll(input, " ", "_")

	// Remove or replace problematic characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
	sanitized = reg.ReplaceAllString(sanitized, "")

	// Limit length
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "script"
	}

	return sanitized
}

// GenerateScriptFilename generates a unique filename for a script
func GenerateScriptFilename(name, command string) string {
	prefix := GenerateRandomPrefix()
	shellExt := GetShellExtension()

	// Use name if provided, otherwise use command
	base := name
	if base == "" {
		base = command
	}

	sanitized := SanitizeForFilename(base)
	return fmt.Sprintf("%s_%s%s", prefix, sanitized, shellExt)
}

// GetScriptsDir returns the path to the scripts directory
func GetScriptsDir() (string, error) {
	// Check for custom config path via environment variable
	if customPath := os.Getenv("SCRIPTO_CONFIG"); customPath != "" {
		// Use the directory of the custom config path
		dir := filepath.Dir(customPath)
		return filepath.Join(dir, scriptsDir), nil
	}

	// Default to home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, scriptsDir), nil
}

// SaveScriptToFile saves a script command to a file and returns the file path
func SaveScriptToFile(name, command string) (string, error) {
	scriptsDir, err := GetScriptsDir()
	if err != nil {
		return "", fmt.Errorf("failed to get scripts directory: %w", err)
	}

	// Create scripts directory if it doesn't exist
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create scripts directory: %w", err)
	}

	// Generate unique filename
	filename := GenerateScriptFilename(name, command)
	filePath := filepath.Join(scriptsDir, filename)

	// Write script content to file
	if err := ioutil.WriteFile(filePath, []byte(command), 0644); err != nil {
		return "", fmt.Errorf("failed to write script file: %w", err)
	}

	return filePath, nil
}

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

type Config map[string][]entities.Script


func GetConfigPath() (string, error) {
	if customPath := os.Getenv("SCRIPTO_CONFIG"); customPath != "" {
		return customPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, configFile), nil
}


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

func GetShellExtension() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
	}

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

func GenerateRandomPrefix() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 6

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "script"
	}

	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}

	return string(bytes)
}

func SanitizeForFilename(input string) string {
	sanitized := strings.ReplaceAll(input, " ", "_")

	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
	sanitized = reg.ReplaceAllString(sanitized, "")

	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	if sanitized == "" {
		sanitized = "script"
	}

	return sanitized
}

func GenerateScriptFilename(name, command string) string {
	prefix := GenerateRandomPrefix()
	shellExt := GetShellExtension()

	base := name
	if base == "" {
		base = command
	}

	sanitized := SanitizeForFilename(base)
	return fmt.Sprintf("%s_%s%s", prefix, sanitized, shellExt)
}

func GetScriptsDir() (string, error) {
	if customPath := os.Getenv("SCRIPTO_CONFIG"); customPath != "" {
		dir := filepath.Dir(customPath)
		return filepath.Join(dir, scriptsDir), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, scriptsDir), nil
}

func SaveScriptToFile(name, command string) (string, error) {
	scriptsDir, err := GetScriptsDir()
	if err != nil {
		return "", fmt.Errorf("failed to get scripts directory: %w", err)
	}

	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create scripts directory: %w", err)
	}

	filename := GenerateScriptFilename(name, command)
	filePath := filepath.Join(scriptsDir, filename)

	if err := ioutil.WriteFile(filePath, []byte(command), 0644); err != nil {
		return "", fmt.Errorf("failed to write script file: %w", err)
	}

	return filePath, nil
}

func GetBinDir() (string, error) {
	if customPath := os.Getenv("SCRIPTO_CONFIG"); customPath != "" {
		dir := filepath.Dir(customPath)
		return filepath.Join(dir, "bin"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, "bin"), nil
}

func CreateShortcutFunction(name string) error {
	if name == "" {
		return fmt.Errorf("script name cannot be empty")
	}

	binDir, err := GetBinDir()
	if err != nil {
		return fmt.Errorf("failed to get bin directory: %w", err)
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	sanitizedName := SanitizeForFilename(name)
	if sanitizedName != name {
		functionName := name
		filename := sanitizedName + GetShellExtension()
		filePath := filepath.Join(binDir, filename)

		functionContent := fmt.Sprintf("function %s() {\n  scripto \"%s\" \"$@\"\n}\n", functionName, name)
		return os.WriteFile(filePath, []byte(functionContent), 0644)
	}

	filename := name + GetShellExtension()
	filePath := filepath.Join(binDir, filename)

	functionContent := fmt.Sprintf("function %s() {\n  scripto \"%s\" \"$@\"\n}\n", name, name)
	return os.WriteFile(filePath, []byte(functionContent), 0644)
}

func RemoveShortcutFunction(name string) error {
	if name == "" {
	}

	binDir, err := GetBinDir()
	if err != nil {
		return fmt.Errorf("failed to get bin directory: %w", err)
	}

	sanitizedName := SanitizeForFilename(name)
	shellExt := GetShellExtension()

	sanitizedPath := filepath.Join(binDir, sanitizedName+shellExt)
	if err := os.Remove(sanitizedPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove shortcut function file: %w", err)
	}

	if sanitizedName != name {
		originalPath := filepath.Join(binDir, name+shellExt)
		if err := os.Remove(originalPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove shortcut function file: %w", err)
		}
	}

	return nil
}

func SyncShortcuts(config Config) error {
	binDir, err := GetBinDir()
	if err != nil {
		return fmt.Errorf("failed to get bin directory: %w", err)
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	existingShortcuts := make(map[string]bool)
	if entries, err := os.ReadDir(binDir); err == nil {
		shellExt := GetShellExtension()
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), shellExt) {
				name := strings.TrimSuffix(entry.Name(), shellExt)
				existingShortcuts[name] = true
			}
		}
	}

	shouldExist := make(map[string]bool)

	if globalScripts, exists := config["global"]; exists {
		for _, script := range globalScripts {
			if script.Name != "" {
				shouldExist[script.Name] = true
				
				if err := CreateShortcutFunction(script.Name); err != nil {
					return fmt.Errorf("failed to create shortcut for '%s': %w", script.Name, err)
				}
			}
		}
	}

	shellExt := GetShellExtension()
	for existingName := range existingShortcuts {
		if !shouldExist[existingName] {
			filePath := filepath.Join(binDir, existingName+shellExt)
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove obsolete shortcut '%s': %w", existingName, err)
			}
		}
	}

	return nil
}

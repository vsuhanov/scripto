package storage

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	configDir  = ".scripto"
	configFile = "scripts.json"
)

// Script represents a single command script.

type Script struct {
	Name         string   `json:"name"`
	Command      string   `json:"command"`
	Placeholders []string `json:"placeholders"`
	Description  string   `json:"description"`
}

// Config represents the entire configuration file.

type Config map[string][]Script

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

package test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	binaryName = "scripto"
	binaryPath = "../bin/scripto"
)

// Script represents a single command script for testing
type Script struct {
	Name         string   `json:"name"`
	Placeholders []string `json:"placeholders"`
	Description  string   `json:"description"`
	FilePath     string   `json:"file_path,omitempty"`
}

// Config represents the entire configuration file for testing
type Config map[string][]Script

// setupTest creates a temporary config file and returns cleanup function
func setupTest(t *testing.T) (configPath string, cleanup func()) {
	t.Helper()

	tmpDir := filepath.Join("tmp", t.Name())
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	configPath = filepath.Join(tmpDir, "scripts.json")

	cleanup = func() {
		os.RemoveAll(tmpDir)
	}

	return configPath, cleanup
}

// runScripto executes the scripto binary with given args and env
func runScripto(t *testing.T, env map[string]string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// readConfig reads and parses the config file
func readConfig(t *testing.T, configPath string) Config {
	t.Helper()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config file: %v", err)
	}

	return config
}

// TestAddBasicCommand tests adding a basic command without placeholders
func TestAddBasicCommand(t *testing.T) {
	configPath, cleanup := setupTest(t)
	defer cleanup()

	env := map[string]string{"SCRIPTO_CONFIG": configPath}

	stdout, stderr, err := runScripto(t, env, "add", "echo", "hello")
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "Added script 'echo'") {
		t.Errorf("Expected success message, got: %s", stdout)
	}

	config := readConfig(t, configPath)

	// Find the non-global key (should be the current working directory)
	var scripts []Script
	for key, value := range config {
		if key != "global" {
			scripts = value
			break
		}
	}

	if len(scripts) != 1 {
		t.Errorf("Expected 1 script, got %d. Config: %+v", len(scripts), config)
	}

	script := scripts[0]
	if script.Name != "echo" {
		t.Errorf("Expected name 'echo', got '%s'", script.Name)
	}
	// Command field removed - check FilePath exists and read content  
	if script.FilePath == "" {
		t.Error("Expected script to have a FilePath")
	} else {
		content, err := os.ReadFile(script.FilePath)
		if err != nil {
			t.Errorf("Failed to read script file: %v", err)
		} else if strings.TrimSpace(string(content)) != "echo hello" {
			t.Errorf("Expected file content 'echo hello', got '%s'", strings.TrimSpace(string(content)))
		}
	}
	if len(script.Placeholders) != 0 {
		t.Errorf("Expected no placeholders, got %v", script.Placeholders)
	}
}

// TestAddWithPlaceholders tests adding a command with placeholders
func TestAddWithPlaceholders(t *testing.T) {
	configPath, cleanup := setupTest(t)
	defer cleanup()

	env := map[string]string{"SCRIPTO_CONFIG": configPath}

	stdout, stderr, err := runScripto(t, env, "add", "--name", "deploy",
		"docker run -d --name {service:service name} -p {port:port number} myapp:latest")
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "Added script 'deploy'") {
		t.Errorf("Expected success message, got: %s", stdout)
	}

	config := readConfig(t, configPath)

	// Find the non-global key (should be the current working directory)
	var scripts []Script
	for key, value := range config {
		if key != "global" {
			scripts = value
			break
		}
	}

	script := scripts[0]
	if script.Name != "deploy" {
		t.Errorf("Expected name 'deploy', got '%s'", script.Name)
	}

	expectedPlaceholders := []string{"service", "port"}
	if len(script.Placeholders) != len(expectedPlaceholders) {
		t.Errorf("Expected %d placeholders, got %d", len(expectedPlaceholders), len(script.Placeholders))
	}

	for i, expected := range expectedPlaceholders {
		if script.Placeholders[i] != expected {
			t.Errorf("Expected placeholder '%s', got '%s'", expected, script.Placeholders[i])
		}
	}
}

// TestAddGlobalScope tests adding a script with global scope
func TestAddGlobalScope(t *testing.T) {
	configPath, cleanup := setupTest(t)
	defer cleanup()

	env := map[string]string{"SCRIPTO_CONFIG": configPath}

	stdout, stderr, err := runScripto(t, env, "add", "--global", "--name", "backup",
		"--description", "Backup database",
		"pg_dump -h {host:database host} -U {user:username} {db:database name} > backup.sql")
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "Added script 'backup'") {
		t.Errorf("Expected success message, got: %s", stdout)
	}

	config := readConfig(t, configPath)

	if len(config["global"]) != 1 {
		t.Errorf("Expected 1 global script, got %d", len(config["global"]))
	}

	script := config["global"][0]
	if script.Name != "backup" {
		t.Errorf("Expected name 'backup', got '%s'", script.Name)
	}
	if script.Description != "Backup database" {
		t.Errorf("Expected description 'Backup database', got '%s'", script.Description)
	}

	expectedPlaceholders := []string{"host", "user", "db"}
	if len(script.Placeholders) != len(expectedPlaceholders) {
		t.Errorf("Expected %d placeholders, got %d", len(expectedPlaceholders), len(script.Placeholders))
	}
}

// TestCustomConfigPath tests that SCRIPTO_CONFIG environment variable works
func TestCustomConfigPath(t *testing.T) {
	configPath, cleanup := setupTest(t)
	defer cleanup()

	env := map[string]string{"SCRIPTO_CONFIG": configPath}

	// Add a script
	_, stderr, err := runScripto(t, env, "add", "test", "command")
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr)
	}

	// Verify the config file was created at the custom path
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config file not created at custom path: %s", configPath)
	}

	config := readConfig(t, configPath)
	if len(config) == 0 {
		t.Error("Config file is empty")
	}
}

// TestAddWithCustomName tests adding a script with custom name and description
func TestAddWithCustomName(t *testing.T) {
	configPath, cleanup := setupTest(t)
	defer cleanup()

	env := map[string]string{"SCRIPTO_CONFIG": configPath}

	stdout, stderr, err := runScripto(t, env, "add", "--name", "custom-name",
		"--description", "Custom description", "ls -la")
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "Added script 'custom-name'") {
		t.Errorf("Expected success message with custom name, got: %s", stdout)
	}

	config := readConfig(t, configPath)

	// Find the non-global key (should be the current working directory)
	var scripts []Script
	for key, value := range config {
		if key != "global" {
			scripts = value
			break
		}
	}

	script := scripts[0]
	if script.Name != "custom-name" {
		t.Errorf("Expected name 'custom-name', got '%s'", script.Name)
	}
	if script.Description != "Custom description" {
		t.Errorf("Expected description 'Custom description', got '%s'", script.Description)
	}
	// Command field removed - check FilePath exists and read content
	if script.FilePath == "" {
		t.Error("Expected script to have a FilePath")
	} else {
		content, err := os.ReadFile(script.FilePath)
		if err != nil {
			t.Errorf("Failed to read script file: %v", err)
		} else if strings.TrimSpace(string(content)) != "ls -la" {
			t.Errorf("Expected file content 'ls -la', got '%s'", strings.TrimSpace(string(content)))
		}
	}
}

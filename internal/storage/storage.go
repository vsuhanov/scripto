package storage

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/utils"
)

const (
	configDir  = ".scripto"
	configFile = "scripts.json"
	scriptsDir = "scripts"

	shortcutExt              = ".zsh"
	DefaultShortcutSeparator = "-"
)

type Config map[string][]*entities.Script

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
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(Config), nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	if config == nil {
		config = make(Config)
	}

	return config, nil
}

func UpdateConfig(path string, mutate func(Config) error) (Config, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open config lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return nil, fmt.Errorf("failed to lock config: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	config, err := ReadConfig(path)
	if err != nil {
		return nil, err
	}
	if err := mutate(config); err != nil {
		return nil, err
	}
	if err := WriteConfig(path, config); err != nil {
		return nil, err
	}
	return config, nil
}

func WriteConfig(path string, config Config) error {
	for _, scripts := range config {
		for _, script := range scripts {
			if script.ID == "" {
				script.ID = uuid.New().String()
			}
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return writeFileAtomic(path, data, 0644)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func GetShellExtension() string {
	switch filepath.Base(os.Getenv("SHELL")) {
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

var filenameInvalidRe = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)

func SanitizeForFilename(input string) string {
	sanitized := strings.ReplaceAll(input, " ", "_")
	sanitized = filenameInvalidRe.ReplaceAllString(sanitized, "")

	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	if sanitized == "" {
		sanitized = "script"
	}

	return sanitized
}

func GenerateScriptFilename(name, command string) string {
	base := name
	if base == "" {
		base = command
	}
	return fmt.Sprintf("%s_%s%s", GenerateRandomPrefix(), SanitizeForFilename(base), GetShellExtension())
}

func GetScriptsDir() (string, error) {
	if customPath := os.Getenv("SCRIPTO_CONFIG"); customPath != "" {
		return filepath.Join(filepath.Dir(customPath), scriptsDir), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, scriptsDir), nil
}

func NewScriptFilePath(name, command string) (string, error) {
	dir, err := GetScriptsDir()
	if err != nil {
		return "", fmt.Errorf("failed to get scripts directory: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create scripts directory: %w", err)
	}

	return filepath.Join(dir, GenerateScriptFilename(name, command)), nil
}

func WriteScriptFile(path, command string) error {
	perm := os.FileMode(0644)
	if strings.HasPrefix(command, "#!") {
		perm = 0755
	}
	if err := os.WriteFile(path, []byte(command), perm); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}

func GetBinDir() (string, error) {
	if customPath := os.Getenv("SCRIPTO_CONFIG"); customPath != "" {
		return filepath.Join(filepath.Dir(customPath), "bin"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, "bin"), nil
}

var (
	shortcutInvalidRe   = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
	shortcutSeparatorRe = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)
)

func ShortcutSeparator() string {
	sep, ok := os.LookupEnv("SCRIPTO_SHORTCUT_SEPARATOR")
	if !ok || !shortcutSeparatorRe.MatchString(sep) {
		return DefaultShortcutSeparator
	}
	return sep
}

func ShortcutName(name string) string {
	sep := ShortcutSeparator()
	normalized := shortcutInvalidRe.ReplaceAllString(strings.TrimSpace(name), sep)
	if sep != "" {
		for strings.HasPrefix(normalized, sep) {
			normalized = strings.TrimPrefix(normalized, sep)
		}
		for strings.HasSuffix(normalized, sep) {
			normalized = strings.TrimSuffix(normalized, sep)
		}
	}
	return strings.TrimLeft(normalized, "-")
}

func shortcutFunctionContent(shortcut, name string) string {
	return fmt.Sprintf("function %s() {\n  scripto %s \"$@\"\n}\n", shortcut, utils.ShellQuote(name))
}

func CreateShortcutFunction(name string) error {
	shortcut := ShortcutName(name)
	if shortcut == "" {
		return fmt.Errorf("script name '%s' has no characters usable in a shell function name", name)
	}

	binDir, err := GetBinDir()
	if err != nil {
		return fmt.Errorf("failed to get bin directory: %w", err)
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	return os.WriteFile(filepath.Join(binDir, shortcut+shortcutExt), []byte(shortcutFunctionContent(shortcut, name)), 0644)
}

func RemoveShortcutFunction(name string) error {
	binDir, err := GetBinDir()
	if err != nil {
		return fmt.Errorf("failed to get bin directory: %w", err)
	}

	if shortcut := ShortcutName(name); shortcut != "" {
		path := filepath.Join(binDir, shortcut+shortcutExt)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove shortcut function file: %w", err)
		}
	}

	legacyPath := filepath.Join(binDir, SanitizeForFilename(name)+GetShellExtension())
	if content, err := os.ReadFile(legacyPath); err == nil && strings.Contains(string(content), fmt.Sprintf("scripto \"%s\"", name)) {
		if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove legacy shortcut function file: %w", err)
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

	shouldExist := make(map[string]bool)
	for _, script := range config["global"] {
		if script.Name == "" || script.Archived {
			continue
		}
		shortcut := ShortcutName(script.Name)
		if shortcut == "" {
			continue
		}
		shouldExist[shortcut+shortcutExt] = true
		if err := CreateShortcutFunction(script.Name); err != nil {
			return fmt.Errorf("failed to create shortcut for '%s': %w", script.Name, err)
		}
	}

	entries, err := os.ReadDir(binDir)
	if err != nil {
		return fmt.Errorf("failed to read bin directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || shouldExist[entry.Name()] || !strings.HasSuffix(entry.Name(), shortcutExt) {
			continue
		}
		if err := os.Remove(filepath.Join(binDir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove obsolete shortcut '%s': %w", entry.Name(), err)
		}
	}

	return nil
}

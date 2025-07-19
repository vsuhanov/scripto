package execution

import (
	"fmt"
	"os"
	"strings"
)

// GetCommandToExecute reads a script file and returns the appropriate command to execute
// If the file starts with a shebang, returns the file path
// Otherwise, returns the file contents with placeholders processed
func GetCommandToExecute(filePath string, placeholders map[string]string) (string, error) {
	// Read the script file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read script file %s: %w", filePath, err)
	}

	contentStr := string(content)

	// Check if file starts with shebang
	if strings.HasPrefix(contentStr, "#!") {
		// File has shebang, return the file path for direct execution
		return filePath, nil
	}

	// File doesn't have shebang, process placeholders and return content
	processedContent := processPlaceholders(contentStr, placeholders)
	return processedContent, nil
}

// processPlaceholders substitutes placeholder values in the content
func processPlaceholders(content string, placeholders map[string]string) string {
	result := content
	for name, value := range placeholders {
		pattern := fmt.Sprintf("%%%s:", name)
		if strings.Contains(result, pattern) {
			// Find and replace the placeholder
			start := strings.Index(result, pattern)
			if start != -1 {
				// Find the next % that closes the placeholder
				endSearch := result[start+len(pattern):]
				endIdx := strings.Index(endSearch, "%")
				if endIdx != -1 {
					end := start + len(pattern) + endIdx + 1
					placeholder := result[start:end]
					result = strings.Replace(result, placeholder, value, 1)
				}
			}
		}
	}
	return result
}

// WriteScriptPathToFile writes the script path to the specified file descriptor path
func WriteScriptPathToFile(scriptPath, fdPath string) error {
	return os.WriteFile(fdPath, []byte(scriptPath), 0600)
}
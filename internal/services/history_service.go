package services

import "os"
import "strings"

type HistoryService struct{}

func NewHistoryService() *HistoryService {
	return &HistoryService{}
}

func (hs *HistoryService) GetHistoryCommands() []string {
	historyFilePath := os.Getenv("SCRIPTO_SHELL_HISTORY_FILE_PATH")

	if historyFilePath == "" {
		return []string{}
	}

	content, err := os.ReadFile(historyFilePath)
	if err != nil {
		return []string{}
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	var commands []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(parts) >= 2 {
			command := parts[1]
			command = strings.ReplaceAll(command, "\\n", "\n")
			commands = append(commands, command)
		}
	}

	for i := len(commands)/2 - 1; i >= 0; i-- {
		opp := len(commands) - 1 - i
		commands[i], commands[opp] = commands[opp], commands[i]
	}

	return commands

}

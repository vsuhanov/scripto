package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func handleInstallSkill(args []string) error {
	pathArg := ""
	for i, arg := range args {
		if arg == "--path" && i+1 < len(args) {
			pathArg = args[i+1]
		}
	}

	printInstallHeader()

	target, err := resolveSkillTarget(pathArg)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}
	if err := os.WriteFile(target, []byte(skillMD), 0644); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}
	printInstallStep("Installed skill", target)

	return nil
}

func resolveSkillTarget(pathArg string) (string, error) {
	if pathArg != "" {
		if strings.HasSuffix(pathArg, ".md") {
			return pathArg, nil
		}
		return filepath.Join(pathArg, "scripto", "SKILL.md"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	claudePath := filepath.Join(homeDir, ".claude", "skills", "scripto", "SKILL.md")
	kiroPath := filepath.Join(homeDir, ".kiro", "skills", "scripto", "SKILL.md")

	printInstallNote("Select skill destination:")
	printInstallNote(fmt.Sprintf("  1) Claude Code  %s (default)", claudePath))
	printInstallNote(fmt.Sprintf("  2) Kiro CLI     %s", kiroPath))
	printInstallNote("  3) Custom path")
	fmt.Fprint(os.Stderr, "  Choice [1]: ")

	scanner := bufio.NewScanner(os.Stdin)
	choice := ""
	if scanner.Scan() {
		choice = strings.TrimSpace(scanner.Text())
	}

	switch choice {
	case "", "1":
		return claudePath, nil
	case "2":
		return kiroPath, nil
	case "3":
		fmt.Fprint(os.Stderr, "  Enter path: ")
		if scanner.Scan() {
			custom := strings.TrimSpace(scanner.Text())
			if custom == "" {
				return "", fmt.Errorf("no path provided")
			}
			if strings.HasSuffix(custom, ".md") {
				return custom, nil
			}
			return filepath.Join(custom, "scripto", "SKILL.md"), nil
		}
		return "", fmt.Errorf("no path provided")
	default:
		return "", fmt.Errorf("invalid choice: %s", choice)
	}
}

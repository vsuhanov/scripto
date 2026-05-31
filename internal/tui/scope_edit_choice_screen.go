package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/services"
	"github.com/vsuhanov/scripto/internal/tui/colors"
)

type ScopeEditChoiceScreen struct {
	script    *entities.Script
	selected  int
	width     int
	height    int
	container *services.Container
}

func NewScopeEditChoiceScreen(script *entities.Script, container *services.Container) *ScopeEditChoiceScreen {
	return &ScopeEditChoiceScreen{
		script:    script,
		selected:  0,
		width:     80,
		height:    24,
		container: container,
	}
}

func (s *ScopeEditChoiceScreen) Init() tea.Cmd {
	return nil
}

func (s *ScopeEditChoiceScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return s, func() tea.Msg { return NavigateBackMsg{} }

		case "j", "down":
			if s.selected < 1 {
				s.selected++
			}
			return s, nil

		case "k", "up":
			if s.selected > 0 {
				s.selected--
			}
			return s, nil

		case "1":
			return s, s.editOriginal()

		case "2":
			return s, s.copyToCurrent()

		case "enter":
			if s.selected == 0 {
				return s, s.editOriginal()
			}
			return s, s.copyToCurrent()
		}
	}

	return s, nil
}

func (s *ScopeEditChoiceScreen) editOriginal() tea.Cmd {
	corrected := *s.script
	corrected.Scope = s.script.OriginalScope
	corrected.OriginalScope = ""
	return func() tea.Msg {
		return ShowScriptEditorMsg{script: &corrected}
	}
}

func (s *ScopeEditChoiceScreen) copyToCurrent() tea.Cmd {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = s.script.OriginalScope
	}
	var command string
	if s.script.FilePath != "" {
		if content, err := os.ReadFile(s.script.FilePath); err == nil {
			command = strings.TrimSpace(string(content))
		}
	}
	newScript := &entities.Script{
		Name:          s.script.Name,
		Description:   s.script.Description,
		Scope:         cwd,
		OriginalScope: "",
	}
	return func() tea.Msg {
		return ShowScriptEditorMsg{script: newScript, initialCommand: command, isNewScript: true}
	}
}

func (s *ScopeEditChoiceScreen) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Primary.Light.TrueColor)).
		Bold(true).
		MarginBottom(1)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.MutedText.Dark.TrueColor))

	scopeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Warning.Light.TrueColor)).
		Bold(true)

	optionStyle := lipgloss.NewStyle().
		PaddingLeft(2)

	selectedStyle := lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color(colors.Primary.Light.TrueColor)).
		Bold(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.MutedText.Dark.TrueColor)).
		MarginTop(1)

	originalScope := s.script.OriginalScope
	scopeDisplay := originalScope
	if originalScope != "global" {
		scopeDisplay = filepath.Base(originalScope)
	}

	lines := []string{
		titleStyle.Render("Edit Script"),
		subtitleStyle.Render(fmt.Sprintf("This script is defined in %s", scopeStyle.Render(scopeDisplay))),
		"",
	}

	options := []string{
		fmt.Sprintf("1. Edit in original scope (%s)", scopeDisplay),
		"2. Copy to current scope",
	}

	for i, opt := range options {
		if i == s.selected {
			lines = append(lines, selectedStyle.Render("▶ "+opt))
		} else {
			lines = append(lines, optionStyle.Render("  "+opt))
		}
	}

	lines = append(lines, hintStyle.Render("j/k to navigate • enter/1/2 to select • esc to cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colors.Border.Dark.TrueColor)).
		Padding(1, 2)

	box := boxStyle.Render(content)

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, box)
}

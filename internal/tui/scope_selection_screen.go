package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"scripto/entities"
)

type ScopeSelectionScreen struct {
	script     *entities.Script
	scriptArgs []string
	width      int
	height     int
	selected   int
}

func (s *ScopeSelectionScreen) Init() tea.Cmd {
	return nil
}

func (s *ScopeSelectionScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
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
		case "enter":
			script := s.script
			scriptArgs := s.scriptArgs
			useScriptDir := s.selected == 0
			return s, func() tea.Msg {
				return ScopeSelectionResultMsg{script: script, scriptArgs: scriptArgs, useScriptDir: useScriptDir}
			}
		case "esc", "q":
			return s, func() tea.Msg { return NavigateBackMsg{} }
		}
	}
	return s, nil
}

func (s *ScopeSelectionScreen) View() string {
	var b strings.Builder

	b.WriteString(PopupTitleStyle.Render("Choose Execution Directory"))
	b.WriteString("\n\n")

	scriptName := s.script.Name
	if scriptName == "" {
		scriptName = "(unnamed)"
	}
	muted := lipgloss.NewStyle().Foreground(mutedTextColor)
	b.WriteString(muted.Render("Script: ") + scriptName)
	b.WriteString("\n")
	b.WriteString(muted.Render("Source: ") + s.script.Scope)
	b.WriteString("\n\n")

	opt0 := "Script's directory (" + s.script.Scope + ")"
	opt1 := "Current directory"

	if s.selected == 0 {
		b.WriteString(ListItemSelectedStyle.Render(opt0))
	} else {
		b.WriteString(ListItemStyle.Render(opt0))
	}
	b.WriteString("\n")
	if s.selected == 1 {
		b.WriteString(ListItemSelectedStyle.Render(opt1))
	} else {
		b.WriteString(ListItemStyle.Render(opt1))
	}

	b.WriteString("\n\n")
	b.WriteString(InstructionStyle.Render("j/k: Navigate • Enter: Select • Esc: Cancel"))

	popup := PopupStyle.Render(b.String())

	popupWidth := lipgloss.Width(popup)
	popupHeight := lipgloss.Height(popup)

	leftPad := (s.width - popupWidth) / 2
	topPad := (s.height - popupHeight) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	if topPad < 0 {
		topPad = 0
	}

	return lipgloss.NewStyle().PaddingLeft(leftPad).PaddingTop(topPad).Render(popup)
}

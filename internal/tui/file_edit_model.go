package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/internal/script"
	"scripto/entities"
)

// FileEditModel represents a model for editing a script loaded from a file
type FileEditModel struct {
	// UI state
	width  int
	height int
	ready  bool
	// Script data
	script   entities.Script
	isGlobal bool
	// Popup state
	editPopup *EditPopup
	// State tracking
	cancelled bool
	statusMsg string
}

// NewFileEditModel creates a new FileEditModel with pre-filled content
func NewFileEditModel(command, filePath, suggestedName string, isGlobal bool) FileEditModel {
	return FileEditModel{
		ready:    false,
		script: entities.Script{
			Name:        suggestedName,
			Description: "",
			FilePath:    filePath,
		},
		isGlobal: isGlobal,
	}
}

// Init initializes the FileEditModel
func (m FileEditModel) Init() tea.Cmd {
	return func() tea.Msg {
		return tea.WindowSizeMsg{Width: 80, Height: 24}
	}
}

// Update handles FileEditModel events
func (m FileEditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		
		// Create a script match result for the edit popup
		scope := "local"
		if m.isGlobal {
			scope = "global"
		}
		
		scriptResult := script.MatchResult{
			Script: m.script,
			Scope: scope,
		}
		
		// Create and show edit popup immediately
		popup := NewEditPopup(scriptResult, m.width, m.height)
		m.editPopup = &popup
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.editPopup != nil && m.editPopup.active {
				// Close edit popup
				m.editPopup.active = false
				m.cancelled = true
				return m, tea.Quit
			}
		}

	case StatusMsg:
		m.statusMsg = string(msg)
		// If we got a success message, we're done
		if strings.Contains(m.statusMsg, "successfully") {
			return m, tea.Quit
		}
		return m, nil

	case ErrorMsg:
		m.statusMsg = fmt.Sprintf("Error: %v", msg)
		return m, nil

	default:
		// Forward messages to edit popup
		if m.editPopup != nil && m.editPopup.active {
			updatedPopup, cmd := m.editPopup.Update(msg)
			m.editPopup = &updatedPopup
			return m, cmd
		}
	}

	return m, nil
}

// View renders the FileEditModel
func (m FileEditModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	var content string

	// Show edit popup
	if m.editPopup != nil && m.editPopup.active {
		content = m.editPopup.View()
	} else {
		content = "No active popup"
	}

	// Add status message if any
	if m.statusMsg != "" {
		status := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(m.statusMsg)
		content += "\n" + status
	}

	return content
}

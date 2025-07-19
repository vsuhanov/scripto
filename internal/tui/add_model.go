package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/internal/script"
	"scripto/entities"
)

// NavigationState represents different states in the add flow
type NavigationState int

const (
	StateHistory NavigationState = iota
	StateEdit
	StateNone
)

// AddModel represents the state for adding a new script
type AddModel struct {
	// UI state
	width  int
	height int
	ready  bool

	// Popup state
	historyPopup *HistoryPopup
	editPopup    *EditPopup

	// Navigation state
	currentState    NavigationState
	previousState   NavigationState
	selectedCommand string

	// State tracking
	cancelled bool
	statusMsg string
}

// initHistoryPopupMsg signals to initialize the history popup
type initHistoryPopupMsg struct{}

// NavigateBackMsg signals to go back to the previous state
type NavigateBackMsg struct{}

// NewAddModel creates a new AddModel
func NewAddModel() AddModel {
	return AddModel{
		ready:         false,
		currentState:  StateHistory,
		previousState: StateNone,
	}
}

// Init initializes the AddModel
func (m AddModel) Init() tea.Cmd {
	// Just return a command to get the window size first
	return tea.Batch(
		tea.EnterAltScreen,
		func() tea.Msg {
			return initHistoryPopupMsg{}
		},
	)
}

// Update handles AddModel events
func (m AddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		wasReady := m.ready
		m.ready = true

		// Update popup sizes if they exist
		if m.historyPopup != nil {
			m.historyPopup.width = msg.Width
			m.historyPopup.height = msg.Height
		}
		if m.editPopup != nil {
			m.editPopup.width = msg.Width
			m.editPopup.height = msg.Height
		}

		// If this is the first time we're ready, initialize the history popup
		if !wasReady {
			return m, func() tea.Msg { return initHistoryPopupMsg{} }
		}

		return m, nil

	case initHistoryPopupMsg:
		// Initialize history popup once we have window size
		if m.ready {
			popup := NewHistoryPopup(m.width, m.height)
			m.historyPopup = &popup

			// Load command history
			updatedPopup, cmd := popup.LoadHistory()
			m.historyPopup = &updatedPopup

			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		// Handle quit commands
		if msg.String() == "ctrl+c" {
			m.cancelled = true
			return m, tea.Quit
		}

		// Handle history popup
		if m.historyPopup != nil && m.historyPopup.active {
			updatedPopup, cmd := m.historyPopup.Update(msg)
			m.historyPopup = &updatedPopup

			// Check if popup was closed
			if !m.historyPopup.active {
				m.historyPopup = nil
				// If there's a command (like selecting a command), execute it
				if cmd != nil {
					return m, cmd
				}
			}
			return m, cmd
		}

		// Handle edit popup
		if m.editPopup != nil && m.editPopup.active {
			updatedPopup, cmd := m.editPopup.Update(msg)
			m.editPopup = &updatedPopup

			// Check if popup was closed
			if !m.editPopup.active {
				m.editPopup = nil
				// If there's a command (like save), execute it
				if cmd != nil {
					return m, cmd
				}
			}
			return m, cmd
		}

		return m, nil

	case HistorySelectedMsg:
		// History popup selected a command, now show edit popup
		command := msg.command
		m.selectedCommand = command

		// Transition to edit state
		m.previousState = m.currentState
		m.currentState = StateEdit
		m.historyPopup = nil

		// Create a new script with the selected command
		newScript := script.MatchResult{
			Script: entities.Script{
				Name:        "",
				Description: "",
        		FilePath: "",
			},
			Scope:     "",
			Directory: "",
		}

		// Create edit popup with the command pre-filled
		popup := NewEditPopup(newScript, m.width, m.height)
		m.editPopup = &popup
		return m, nil

	case NavigateBackMsg:
		// Handle back navigation
		switch m.currentState {
		case StateEdit:
			// Go back to history if we came from there
			if m.previousState == StateHistory {
				m.currentState = StateHistory
				m.previousState = StateNone
				m.editPopup = nil

				// Recreate history popup
				popup := NewHistoryPopup(m.width, m.height)
				m.historyPopup = &popup
				updatedPopup, cmd := popup.LoadHistory()
				m.historyPopup = &updatedPopup
				return m, cmd
			} else {
				// Exit if no previous state
				m.cancelled = true
				return m, tea.Quit
			}
		case StateHistory:
			// Exit from history state
			m.cancelled = true
			return m, tea.Quit
		default:
			// Exit for any other state
			m.cancelled = true
			return m, tea.Quit
		}
		return m, nil

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
		// Forward other messages to active popup
		if m.historyPopup != nil && m.historyPopup.active {
			updatedPopup, cmd := m.historyPopup.Update(msg)
			m.historyPopup = &updatedPopup
			return m, cmd
		}

		if m.editPopup != nil && m.editPopup.active {
			updatedPopup, cmd := m.editPopup.Update(msg)
			m.editPopup = &updatedPopup
			return m, cmd
		}
	}

	return m, nil
}

// View renders the AddModel
func (m AddModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	var content string

	// Show history popup if active
	if m.historyPopup != nil && m.historyPopup.active {
		content = m.historyPopup.View()
	} else if m.editPopup != nil && m.editPopup.active {
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
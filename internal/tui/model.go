package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/internal/script"
	"scripto/internal/storage"
)

// Model represents the main TUI state
type Model struct {
	// Core data
	scripts     []script.MatchResult
	selectedIdx int
	config      storage.Config
	configPath  string

	// UI state
	width  int
	height int
	ready  bool
	err    error

	// Focus state
	focusedPane string // "list" or "preview"

	// Operation state
	showHelp      bool
	editMode      bool
	externalEdit  bool
	nameEditMode  bool
	deleteMode    bool
	confirmDelete bool
	statusMsg     string
	quitting      bool

	// Edit popup
	editPopup *EditPopup

	// Viewport for preview
	viewport viewport.Model
}

// Messages for the TUI
type (
	ScriptsLoadedMsg  []script.MatchResult
	ErrorMsg          error
	StatusMsg         string
	ExitWithScriptMsg string
	ExitForEditMsg    string
)

// NewModel creates a new TUI model
func NewModel() Model {
	return Model{
		focusedPane: "list",
		ready:       false,
		viewport:    viewport.New(0, 0),
	}
}

// Init initializes the TUI model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadScripts(),
		tea.EnterAltScreen,
	)
}

// Update handles TUI events
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		// Update viewport size
		previewWidth := m.width/2 - 2
		previewHeight := m.height - 4
		m.viewport.Width = previewWidth
		m.viewport.Height = previewHeight
		return m, nil

	case ScriptsLoadedMsg:
		m.scripts = []script.MatchResult(msg)
		// Adjust selected index if it's out of bounds
		if len(m.scripts) == 0 {
			m.selectedIdx = 0
		} else if m.selectedIdx >= len(m.scripts) {
			m.selectedIdx = len(m.scripts) - 1
		}
		// Update viewport content
		m.updateViewportContent()
		return m, nil

	case ErrorMsg:
		m.err = error(msg)
		return m, nil

	case StatusMsg:
		m.statusMsg = string(msg)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

// handleKeyMsg handles keyboard input
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle edit popup events first
	if m.editPopup != nil && m.editPopup.active {
		updatedPopup, cmd := m.editPopup.Update(msg)
		m.editPopup = &updatedPopup

		// Check if popup was closed
		if !m.editPopup.active {
			m.editPopup = nil
			// If there's a command (like save), execute it and reload scripts
			if cmd != nil {
				return m, tea.Sequence(cmd, loadScripts())
			}
		}

		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "enter":
		return m.handleEnter()

	case "e":
		return m.handleInlineEdit()

	case "E":
		return m.handleExternalEdit()

	case "n":
		return m.handleNameEdit()

	case "s":
		return m.handleScopeToggle()

	case "d":
		return m.handleDelete(true) // with confirmation

	case "D":
		return m.handleDelete(false) // without confirmation

	case "y":
		if m.deleteMode && m.confirmDelete {
			return m.confirmDeleteAction()
		}

	case "N":
		if m.deleteMode && m.confirmDelete {
			m.deleteMode = false
			m.confirmDelete = false
			m.statusMsg = "Delete cancelled"
			return m, nil
		}

	case "j", "down":
		if m.focusedPane == "preview" {
			m.viewport.LineDown(1)
			return m, nil
		}
		return m.handleDown(), nil

	case "k", "up":
		if m.focusedPane == "preview" {
			m.viewport.LineUp(1)
			return m, nil
		}
		return m.handleUp(), nil

	case "tab":
		return m.handleTabSwitch(), nil
	}

	return m, nil
}

// Navigation helpers
func (m Model) handleUp() Model {
	if len(m.scripts) > 0 {
		m.selectedIdx = (m.selectedIdx - 1 + len(m.scripts)) % len(m.scripts)
		m.updateViewportContent()
	}
	return m
}

func (m Model) handleDown() Model {
	if len(m.scripts) > 0 {
		m.selectedIdx = (m.selectedIdx + 1) % len(m.scripts)
		m.updateViewportContent()
	}
	return m
}

func (m Model) handleTabSwitch() Model {
	if m.focusedPane == "list" {
		m.focusedPane = "preview"
	} else {
		m.focusedPane = "list"
	}
	return m
}

// Action handlers
func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]

	// Return the script path for execution
	scriptPath := selected.Script.FilePath
	if scriptPath == "" {
		// Fallback to command if no file path
		scriptPath = selected.Script.Command
	}

	return m, tea.Sequence(
		func() tea.Msg { return ExitWithScriptMsg(scriptPath) },
		tea.Quit,
	)
}

func (m Model) handleInlineEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]

	// Create edit popup
	popup := NewEditPopup(selected, m.width, m.height)
	m.editPopup = &popup

	return m, nil
}

func (m Model) handleExternalEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]
	scriptPath := selected.Script.FilePath

	if scriptPath == "" {
		m.statusMsg = "Cannot edit: script has no file path"
		return m, nil
	}

	// Set external edit mode and quit
	m.editMode = true
	m.externalEdit = true
	return m, tea.Quit
}

func (m Model) handleNameEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	// TODO: Implement name editing
	m.statusMsg = "Name editing not implemented yet"
	return m, nil
}

func (m Model) handleScopeToggle() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	// TODO: Implement scope toggling
	m.statusMsg = "Scope toggling not implemented yet"
	return m, nil
}

func (m Model) handleDelete(withConfirmation bool) (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	if withConfirmation {
		m.deleteMode = true
		m.confirmDelete = true
		selected := m.scripts[m.selectedIdx]
		scriptName := selected.Script.Name
		if scriptName == "" {
			scriptName = "unnamed script"
		}
		m.statusMsg = fmt.Sprintf("Delete '%s'? (y/N)", scriptName)
		return m, nil
	} else {
		// Direct delete without confirmation
		return m.performDelete()
	}
}

func (m Model) confirmDeleteAction() (tea.Model, tea.Cmd) {
	m.deleteMode = false
	m.confirmDelete = false
	return m.performDelete()
}

func (m Model) performDelete() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]

	// Load current config
	configPath, err := storage.GetConfigPath()
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error getting config path: %v", err)
		return m, nil
	}

	config, err := storage.ReadConfig(configPath)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error reading config: %v", err)
		return m, nil
	}

	// Find and remove the script from config
	key := selected.Directory
	if key == "global" {
		key = "global"
	}

	if scripts, exists := config[key]; exists {
		// Find the script in the array
		for i, script := range scripts {
			if script.Name == selected.Script.Name &&
				script.Command == selected.Script.Command &&
				script.FilePath == selected.Script.FilePath {
				// Remove the script from the array
				config[key] = append(scripts[:i], scripts[i+1:]...)

				// If the directory has no more scripts, remove the key
				if len(config[key]) == 0 {
					delete(config, key)
				}
				break
			}
		}
	}

	// Save the updated config
	if err := storage.WriteConfig(configPath, config); err != nil {
		m.statusMsg = fmt.Sprintf("Error saving config: %v", err)
		return m, nil
	}

	// Remove script file if it exists
	if selected.Script.FilePath != "" {
		if err := os.Remove(selected.Script.FilePath); err != nil {
			// Don't fail if file doesn't exist, just warn
			m.statusMsg = fmt.Sprintf("Script deleted from config, but file removal failed: %v", err)
		} else {
			m.statusMsg = "Script deleted successfully"
		}
	} else {
		m.statusMsg = "Script deleted successfully"
	}

	// Reload scripts
	return m, loadScripts()
}

// View renders the TUI
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.err != nil {
		return ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.showHelp {
		return m.renderHelp()
	}

	// Render edit popup if active
	if m.editPopup != nil && m.editPopup.active {
		return m.renderWithPopup()
	}

	// Calculate dimensions for two-pane layout
	listWidth := m.width / 2
	previewWidth := m.width - listWidth - 2 // Account for borders

	listView := m.renderList(listWidth, m.height-4) // Account for status bar
	previewView := m.renderPreviewWithViewport(previewWidth, m.height-4)

	// Combine panes horizontally
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		listView,
		previewView,
	)

	// Add status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		statusBar,
	)
}

// renderStatusBar renders the bottom status bar
func (m Model) renderStatusBar() string {
	var parts []string

	if m.deleteMode && m.confirmDelete {
		// Show confirmation prompt
		return StatusStyle.Width(m.width).Render(m.statusMsg)
	}

	// Add key hints
	parts = append(parts, "q: quit")
	parts = append(parts, "?: help")
	parts = append(parts, "enter: execute")
	parts = append(parts, "e: edit inline")
	parts = append(parts, "E: edit external")
	parts = append(parts, "d: delete")
	parts = append(parts, "D: delete (no confirm)")

	status := strings.Join(parts, " • ")

	// Add status message if present
	if m.statusMsg != "" {
		status = fmt.Sprintf("%s | %s", status, m.statusMsg)
	}

	return StatusStyle.Width(m.width).Render(status)
}

// renderWithPopup renders the main view with edit popup overlay
func (m Model) renderWithPopup() string {
	// Render popup
	popup := m.editPopup.View()

	// Overlay popup centered on screen
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		popup,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#40444b")),
	)
}

// renderHelp renders the help screen
func (m Model) renderHelp() string {
	help := `
Scripto TUI Help

Navigation:
  j, ↓     Move down
  k, ↑     Move up
  tab      Switch between panes
  
Actions:  
  enter    Execute selected script
  e        Edit script inline (popup)
  E        Edit script in external editor
  d        Delete script (with confirmation)
  D        Delete script (no confirmation)
  n        Add/edit script name
  s        Toggle script scope
  
Other:
  ?        Toggle this help
  q        Quit
  ctrl+c   Quit

Press ? again to return to the script list.
`

	return ContainerStyle.
		Width(m.width).
		Height(m.height).
		Render(help)
}

// loadScripts loads all available scripts
func loadScripts() tea.Cmd {
	return func() tea.Msg {
		// Load configuration
		configPath, err := storage.GetConfigPath()
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to get config path: %w", err))
		}

		config, err := storage.ReadConfig(configPath)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to read config: %w", err))
		}

		// Create matcher and find all scripts
		matcher := script.NewMatcher(config)
		scripts, err := matcher.FindAllScripts()
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to find scripts: %w", err))
		}

		return ScriptsLoadedMsg(scripts)
	}
}

// updateViewportContent updates the viewport content with current script info
func (m *Model) updateViewportContent() {
	if len(m.scripts) == 0 {
		m.viewport.SetContent("No script selected")
		return
	}

	selected := m.scripts[m.selectedIdx]
	content := m.formatViewportContent(selected)
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
}

// formatViewportContent formats content for the viewport (without Command: section)
func (m Model) formatViewportContent(selected script.MatchResult) string {
	var sections []string

	// Script title
	title := m.formatPreviewTitle(selected)
	sections = append(sections, title)

	// Script metadata
	metadata := m.formatPreviewMetadata(selected)
	sections = append(sections, metadata)

	// Placeholders
	if len(selected.Script.Placeholders) > 0 {
		placeholdersSection := m.formatPreviewPlaceholders(selected.Script.Placeholders)
		sections = append(sections, placeholdersSection)
	}

	// Description
	if selected.Script.Description != "" {
		descSection := m.formatPreviewDescription(selected.Script.Description, m.viewport.Width)
		sections = append(sections, descSection)
	}

	// File content (if available)
	if selected.Script.FilePath != "" {
		fileSection := m.formatPreviewFileContent(selected.Script.FilePath, m.viewport.Width)
		if fileSection != "" {
			sections = append(sections, fileSection)
		}
	}

	return strings.Join(sections, "\n\n")
}

// renderPreviewWithViewport renders the preview pane using viewport
func (m Model) renderPreviewWithViewport(width, height int) string {
	// Update viewport size
	m.viewport.Width = width
	m.viewport.Height = height

	style := PreviewStyle.Width(width).Height(height)

	// Highlight focused pane
	if m.focusedPane == "preview" {
		style = style.BorderForeground(primaryColor)
	}

	return style.Render(m.viewport.View())
}

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
			Script: storage.Script{
				Name:        "",
				Command:     command,
				Description: "",
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

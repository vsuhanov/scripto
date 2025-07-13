package tui

import (
	"fmt"
	"os"
	"strings"

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
	nameEditMode  bool
	deleteMode    bool
	confirmDelete bool
	statusMsg     string
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
		return m, nil

	case ScriptsLoadedMsg:
		m.scripts = []script.MatchResult(msg)
		// Adjust selected index if it's out of bounds
		if len(m.scripts) == 0 {
			m.selectedIdx = 0
		} else if m.selectedIdx >= len(m.scripts) {
			m.selectedIdx = len(m.scripts) - 1
		}
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
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "enter":
		return m.handleEnter()

	case "e":
		return m.handleEdit()

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
		return m.handleDown(), nil

	case "k", "up":
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
	}
	return m
}

func (m Model) handleDown() Model {
	if len(m.scripts) > 0 {
		m.selectedIdx = (m.selectedIdx + 1) % len(m.scripts)
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

func (m Model) handleEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]
	scriptPath := selected.Script.FilePath

	if scriptPath == "" {
		m.statusMsg = "Cannot edit: script has no file path"
		return m, nil
	}

	// Set edit mode and quit
	m.editMode = true
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

	// Calculate dimensions for two-pane layout
	listWidth := m.width / 2
	previewWidth := m.width - listWidth - 2 // Account for borders

	listView := m.renderList(listWidth, m.height-4) // Account for status bar
	previewView := m.renderPreview(previewWidth, m.height-4)

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
	parts = append(parts, "e: edit")
	parts = append(parts, "d: delete")
	parts = append(parts, "D: delete (no confirm)")

	status := strings.Join(parts, " • ")

	// Add status message if present
	if m.statusMsg != "" {
		status = fmt.Sprintf("%s | %s", status, m.statusMsg)
	}

	return StatusStyle.Width(m.width).Render(status)
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
  e        Edit script in external editor
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

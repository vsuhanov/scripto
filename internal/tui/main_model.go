package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/internal/script"
	"scripto/internal/services"
	"scripto/internal/storage"
)

// MainModel represents the main TUI state
type MainModel struct {
	// Core data
	scripts       []script.MatchResult
	selectedIdx   int
	config        storage.Config
	configPath    string
	scriptService *services.ScriptService

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
func NewModel() MainModel {
	scriptService, _ := services.NewScriptService()
	return MainModel{
		focusedPane:   "list",
		ready:         false,
		viewport:      viewport.New(0, 0),
		scriptService: scriptService,
	}
}

// Init initializes the TUI model
func (m MainModel) Init() tea.Cmd {
	return tea.Batch(
		loadScripts(),
		tea.EnterAltScreen,
	)
}

// Update handles TUI events
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (m MainModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {

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
func (m MainModel) handleUp() MainModel {
	if len(m.scripts) > 0 {
		m.selectedIdx = (m.selectedIdx - 1 + len(m.scripts)) % len(m.scripts)
		m.updateViewportContent()
	}
	return m
}

func (m MainModel) handleDown() MainModel {
	if len(m.scripts) > 0 {
		m.selectedIdx = (m.selectedIdx + 1) % len(m.scripts)
		m.updateViewportContent()
	}
	return m
}

func (m MainModel) handleTabSwitch() MainModel {
	if m.focusedPane == "list" {
		m.focusedPane = "preview"
	} else {
		m.focusedPane = "list"
	}
	return m
}

func (m MainModel) handleEnter() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]
	scriptPath := selected.Script.FilePath

	return m, tea.Sequence(
		func() tea.Msg { return ExitWithScriptMsg(scriptPath) },
		tea.Quit,
	)
}

func (m MainModel) handleInlineEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]

	// Run the script editor
	return m, func() tea.Msg {
		result, err := RunScriptEditor(selected.Script, false)
		if err != nil {
			return ErrorMsg(err)
		}

		if !result.Cancelled {
			// Save the updated script
			if m.scriptService != nil {
				err := m.scriptService.SaveScript(result.Script, result.Command, &selected.Script)
				if err != nil {
					return ErrorMsg(err)
				}
			}
			return StatusMsg("Script updated successfully")
		}

		return StatusMsg("Edit cancelled")
	}
}

func (m MainModel) handleExternalEdit() (tea.Model, tea.Cmd) {
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

func (m MainModel) handleNameEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	// TODO: Implement name editing
	m.statusMsg = "Name editing not implemented yet"
	return m, nil
}

func (m MainModel) handleScopeToggle() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	// TODO: Implement scope toggling
	m.statusMsg = "Scope toggling not implemented yet"
	return m, nil
}

func (m MainModel) handleDelete(withConfirmation bool) (tea.Model, tea.Cmd) {
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

func (m MainModel) confirmDeleteAction() (tea.Model, tea.Cmd) {
	m.deleteMode = false
	m.confirmDelete = false
	return m.performDelete()
}

func (m MainModel) performDelete() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]

	// Use script service to delete
	if m.scriptService != nil {
		if err := m.scriptService.DeleteScript(selected.Script); err != nil {
			m.statusMsg = fmt.Sprintf("Error deleting script: %v", err)
			return m, nil
		}
	}

	m.statusMsg = "Script deleted successfully"
	
	// Reload scripts
	return m, loadScripts()
}

// View renders the TUI
func (m MainModel) View() string {
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
func (m MainModel) renderStatusBar() string {
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


// renderHelp renders the help screen
func (m MainModel) renderHelp() string {
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

// getScopeDisplayName returns a user-friendly display name for a scope
func (m MainModel) getScopeDisplayName(scope string) string {
	if m.scriptService != nil {
		return m.scriptService.GetScopeDisplayName(scope)
	}
	return scope
}

// updateViewportContent updates the viewport content with current script info
func (m *MainModel) updateViewportContent() {
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
func (m MainModel) formatViewportContent(selected script.MatchResult) string {
	var sections []string

	// Script title
	title := m.formatPreviewTitle(selected)
	sections = append(sections, title)

	// Script metadata
	metadata := m.formatPreviewMetadata(selected)
	sections = append(sections, metadata)

	// // Placeholders
	// if len(selected.Script.Placeholders) > 0 {
	// 	placeholdersSection := m.formatPreviewPlaceholders(selected.Script.Placeholders)
	// 	sections = append(sections, placeholdersSection)
	// }

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
func (m MainModel) renderPreviewWithViewport(width, height int) string {
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


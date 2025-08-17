package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/internal/script"
	"scripto/internal/services"
	"scripto/internal/storage"
)

// MainListScreen represents the main script list screen
type MainListScreen struct {
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

	// Screen interface state
	result     ScreenResult
	isComplete bool
}

// NewMainListScreen creates a new main list screen
func NewMainListScreen() (*MainListScreen, error) {
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	return &MainListScreen{
		configPath:  configPath,
		focusedPane: "list",
		viewport:    viewport.New(50, 10),
	}, nil
}

// SetServices implements Screen interface
func (m *MainListScreen) SetServices(svcs interface{}) {
	if scriptService, ok := svcs.(*services.ScriptService); ok {
		m.scriptService = scriptService
	}
}

// GetResult implements Screen interface
func (m *MainListScreen) GetResult() ScreenResult {
	return m.result
}

// IsComplete implements Screen interface
func (m *MainListScreen) IsComplete() bool {
	return m.isComplete
}

// SetStatusMessage sets the status message
func (m *MainListScreen) SetStatusMessage(msg string) {
	m.statusMsg = msg
}

// RefreshScripts refreshes the script list
func (m *MainListScreen) RefreshScripts() {
	// This will trigger a reload on next update
	m.ready = false
}

// Init initializes the main list screen
func (m *MainListScreen) Init() tea.Cmd {
	return tea.Batch(
		loadScripts(),
		tea.EnterAltScreen,
	)
}

// Update handles events for the main list screen
func (m *MainListScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update viewport
		headerHeight := 3
		footerHeight := 3
		availableHeight := msg.Height - headerHeight - footerHeight

		listWidth := min(50, msg.Width/2)
		previewWidth := msg.Width - listWidth - 4

		m.viewport.Width = previewWidth
		m.viewport.Height = availableHeight
		return m, nil

	case ScriptsLoadedMsg:
		m.scripts = []script.MatchResult(msg)
		m.ready = true
		if len(m.scripts) > 0 && m.selectedIdx >= len(m.scripts) {
			m.selectedIdx = 0
		}
		return m, m.updatePreview()

	case ErrorMsg:
		m.err = error(msg)
		m.ready = true
		return m, nil

	case StatusMsg:
		m.statusMsg = string(msg)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// handleKeyPress handles keyboard input
func (m *MainListScreen) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmDelete {
		return m.handleDeleteConfirmation(msg)
	}

	if m.showHelp {
		if msg.String() == "?" || msg.String() == "esc" {
			m.showHelp = false
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.result = ScreenResult{
			Action:     ActionExitApp,
			ShouldExit: true,
			ExitCode:   3,
		}
		m.isComplete = true
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "tab":
		if m.focusedPane == "list" {
			m.focusedPane = "preview"
		} else {
			m.focusedPane = "list"
		}
		return m, nil

	case "enter":
		if len(m.scripts) > 0 {
			selected := m.scripts[m.selectedIdx]
			m.result = ScreenResult{
				Action:     ActionExecuteScript,
				Data:       NewActionDataWithPath(selected.Script.FilePath),
				ShouldExit: true,
				ExitCode:   0,
			}
			m.isComplete = true
			return m, tea.Quit
		}

	case "e":
		return m.handleInlineEdit()

	case "E":
		return m.handleExternalEdit()

	case "d":
		return m.handleDeleteRequest()

	case "D":
		return m.handleImmediateDelete()

	// Navigation keys
	case "j", "down":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedIdx = min(m.selectedIdx+1, len(m.scripts)-1)
			return m, m.updatePreview()
		}

	case "k", "up":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedIdx = max(0, m.selectedIdx-1)
			return m, m.updatePreview()
		}

	case "g":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedIdx = 0
			return m, m.updatePreview()
		}

	case "G":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedIdx = len(m.scripts) - 1
			return m, m.updatePreview()
		}
	}

	// Handle viewport navigation when preview is focused
	if m.focusedPane == "preview" {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleInlineEdit handles inline editing request
func (m *MainListScreen) handleInlineEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]
	m.result = ScreenResult{
		Action: ActionEditScriptInline,
		Data:   NewActionDataWithScript(selected.Script),
	}
	m.isComplete = true
	return m, tea.Quit
}

// handleExternalEdit handles external editing request
func (m *MainListScreen) handleExternalEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedIdx]
	scriptPath := selected.Script.FilePath

	if scriptPath == "" {
		m.statusMsg = "Cannot edit: script has no file path"
		return m, nil
	}

	m.result = ScreenResult{
		Action:     ActionEditScriptExternal,
		Data:       NewActionDataWithPath(scriptPath),
		ShouldExit: true,
		ExitCode:   4,
	}
	m.isComplete = true
	return m, tea.Quit
}

// handleDeleteRequest handles delete confirmation request
func (m *MainListScreen) handleDeleteRequest() (tea.Model, tea.Cmd) {
	if len(m.scripts) > 0 {
		m.confirmDelete = true
		m.statusMsg = "Delete script? (y/n)"
	}
	return m, nil
}

// handleImmediateDelete handles immediate delete without confirmation
func (m *MainListScreen) handleImmediateDelete() (tea.Model, tea.Cmd) {
	if len(m.scripts) > 0 {
		selected := m.scripts[m.selectedIdx]
		m.result = ScreenResult{
			Action: ActionDeleteScript,
			Data:   NewActionDataWithScript(selected.Script),
		}
		// Don't mark as complete, let flow controller handle it
		return m, nil
	}
	return m, nil
}

// handleDeleteConfirmation handles delete confirmation dialog
func (m *MainListScreen) handleDeleteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirmDelete = false
		if len(m.scripts) > 0 {
			selected := m.scripts[m.selectedIdx]
			m.result = ScreenResult{
				Action: ActionDeleteScript,
				Data:   NewActionDataWithScript(selected.Script),
			}
		}
		return m, nil

	case "n", "N", "esc":
		m.confirmDelete = false
		m.statusMsg = "Delete cancelled"
		return m, nil
	}
	return m, nil
}

// updatePreview updates the preview content
func (m *MainListScreen) updatePreview() tea.Cmd {
	if len(m.scripts) == 0 || m.selectedIdx >= len(m.scripts) {
		m.viewport.SetContent("")
		return nil
	}

	selected := m.scripts[m.selectedIdx]
	content := m.formatPreviewContent(selected)
	m.viewport.SetContent(content)
	return nil
}

// formatPreviewContent formats the preview content for a script using rich formatting
func (m *MainListScreen) formatPreviewContent(script script.MatchResult) string {
	var sections []string

	// Title with scope indicator
	title := m.formatPreviewTitle(script)
	sections = append(sections, title)

	// Metadata section
	metadata := m.formatPreviewMetadata(script)
	sections = append(sections, metadata)

	// Description section
	if script.Script.Description != "" {
		maxWidth := m.viewport.Width - 4 // Account for padding
		description := m.formatPreviewDescription(script.Script.Description, maxWidth)
		sections = append(sections, description)
	}

	// File content section
	if script.Script.FilePath != "" {
		maxWidth := m.viewport.Width - 4 // Account for padding  
		fileContent := m.formatPreviewFileContent(script.Script.FilePath, maxWidth)
		sections = append(sections, fileContent)
	}

	return strings.Join(sections, "\n\n")
}

// formatPreviewTitle formats the preview title with scope indicator
func (m *MainListScreen) formatPreviewTitle(selected script.MatchResult) string {
	scopeIndicator := FormatScopeIndicator(selected.Script.Scope)

	var title string
	if selected.Script.Name != "" {
		title = selected.Script.Name
	} else {
		title = "Unnamed Script"
	}

	return PreviewTitleStyle.Render(fmt.Sprintf("%s %s", scopeIndicator, title))
}

// formatPreviewMetadata formats script metadata
func (m *MainListScreen) formatPreviewMetadata(selected script.MatchResult) string {
	var metadata []string

	// Scope display
	if selected.Script.Scope == "global" {
		metadata = append(metadata, "Scope: global")
	} else {
		// Show both scope label and directory path
		scopeLabel := m.getScopeDisplayName(selected.Script.Scope)
		metadata = append(metadata, fmt.Sprintf("Scope: %s", scopeLabel))
		
		// Show directory path if it's long
		dir := selected.Script.Scope
		if len(dir) > 50 {
			dir = "..." + dir[len(dir)-47:]
		}
		metadata = append(metadata, fmt.Sprintf("Directory: %s", dir))
	}

	// File path
	if selected.Script.FilePath != "" {
		filename := filepath.Base(selected.Script.FilePath)
		metadata = append(metadata, fmt.Sprintf("File: %s", filename))
	}

	return PreviewContentStyle.Render(strings.Join(metadata, "\n"))
}

// formatPreviewDescription formats the script description
func (m *MainListScreen) formatPreviewDescription(description string, maxWidth int) string {
	title := PreviewTitleStyle.Render("Description:")
	wrappedDesc := m.wrapText(description, maxWidth)
	content := PreviewContentStyle.Render(wrappedDesc)
	return title + "\n" + content
}

// formatPreviewFileContent formats the script file content preview
func (m *MainListScreen) formatPreviewFileContent(filePath string, maxWidth int) string {
	content, err := readScriptFile(filePath)
	if err != nil {
		return PreviewContentStyle.Render(fmt.Sprintf("Error reading file: %v", err))
	}

	title := PreviewTitleStyle.Render("File Content:")

	// Limit preview to first 10 lines
	lines := strings.Split(content, "\n")
	if len(lines) > 10 {
		lines = lines[:10]
		lines = append(lines, "...")
	}

	// Wrap long lines
	var wrappedLines []string
	for _, line := range lines {
		if len(line) > maxWidth {
			wrapped := strings.Split(m.wrapText(line, maxWidth), "\n")
			wrappedLines = append(wrappedLines, wrapped...)
		} else {
			wrappedLines = append(wrappedLines, line)
		}
	}

	fileContent := strings.Join(wrappedLines, "\n")
	styledContent := PreviewCommandStyle.Render(fileContent)

	return title + "\n" + styledContent
}

// getScopeDisplayName returns a user-friendly display name for a scope
func (m *MainListScreen) getScopeDisplayName(scope string) string {
	if m.scriptService != nil {
		return m.scriptService.GetScopeDisplayName(scope)
	}
	return scope
}

// wrapText wraps text to the specified width
func (m *MainListScreen) wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine string

	for _, word := range words {
		// If adding this word would exceed the width, start a new line
		if len(currentLine)+len(word)+1 > width && currentLine != "" {
			lines = append(lines, currentLine)
			currentLine = word
		} else if currentLine == "" {
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}

	// Add the last line
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n")
}

// View renders the main list screen
func (m *MainListScreen) View() string {
	if !m.ready {
		return LoadingStyle.Render("Loading scripts...")
	}

	if m.err != nil {
		return ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.showHelp {
		return m.renderHelp()
	}

	return m.renderMainView()
}

// renderMainView renders the main two-pane view
func (m *MainListScreen) renderMainView() string {
	// Calculate dimensions
	headerHeight := 3
	footerHeight := 3
	availableHeight := m.height - headerHeight - footerHeight

	listWidth := min(50, m.width/2)
	previewWidth := m.width - listWidth - 4

	// Render components
	header := m.renderHeader()
	listView := m.renderList(listWidth, availableHeight)
	previewView := m.renderPreview(previewWidth, availableHeight)
	footer := m.renderFooter()

	// Combine views
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		listView,
		" ",
		previewView,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		mainContent,
		footer,
	)
}

// renderHeader renders the header
func (m *MainListScreen) renderHeader() string {
	title := TitleStyle.Render("Scripto - Script Manager")
	help := HelpStyle.Render("? for help • q to quit")
	
	return HeaderStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Center,
			title,
			strings.Repeat(" ", max(0, m.width-lipgloss.Width(title)-lipgloss.Width(help))),
			help,
		),
	)
}

// renderList renders the script list with scope grouping
func (m *MainListScreen) renderList(width, height int) string {
	if len(m.scripts) == 0 {
		emptyMsg := "No scripts found.\nUse 'scripto add' to create some scripts."
		return ListStyle.
			Width(width).
			Height(height).
			Render(emptyMsg)
	}

	var items []string
	var currentScope string

	for i, script := range m.scripts {
		// Add scope header if scope changes
		if script.Script.Scope != currentScope {
			if currentScope != "" {
				items = append(items, "") // Add spacing between scopes
			}

			scopeHeader := m.formatScopeHeader(script.Script.Scope)
			items = append(items, scopeHeader)
			currentScope = script.Script.Scope
		}

		// Format script item
		item := m.formatScriptItem(script, i)
		items = append(items, item)
	}

	// Join all items
	content := strings.Join(items, "\n")

	// Calculate available height for scrolling
	visibleHeight := height - 2 // Account for borders

	// Simple scrolling: show a window around the selected item
	lines := strings.Split(content, "\n")
	if len(lines) > visibleHeight {
		start, end := m.calculateScrollWindow(lines, visibleHeight)
		content = strings.Join(lines[start:end], "\n")
	}

	// Apply list styling
	style := ListStyle.Width(width).Height(height)

	// Highlight focused pane
	if m.focusedPane == "list" {
		style = ListFocusedStyle.Width(width).Height(height)
	}

	return style.Render(content)
}

// renderPreview renders the preview pane
func (m *MainListScreen) renderPreview(width, height int) string {
	previewStyle := PreviewStyle.Width(width).Height(height)
	if m.focusedPane == "preview" {
		previewStyle = PreviewFocusedStyle.Width(width).Height(height)
	}

	return previewStyle.Render(m.viewport.View())
}

// renderFooter renders the footer with status and key hints
func (m *MainListScreen) renderFooter() string {
	var statusText string
	if m.confirmDelete {
		statusText = "Delete script? (y/n)"
	} else if m.statusMsg != "" {
		statusText = m.statusMsg
	} else {
		statusText = "Ready"
	}

	status := StatusStyle.Render(statusText)
	
	var keyHints string
	if m.confirmDelete {
		keyHints = HelpStyle.Render("y/n: confirm/cancel")
	} else {
		keyHints = HelpStyle.Render("↵: execute • e: edit • E: external • d: delete • tab: switch pane")
	}

	return FooterStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Center,
			status,
			strings.Repeat(" ", max(0, m.width-lipgloss.Width(status)-lipgloss.Width(keyHints))),
			keyHints,
		),
	)
}

// renderHelp renders the help screen
func (m *MainListScreen) renderHelp() string {
	helpText := `Scripto - Script Manager

Navigation:
  j, ↓         Move down in list
  k, ↑         Move up in list  
  g            Go to first script
  G            Go to last script
  tab          Switch between list and preview
  
Actions:
  ↵ (enter)    Execute selected script
  e            Edit script inline
  E            Edit script in external editor
  d            Delete script (with confirmation)
  D            Delete script immediately
  
Other:
  ?            Toggle this help
  q, Ctrl+C    Quit

Press ? or Esc to close this help.`

	return HelpScreenStyle.Width(m.width).Height(m.height).Render(helpText)
}

// formatScriptItem formats a single script item for display
func (m *MainListScreen) formatScriptItem(script script.MatchResult, index int) string {
	var parts []string

	// Add scope indicator
	scopeIndicator := FormatScopeIndicator(script.Script.Scope)
	parts = append(parts, scopeIndicator)

	// Add script name or file path
	var displayName string
	if script.Script.Name != "" {
		displayName = script.Script.Name
	} else {
		// Show truncated file path for unnamed scripts
		displayName = m.truncateString(script.Script.FilePath, 60)
	}

	parts = append(parts, displayName)

	item := strings.Join(parts, " ")

	// Apply selection styling
	if index == m.selectedIdx {
		return ListItemSelectedStyle.Render(item)
	}

	return ListItemStyle.Render(item)
}

// formatScopeHeader formats a scope section header with directory name
func (m *MainListScreen) formatScopeHeader(scope string) string {
	var header string
	scopeType := getScopeType(scope)
	style := GetScopeStyle(scopeType)

	switch scopeType {
	case "local":
		header = "● " + m.formatDirectoryName(scope)
	case "parent":
		header = "◐ " + m.formatDirectoryName(scope)
	case "global":
		header = "○ Global Scripts"
	default:
		header = m.formatDirectoryName(scope)
	}

	return style.Bold(true).Render(header)
}

// formatDirectoryName formats a directory path for display with full paths
func (m *MainListScreen) formatDirectoryName(dir string) string {
	if dir == "global" {
		return "Global Scripts"
	}
	
	// Use the full absolute path
	fullPath := dir
	
	// Truncate from the left if longer than 100 characters
	if len(fullPath) > 100 {
		return "..." + fullPath[len(fullPath)-97:] // 97 + 3 ("...") = 100
	}
	
	return fullPath
}

// calculateScrollWindow calculates which lines to show for scrolling
func (m *MainListScreen) calculateScrollWindow(lines []string, visibleHeight int) (int, int) {
	// Find the line index of the selected item
	selectedLine := m.findSelectedLine(lines)

	// Calculate scroll window
	halfWindow := visibleHeight / 2
	start := selectedLine - halfWindow
	end := selectedLine + halfWindow

	// Adjust bounds
	if start < 0 {
		start = 0
		end = visibleHeight
	}
	if end > len(lines) {
		end = len(lines)
		start = end - visibleHeight
		if start < 0 {
			start = 0
		}
	}

	return start, end
}

// findSelectedLine finds the line index of the currently selected script
func (m *MainListScreen) findSelectedLine(lines []string) int {
	// Count scope headers and estimate position
	scopeHeaders := 0
	for i := 0; i <= m.selectedIdx && i < len(m.scripts); i++ {
		if i == 0 || m.scripts[i].Script.Scope != m.scripts[i-1].Script.Scope {
			scopeHeaders++
		}
	}

	// Rough estimate: selected index + scope headers + spacing
	estimatedLine := m.selectedIdx + scopeHeaders
	if estimatedLine >= len(lines) {
		estimatedLine = len(lines) - 1
	}

	return estimatedLine
}

// truncateString truncates a string to the specified length with ellipsis
func (m *MainListScreen) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
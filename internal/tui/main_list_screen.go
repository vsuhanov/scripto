package tui

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/internal/script"
	"scripto/internal/services"
	"scripto/internal/storage"

	"unicode/utf8"
)

type MainListScreen struct {
	scripts           []script.MatchResult
	selectedItemIndex int
	config            storage.Config
	configPath        string
	container         *services.Container

	width         int
	maxWidth      int
	visibleHeight int
	height        int
	ready         bool
	err           error

	showHelp      bool
	focusedPane   string
	editMode      bool
	externalEdit  bool
	nameEditMode  bool
	deleteMode    bool
	confirmDelete bool
	statusMsg     string
	quitting      bool

	viewport viewport.Model
}

func NewMainListScreen(container *services.Container) (*MainListScreen, error) {
	configPath, err := storage.GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	return &MainListScreen{
		configPath:  configPath,
		container:   container,
		focusedPane: "list",
		viewport:    viewport.New(50, 10),
	}, nil
}

func (m *MainListScreen) SetStatusMessage(msg string) {
	m.statusMsg = msg
}

func (m *MainListScreen) RefreshScripts() {
	m.ready = false
}

func (m *MainListScreen) Init() tea.Cmd {
	return tea.Batch(
		loadScripts(m.container.ScriptService),
		tea.EnterAltScreen,
	)
}

func (m *MainListScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3
		footerHeight := 3
		availableHeight := msg.Height - headerHeight - footerHeight

		log.Printf("WindowSize - Width: %d, Height: %d, HeaderHeight: %d, FooterHeight: %d, AvailableHeight: %d, ListWidth: %d, PreviewWidth: %d", msg.Width, msg.Height, headerHeight, footerHeight, availableHeight, min(50, msg.Width/2), msg.Width-min(50, msg.Width/2)-4)

		listWidth := min(50, msg.Width/2)
		previewWidth := msg.Width - listWidth - 4

		m.visibleHeight = availableHeight
		m.maxWidth = previewWidth
		m.viewport.Width = previewWidth
		m.viewport.Height = availableHeight
		return m, nil

	case ScriptsLoadedMsg:
		m.scripts = []script.MatchResult(msg)
		m.ready = true
		if len(m.scripts) > 0 && m.selectedItemIndex >= len(m.scripts) {
			m.selectedItemIndex = 0
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
		return m, func() tea.Msg {
			return ExitAppMsg{exitCode: ExitBuiltinComplete, message: ""}
		}

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
			selected := m.scripts[m.selectedItemIndex]
			return m, func() tea.Msg {
				return ExecuteScriptMsg{scriptPath: selected.Script.FilePath}
			}
		}

	case "e":
		return m.handleInlineEdit()

	case "E":
		return m.handleExternalEdit()

	case "d":
		return m.handleDeleteRequest()

	case "D":
		return m.handleImmediateDelete()

	case "j", "down":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedItemIndex = min(m.selectedItemIndex+1, len(m.scripts)-1)
			return m, m.updatePreview()
		}

	case "k", "up":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedItemIndex = max(0, m.selectedItemIndex-1)
			return m, m.updatePreview()
		}

	case "g":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedItemIndex = 0
			return m, m.updatePreview()
		}

	case "G":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedItemIndex = len(m.scripts) - 1
			return m, m.updatePreview()
		}
	}

	if m.focusedPane == "preview" {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *MainListScreen) handleInlineEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedItemIndex]
	return m, func() tea.Msg {
		return ShowScriptEditorMsg{script: selected.Script}
	}
}

func (m *MainListScreen) handleExternalEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedItemIndex]
	scriptPath := selected.Script.FilePath

	if scriptPath == "" {
		m.statusMsg = "Cannot edit: script has no file path"
		return m, nil
	}

	return m, func() tea.Msg {
		return EditScriptExternalMsg{scriptPath: scriptPath}
	}
}

func (m *MainListScreen) handleDeleteRequest() (tea.Model, tea.Cmd) {
	if len(m.scripts) > 0 {
		m.confirmDelete = true
		m.statusMsg = "Delete script? (y/n)"
	}
	return m, nil
}

func (m *MainListScreen) handleImmediateDelete() (tea.Model, tea.Cmd) {
	if len(m.scripts) > 0 {
		selected := m.scripts[m.selectedItemIndex]
		return m, func() tea.Msg {
			return DeleteScriptMsg{script: selected.Script}
		}
	}
	return m, nil
}

func (m *MainListScreen) handleDeleteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirmDelete = false
		if len(m.scripts) > 0 {
			selected := m.scripts[m.selectedItemIndex]
			return m, func() tea.Msg {
				return DeleteScriptMsg{script: selected.Script}
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

func (m *MainListScreen) updatePreview() tea.Cmd {
	if len(m.scripts) == 0 || m.selectedItemIndex >= len(m.scripts) {
		m.viewport.SetContent("")
		return nil
	}

	selected := m.scripts[m.selectedItemIndex]
	content := m.formatPreviewContent(selected)
	m.viewport.SetContent(content)
	return nil
}

func (m *MainListScreen) formatPreviewContent(script script.MatchResult) string {
	var sections []string

	title := m.formatPreviewTitle(script)
	sections = append(sections, title)

	metadata := m.formatPreviewMetadata(script)
	sections = append(sections, metadata)

	if script.Script.Description != "" {
		description := m.formatPreviewDescription(script.Script.Description, m.maxWidth)
		sections = append(sections, description)
	}

	if script.Script.FilePath != "" {
		fileContent := m.formatPreviewFileContent(script.Script.FilePath, m.maxWidth)
		sections = append(sections, fileContent)
	}

	return strings.Join(sections, "\n\n")
}

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

func (m *MainListScreen) formatPreviewMetadata(selected script.MatchResult) string {
	var metadata []string

	if selected.Script.Scope == "global" {
		metadata = append(metadata, "Scope: global")
	} else {
		scopeLabel := m.getScopeDisplayName(selected.Script.Scope)
		metadata = append(metadata, fmt.Sprintf("Scope: %s", scopeLabel))

		dir := selected.Script.Scope
		if len(dir) > 50 {
			dir = "..." + dir[len(dir)-47:]
		}
		metadata = append(metadata, fmt.Sprintf("Directory: %s", dir))
	}

	if selected.Script.FilePath != "" {
		filename := filepath.Base(selected.Script.FilePath)
		metadata = append(metadata, fmt.Sprintf("File: %s", filename))
	}

	return PreviewContentStyle.Render(strings.Join(metadata, "\n"))
}

func (m *MainListScreen) formatPreviewDescription(description string, maxWidth int) string {
	title := PreviewTitleStyle.Render("Description:")
	wrappedDesc := m.wrapText(description, maxWidth)
	content := PreviewContentStyle.Render(wrappedDesc)
	return title + "\n" + content
}

func (m *MainListScreen) formatPreviewFileContent(filePath string, maxWidth int) string {
	content, err := readScriptFile(filePath)
	if err != nil {
		return PreviewContentStyle.Render(fmt.Sprintf("Error reading file: %v", err))
	}

	title := PreviewTitleStyle.Render("File Content:")

	lines := strings.Split(content, "\n")
	if len(lines) > 10 {
		lines = lines[:10]
		lines = append(lines, "...")
	}

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

func (m *MainListScreen) getScopeDisplayName(scope string) string {
	if m.container != nil {
		return m.container.ScriptService.GetScopeDisplayName(scope)
	}
	return scope
}

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
		if len(currentLine)+len(word)+1 > width && currentLine != "" {
			lines = append(lines, currentLine)
			currentLine = word
		} else if currentLine == "" {
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n")
}

func (m *MainListScreen) renderMainView() string {

	listWidth := min(50, m.width/2)
	previewWidth := m.width - listWidth

	header := m.renderHeader()
	footer := m.renderFooter()
	availableHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	log.Printf("RenderMainView - Width: %d, Height: %d, AvailableHeight: %d, ListWidth: %d, PreviewWidth: %d", m.width, m.height, availableHeight, listWidth, previewWidth)
	listView := m.renderList(listWidth, availableHeight)
	// previewView := m.renderPreview(previewWidth, availableHeight)

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		listView,
		// " ",
		// previewView,
	)

	// return mainContent
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		mainContent,
		footer,
	)
}

func (m *MainListScreen) renderHeader() string {
	title := TitleStyle.Render("Scripto - Script Manager")
	help := HelpStyle.Render("? for help • q to quit")
	log.Printf("Header - Width: %d, TitleWidth: %d, HelpWidth: %d, Spacing: %d", m.width, lipgloss.Width(title), lipgloss.Width(help), max(0, m.width-lipgloss.Width(title)-lipgloss.Width(help)))

	return title
	// return HeaderStyle.Width(m.width).Height(3).Render(
	// 	lipgloss.JoinHorizontal(
	// 		lipgloss.Center,
	// 		title,
	// 		strings.Repeat(" ", max(0, m.width-lipgloss.Width(title)-lipgloss.Width(help))),
	// 		help,
	// 	),
	// )
}

type scopedSubList struct {
	scopeTitle          string
	formattedScopeTitle string
	items               []string
}

func (m *MainListScreen) renderList(maxWidth, maxHeight int) string {
	totalVerticalBorder := 2
	totalHorizontalBorder := 2
	maxListItemWidth := maxWidth - totalHorizontalBorder
	if len(m.scripts) == 0 {
		emptyMsg := "No scripts found.\nUse 'scripto add' to create some scripts."
		return ListStyle.
			Width(maxWidth).
			Height(maxHeight).
			Render(emptyMsg)
	}
	// return ListStyle.
	// 	Width(width).
	// 	Height(height).
	// 	Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9900")).Bold(false).Render("mytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nytext \n my text \n"))

	var items []string
	var currentScope string

	for i, script := range m.scripts {
		if script.Script.Scope != currentScope {
			if currentScope != "" {
			}

			scopeHeader := m.formatScopeHeader(script.Script.Scope)
			items = append(items, scopeHeader)
			currentScope = script.Script.Scope
		}

		item := m.formatScriptItem(script, i, maxListItemWidth, 3)
		items = append(items, item)
	}

	content := strings.Join(items, "\n")

	maxPossibleContentHeight := max(1, maxHeight-totalVerticalBorder)

	var start, end int;
	if len(items) > maxPossibleContentHeight {
		selectedLine := m.findSelectedLine(items)
		start, end = calculateScrollWindow(selectedLine, len(items), maxHeight-totalVerticalBorder)
		items = items[start:end]
		content = strings.Join(items, "\n")
	}

	log.Printf("renderList - maxWidth: %v, maxHeight: %v, len(m.scripts): %v, maxHeight-totalVerticalBorder: %v, maxPossibleContentHeight: %v, start: %v, end: %v", maxWidth, maxHeight, len(m.scripts), maxHeight-totalVerticalBorder, maxPossibleContentHeight, start, end)

	style := ListStyle
	if m.focusedPane == "list" {
		style = ListFocusedStyle
	}

	style = style.
		Width(maxListItemWidth).
		MaxWidth(maxWidth).
		Height(maxHeight - totalVerticalBorder).
		MaxHeight(maxHeight)

	rendered := style.Render(content)
	// renderedHeight := lipgloss.Height(rendered)

	// for renderedHeight > contentHeight && len(lines) > 1 {
	// 	lines = lines[:len(lines)-1]
	// 	content = strings.Join(lines, "\n")
	// 	rendered = style.Render(content)
	// 	renderedHeight = lipgloss.Height(rendered)
	// }

	return rendered
}

func (m *MainListScreen) renderPreview(width, height int) string {
	previewStyle := PreviewStyle.Width(width).Height(height)
	if m.focusedPane == "preview" {
		previewStyle = PreviewFocusedStyle.Width(width).Height(height)
	}

	return previewStyle.Render(m.viewport.View())
}

func (m *MainListScreen) renderFooter() string {
	var statusText string
	if m.confirmDelete {
		statusText = "Delete script? (y/n)"
	} else if m.statusMsg != "" {
		statusText = m.statusMsg
	} else {
		statusText = ""
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

func (m *MainListScreen) formatScriptItem(script script.MatchResult, index int, maxWidth int, indent int) string {
	var parts []string

	scopeIndicator := FormatScopeIndicator(script.Script.Scope)
	parts = append(parts, scopeIndicator)

	var displayName string
	if script.Script.Name != "" {
		displayName = script.Script.Name
	} else {
		displayName = m.truncateString(script.Script.FilePath, 60)
	}

	if utf8.RuneCountInString(displayName) > maxWidth {
		displayName = m.truncateString(displayName, (maxWidth-4-indent)) + "…"
	}

	item := ListItemStyle.Bold(false).Width(maxWidth - 4).Render(displayName)

	if index == m.selectedItemIndex {
		item = ListItemSelectedStyle.Width(maxWidth - 4).Render(displayName)
	}

	parts = append(parts, item)

	// return lipgloss.NewStyle().Width(maxWidth-4).Background(lipgloss.Color("#ff9900")).Render(strings.Join(parts, " "))
	return strings.Join(parts, " ")
	// return lipgloss.NewStyle().Width(maxWidth-4).Background(lipgloss.Color("#ff9900")).Render()

}

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

func (m *MainListScreen) formatDirectoryName(dir string) string {
	if dir == "global" {
		return "Global Scripts"
	}

	fullPath := dir

	if len(fullPath) > 100 {
	}

	return fullPath
}

func calculateScrollWindow(selectedLine, totalLines, visibleHeight int) (int, int) {
	halfWindow := visibleHeight / 2
	start := selectedLine - halfWindow
	end := selectedLine + halfWindow

	if start < 0 {
		start = 0
	}
	if end > totalLines {
		end = totalLines
	}
	if end-start < visibleHeight && totalLines > visibleHeight {
		if start > 0 {
			start = end - visibleHeight
		} else {
			end = start + visibleHeight
		}
	}
	if start < 0 {
		start = 0
	}

	return start, end
}

// TODO: this feels extremely unreliable, better to keep some structures with scopes
func (m *MainListScreen) findSelectedLine(lines []string) int {
	scopeHeaders := 0
	for i := 0; i <= m.selectedItemIndex && i < len(m.scripts); i++ {
		if i == 0 || m.scripts[i].Script.Scope != m.scripts[i-1].Script.Scope {
			scopeHeaders++
		}
	}

	estimatedLine := m.selectedItemIndex + scopeHeaders
	if estimatedLine >= len(lines) {
		estimatedLine = len(lines) - 1
	}

	return estimatedLine
}

func (m *MainListScreen) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

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

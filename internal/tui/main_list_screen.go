package tui

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/entities"
	"scripto/internal/services"
	"scripto/internal/storage"
)

type MainListScreen struct {
	scripts           []entities.Script
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
		m.loadScripts(),
		tea.EnterAltScreen,
	)
}

func (m *MainListScreen) loadScripts() tea.Cmd {
	return func() tea.Msg {
		scripts, err := m.container.ScriptService.FindAllScripts()
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to find scripts: %w", err))
		}

		result := make([]entities.Script, len(scripts))

		for i, v := range scripts {
			result[i] = v.Script
		}

		return ScriptsLoadedMsg(result)
	}
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
		m.scripts = []entities.Script(msg)
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
				return ExecuteScriptMsg{scriptPath: selected.FilePath}
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
		return ShowScriptEditorMsg{script: selected}
	}
}

func (m *MainListScreen) handleExternalEdit() (tea.Model, tea.Cmd) {
	if len(m.scripts) == 0 {
		return m, nil
	}

	selected := m.scripts[m.selectedItemIndex]
	scriptPath := selected.FilePath

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
			return DeleteScriptMsg{script: selected}
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
				return DeleteScriptMsg{script: selected}
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
	// #FIXME: content for the preview need to be set somehow differntly
	if len(m.scripts) == 0 || m.selectedItemIndex >= len(m.scripts) {
		m.viewport.SetContent("")
		return nil
	}

	selected := m.scripts[m.selectedItemIndex]
	content := m.formatPreviewContent(selected)
	m.viewport.SetContent(content)
	return nil
}

func (m *MainListScreen) getScopeDisplayName(scope string) string {
	if m.container != nil {
		return m.container.ScriptService.GetScopeDisplayName(scope)
	}
	return scope
}

func (m *MainListScreen) renderMainView() string {
	listWidth := min(50, m.width/2)
	previewWidth := m.width - listWidth

	header := m.renderHeader()
	footer := m.renderFooter()
	availableHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	log.Printf("RenderMainView - Width: %d, Height: %d, AvailableHeight: %d, ListWidth: %d, PreviewWidth: %d", m.width, m.height, availableHeight, listWidth, previewWidth)
	listView := m.renderList(listWidth, availableHeight)
	previewView := m.renderPreview(previewWidth, availableHeight)

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		listView,
		// " ",
		previewView,
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

func (m *MainListScreen) renderPreview(maxWidth, maxHeight int) string {
	totalVerticalBorder := 2
	totalHorizontalBorder := 2

	previewStyle := PreviewStyle
	if m.focusedPane == "preview" {
		previewStyle = PreviewFocusedStyle
	}

	log.Printf("renderPreview - maxWidth: %v, maxHeight: %v", maxWidth, maxHeight)

	previewStyle = previewStyle.
		Width(maxWidth - totalHorizontalBorder).
		MaxWidth(maxWidth).
		Height(maxHeight - totalVerticalBorder).
		MaxHeight(maxHeight)

	rendered := previewStyle.Render("Preview")
	log.Printf("renderPreview - rendered - rendered.Width: %v, rendered.Height: %v", lipgloss.Width(rendered), lipgloss.Height(rendered))

	return rendered
}

func (m *MainListScreen) formatPreviewContent(script entities.Script) string {
	var sections []string

	title := m.formatPreviewTitle(script)
	sections = append(sections, title)

	metadata := m.formatPreviewMetadata(script)
	sections = append(sections, metadata)

	if script.Description != "" {
		description := m.formatPreviewDescription(script.Description, m.maxWidth)
		sections = append(sections, description)
	}

	if script.FilePath != "" {
		fileContent := m.formatPreviewFileContent(script.FilePath, m.maxWidth)
		sections = append(sections, fileContent)
	}

	return strings.Join(sections, "\n\n")
}

func (m *MainListScreen) formatPreviewTitle(selected entities.Script) string {
	scopeIndicator := FormatScopeIndicator(selected.Scope)

	var title string
	if selected.Name != "" {
		title = selected.Name
	} else {
		title = "Unnamed Script"
	}

	return PreviewTitleStyle.Render(fmt.Sprintf("%s %s", scopeIndicator, title))
}

func (m *MainListScreen) formatPreviewMetadata(selected entities.Script) string {
	var metadata []string

	if selected.Scope == "global" {
		metadata = append(metadata, "Scope: global")
	} else {
		scopeLabel := m.getScopeDisplayName(selected.Scope)
		metadata = append(metadata, fmt.Sprintf("Scope: %s", scopeLabel))

		dir := selected.Scope
		if len(dir) > 50 {
			dir = "..." + dir[len(dir)-47:]
		}
		metadata = append(metadata, fmt.Sprintf("Directory: %s", dir))
	}

	if selected.FilePath != "" {
		filename := filepath.Base(selected.FilePath)
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

func (m *MainListScreen) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
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

// #TODO: move this to some sort of math utils
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

// TODO: this is also just a utility function
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

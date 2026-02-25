package tui

import (
	"fmt"
	"log"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/entities"
	"scripto/internal/services"
	"scripto/internal/storage"
)

const (
	previewFocusName = iota
	previewFocusDirectory
	previewFocusViewport
	previewFocusCount
)

type MainListScreen struct {
	scripts           []*entities.Script
	selectedItemIndex int
	selectedScript    *entities.Script
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

	previewViewport      viewport.Model
	previewViewportReady bool
	previewNavMode       bool
	previewFocusedElement int
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
	}, nil
}

func (m *MainListScreen) SetStatusMessage(msg string) {
	m.statusMsg = msg
}

func (m *MainListScreen) RefreshScripts() {
	m.ready = false
}

func (m *MainListScreen) updateSelectedScript() {
	if m.selectedItemIndex >= 0 && m.selectedItemIndex < len(m.scripts) {
		m.selectedScript = m.scripts[m.selectedItemIndex]
		m.updatePreviewViewportContent()
	} else {
		m.selectedScript = nil
		m.previewViewport.SetContent("")
	}
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

		return ScriptsLoadedMsg(scripts)
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

		viewportWidth := previewWidth - 4
		viewportHeight := max(1, availableHeight-12)
		if !m.previewViewportReady {
			m.previewViewport = viewport.New(viewportWidth, viewportHeight)
			m.previewViewportReady = true
		} else {
			m.previewViewport.Width = viewportWidth
			m.previewViewport.Height = viewportHeight
		}

		return m, nil

	case ScriptsLoadedMsg:
		m.scripts = []*entities.Script(msg)
		m.ready = true
		if len(m.scripts) > 0 && m.selectedItemIndex >= len(m.scripts) {
			m.selectedItemIndex = 0
		}
		m.updateSelectedScript()
		return m, nil

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

func (m *MainListScreen) updatePreviewViewportContent() {
	if m.selectedScript != nil && m.selectedScript.FilePath != "" {
		content, err := readScriptFile(m.selectedScript.FilePath)
		if err == nil {
			m.previewViewport.SetContent(content)
			m.previewViewport.GotoTop()
		} else {
			m.previewViewport.SetContent(fmt.Sprintf("Error reading file: %v", err))
		}
	} else {
		m.previewViewport.SetContent("")
	}
}

func (m *MainListScreen) isPreviewElementFocusable(element int) bool {
	switch element {
	case previewFocusName:
		return true
	case previewFocusDirectory:
		return m.selectedScript != nil && m.selectedScript.Scope != "global"
	case previewFocusViewport:
		return m.selectedScript != nil && m.selectedScript.FilePath != ""
	}
	return false
}

func (m *MainListScreen) nextPreviewFocus(current, delta int) int {
	for i := 0; i < previewFocusCount; i++ {
		next := (current + delta + previewFocusCount) % previewFocusCount
		current = next
		if m.isPreviewElementFocusable(next) {
			return next
		}
	}
	return current
}

func (m *MainListScreen) firstFocusablePreviewElement() int {
	for i := 0; i < previewFocusCount; i++ {
		if m.isPreviewElementFocusable(i) {
			return i
		}
	}
	return 0
}

func (m *MainListScreen) getFocusedPreviewText() string {
	if m.selectedScript == nil {
		return ""
	}
	switch m.previewFocusedElement {
	case previewFocusName:
		return m.selectedScript.Name
	case previewFocusDirectory:
		return m.selectedScript.Scope
	case previewFocusViewport:
		content, _ := readScriptFile(m.selectedScript.FilePath)
		return content
	}
	return ""
}

func (m *MainListScreen) handlePreviewNavKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.previewNavMode = false
		m.focusedPane = "list"
		return m, nil

	case "tab":
		m.previewFocusedElement = m.nextPreviewFocus(m.previewFocusedElement, 1)
		return m, nil

	case "shift+tab":
		m.previewFocusedElement = m.nextPreviewFocus(m.previewFocusedElement, -1)
		return m, nil

	case "y":
		text := m.getFocusedPreviewText()
		_ = clipboard.WriteAll(text)
		m.statusMsg = "Copied to clipboard"
		return m, nil

	case "j", "down":
		if m.previewFocusedElement == previewFocusViewport {
			m.previewViewport.ScrollDown(1)
		}
		return m, nil

	case "k", "up":
		if m.previewFocusedElement == previewFocusViewport {
			m.previewViewport.ScrollUp(1)
		}
		return m, nil

	case "h":
		if m.previewFocusedElement == previewFocusViewport {
			m.previewViewport.HalfPageUp()
		}
		return m, nil

	case "l":
		if m.previewFocusedElement == previewFocusViewport {
			m.previewViewport.HalfPageDown()
		}
		return m, nil
	}

	return m, nil
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
	}

	if m.previewNavMode {
		return m.handlePreviewNavKeys(msg)
	}

	switch msg.String() {
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
		if m.focusedPane == "preview" {
			m.previewNavMode = true
			m.previewFocusedElement = m.firstFocusablePreviewElement()
			return m, nil
		}
		if m.selectedScript != nil {
			return m, func() tea.Msg {
				return ExecuteScriptMsg{scriptPath: m.selectedScript.FilePath}
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
			m.updateSelectedScript()
			return m, nil
		}

	case "k", "up":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedItemIndex = max(0, m.selectedItemIndex-1)
			m.updateSelectedScript()
			return m, nil
		}

	case "g":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedItemIndex = 0
			m.updateSelectedScript()
			return m, nil
		}

	case "G":
		if m.focusedPane == "list" && len(m.scripts) > 0 {
			m.selectedItemIndex = len(m.scripts) - 1
			m.updateSelectedScript()
			return m, nil
		}
	}

	if m.focusedPane == "preview" {
		var cmd tea.Cmd
		return m, cmd
	}

	return m, nil
}

func (m *MainListScreen) handleInlineEdit() (tea.Model, tea.Cmd) {
	if m.selectedScript == nil {
		return m, nil
	}

	return m, func() tea.Msg {
		return ShowScriptEditorMsg{script: m.selectedScript}
	}
}

func (m *MainListScreen) handleExternalEdit() (tea.Model, tea.Cmd) {
	if m.selectedScript == nil {
		return m, nil
	}

	scriptPath := m.selectedScript.FilePath

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
	if m.selectedScript != nil {
		return m, func() tea.Msg {
			return DeleteScriptMsg{script: m.selectedScript}
		}
	}
	return m, nil
}

func (m *MainListScreen) handleDeleteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirmDelete = false
		if m.selectedScript != nil {
			return m, func() tea.Msg {
				return DeleteScriptMsg{script: m.selectedScript}
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

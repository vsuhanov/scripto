package tui

import (
	"fmt"
	"log"
	"sort"
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

const (
	sortDefault = iota
	sortLastExecution
	sortFrequency
	sortAlphabetic
	sortModeCount
)

type MainListScreen struct {
	scripts           []*entities.Script
	allScripts        []*entities.Script
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
	pendingGKey   bool

	sortMode       int
	scriptStats    map[string]services.ScriptStats
	frecencyScores map[string]float64
	showAllScopes  bool

	previewViewport       viewport.Model
	previewViewportReady  bool
	previewNavMode        bool
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

func (m *MainListScreen) activeScripts() []*entities.Script {
	if m.showAllScopes {
		return m.allScripts
	}
	return m.scripts
}

type listItem struct {
	script *entities.Script
	scope  string
}

func (li listItem) isSelectableHeader() bool {
	return li.script == nil && li.scope != "global"
}

func (m *MainListScreen) buildListItems() []listItem {
	scripts := m.activeScripts()
	var items []listItem
	var currentScope string
	for _, s := range scripts {
		if s.Scope != currentScope {
			items = append(items, listItem{scope: s.Scope})
			currentScope = s.Scope
		}
		items = append(items, listItem{script: s})
	}
	return items
}

func (m *MainListScreen) SetStatusMessage(msg string) {
	m.statusMsg = msg
}

func (m *MainListScreen) RefreshScripts() {
	m.ready = false
}

func (m *MainListScreen) updateSelectedScript() {
	items := m.buildListItems()
	if m.selectedItemIndex >= 0 && m.selectedItemIndex < len(items) {
		m.selectedScript = items[m.selectedItemIndex].script
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

type scriptsWithStatsMsg struct {
	scripts        []*entities.Script
	allScripts     []*entities.Script
	stats          map[string]services.ScriptStats
	frecencyScores map[string]float64
}

func (m *MainListScreen) loadScripts() tea.Cmd {
	return func() tea.Msg {
		scripts, err := m.container.ScriptService.FindAllScripts()
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to find scripts: %w", err))
		}

		allScripts, err := m.container.ScriptService.FindAllScopesScripts()
		if err != nil {
			allScripts = scripts
		}

		var stats map[string]services.ScriptStats
		var frecencyScores map[string]float64
		if m.container.ExecutionHistoryService != nil {
			stats, _ = m.container.ExecutionHistoryService.GetAllScriptStats()
			frecencyScores = m.container.ExecutionHistoryService.GetFrecencyScores()
		}
		if stats == nil {
			stats = map[string]services.ScriptStats{}
		}
		if frecencyScores == nil {
			frecencyScores = map[string]float64{}
		}

		return scriptsWithStatsMsg{scripts: scripts, allScripts: allScripts, stats: stats, frecencyScores: frecencyScores}
	}
}

func (m *MainListScreen) sortScripts() {
	less := func(i, j *entities.Script) bool {
		switch m.sortMode {
		case sortDefault:
			return m.frecencyScores[i.ID] > m.frecencyScores[j.ID]
		case sortLastExecution:
			si := m.scriptStats[i.ID]
			sj := m.scriptStats[j.ID]
			return si.LastExecutionTime.After(sj.LastExecutionTime)
		case sortFrequency:
			si := m.scriptStats[i.ID]
			sj := m.scriptStats[j.ID]
			return si.ExecutionCount > sj.ExecutionCount
		case sortAlphabetic:
			return i.Name < j.Name
		}
		return false
	}

	sortList := func(list []*entities.Script) []*entities.Script {
		scopeOrder := []string{}
		scopeMap := map[string][]*entities.Script{}
		for _, s := range list {
			if _, exists := scopeMap[s.Scope]; !exists {
				scopeOrder = append(scopeOrder, s.Scope)
			}
			scopeMap[s.Scope] = append(scopeMap[s.Scope], s)
		}
		for scope := range scopeMap {
			group := scopeMap[scope]
			sort.SliceStable(group, func(i, j int) bool {
				return less(group[i], group[j])
			})
		}
		result := make([]*entities.Script, 0, len(list))
		for _, scope := range scopeOrder {
			result = append(result, scopeMap[scope]...)
		}
		return result
	}

	m.scripts = sortList(m.scripts)
	m.allScripts = sortList(m.allScripts)
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
		m.allScripts = m.scripts
		m.ready = true
		if len(m.buildListItems()) > 0 && m.selectedItemIndex >= len(m.buildListItems()) {
			m.selectedItemIndex = 0
		}
		m.sortScripts()
		m.updateSelectedScript()
		return m, nil

	case scriptsWithStatsMsg:
		m.scripts = msg.scripts
		m.allScripts = msg.allScripts
		m.scriptStats = msg.stats
		m.frecencyScores = msg.frecencyScores
		m.ready = true
		if len(m.buildListItems()) > 0 && m.selectedItemIndex >= len(m.buildListItems()) {
			m.selectedItemIndex = 0
		}
		m.sortScripts()
		m.updateSelectedScript()
		return m, nil

	case ScriptDeletedMsg:
		m.scripts = removeScript(m.scripts, msg.script)
		m.allScripts = removeScript(m.allScripts, msg.script)
		if m.selectedItemIndex >= len(m.buildListItems()) && m.selectedItemIndex > 0 {
			m.selectedItemIndex--
		}
		m.selectedScript = nil
		m.previewViewport.SetContent("")
		m.updateSelectedScript()
		m.statusMsg = "Script deleted"
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

	if m.pendingGKey {
		m.pendingGKey = false
		switch msg.String() {
		case "h":
			if m.selectedScript != nil {
				scriptID := m.selectedScript.ID
				return m, func() tea.Msg {
					return ShowExecutionHistoryMsg{scriptID: scriptID}
				}
			}
			return m, nil
		case "H":
			return m, func() tea.Msg {
				return ShowExecutionHistoryMsg{}
			}
		case "g":
			if m.focusedPane == "list" {
				items := m.buildListItems()
				if len(items) > 0 {
					m.selectedItemIndex = 0
					if items[0].script == nil && items[0].scope == "global" && len(items) > 1 {
						m.selectedItemIndex = 1
					}
					m.updateSelectedScript()
				}
			}
			return m, nil
		}
		return m, nil
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
		items := m.buildListItems()
		if m.selectedItemIndex < len(items) {
			item := items[m.selectedItemIndex]
			if item.isSelectableHeader() {
				dir := item.scope
				return m, func() tea.Msg { return CdToDirectoryMsg{dir: dir} }
			} else if item.script != nil {
				script := item.script
				return m, func() tea.Msg { return ExecuteScriptMsg{script: script} }
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
		if m.focusedPane == "list" {
			items := m.buildListItems()
			if len(items) > 0 {
				m.selectedItemIndex = (m.selectedItemIndex + 1) % len(items)
				if items[m.selectedItemIndex].script == nil && items[m.selectedItemIndex].scope == "global" {
					m.selectedItemIndex = (m.selectedItemIndex + 1) % len(items)
				}
				m.updateSelectedScript()
			}
			return m, nil
		}

	case "k", "up":
		if m.focusedPane == "list" {
			items := m.buildListItems()
			if len(items) > 0 {
				m.selectedItemIndex = (m.selectedItemIndex - 1 + len(items)) % len(items)
				if items[m.selectedItemIndex].script == nil && items[m.selectedItemIndex].scope == "global" {
					m.selectedItemIndex = (m.selectedItemIndex - 1 + len(items)) % len(items)
				}
				m.updateSelectedScript()
			}
			return m, nil
		}

	case "g":
		m.pendingGKey = true
		return m, nil

	case "G":
		if m.focusedPane == "list" {
			items := m.buildListItems()
			if len(items) > 0 {
				m.selectedItemIndex = len(items) - 1
				if items[m.selectedItemIndex].script == nil && items[m.selectedItemIndex].scope == "global" && m.selectedItemIndex > 0 {
					m.selectedItemIndex--
				}
				m.updateSelectedScript()
			}
			return m, nil
		}

	case "S":
		m.showAllScopes = !m.showAllScopes
		if m.selectedItemIndex >= len(m.buildListItems()) {
			m.selectedItemIndex = 0
		}
		m.updateSelectedScript()
		if m.showAllScopes {
			m.statusMsg = "Showing all scopes"
		} else {
			m.statusMsg = "Showing current scopes only"
		}
		return m, nil

	case "o":
		m.sortMode = (m.sortMode + 1) % sortModeCount
		m.sortScripts()
		m.updateSelectedScript()
		m.statusMsg = sortModeName(m.sortMode)
		return m, nil

	case "O":
		m.sortMode = (m.sortMode - 1 + sortModeCount) % sortModeCount
		m.sortScripts()
		m.updateSelectedScript()
		m.statusMsg = sortModeName(m.sortMode)
		return m, nil

	case "y":
		if m.focusedPane == "list" && m.selectedScript != nil {
			return m, func() tea.Msg {
				return CopyScriptToClipboardMsg{script: m.selectedScript}
			}
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

	return m, func() tea.Msg {
		return EditScriptExternalMsg{script: m.selectedScript}
	}
}

func removeScript(list []*entities.Script, target *entities.Script) []*entities.Script {
	result := make([]*entities.Script, 0, len(list))
	for _, s := range list {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}

func (m *MainListScreen) handleDeleteRequest() (tea.Model, tea.Cmd) {
	if m.selectedScript != nil {
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
		keyHints = HelpStyle.Render("↵: execute • e: edit • E: external • d: delete • y: copy • tab: switch pane")
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
  y            Copy command to clipboard

Other:
  S            Toggle all scopes visibility
  ?            Toggle this help
  q, Ctrl+C    Quit

Press ? or Esc to close this help.`

	return HelpScreenStyle.Width(m.width).Height(m.height).Render(helpText)
}

func sortModeName(mode int) string {
	switch mode {
	case sortDefault:
		return "Sort: default"
	case sortLastExecution:
		return "Sort: last execution"
	case sortFrequency:
		return "Sort: frequency"
	case sortAlphabetic:
		return "Sort: alphabetic"
	}
	return ""
}

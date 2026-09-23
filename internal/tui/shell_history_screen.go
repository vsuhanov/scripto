package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/services"
)

const (
	shellHistoryPageSize      = 200
	shellHistoryRetentionDays = 90
	shellHistoryStatusOK      = "✓"
	shellHistoryStatusUnknown = "·"

	shellHistoryStatusWidth = 5
	shellHistoryTimeWidth   = 16
	shellHistoryDirWidth    = 18
)

type ShellHistoryScreen struct {
	container *services.Container

	records []services.ShellHistoryRecord
	cwd     string

	filter        string
	filterRe      *regexp.Regexp
	filterInvalid bool
	filterInput   textinput.Model
	searchMode    bool
	cwdOnly       bool
	failuresOnly  bool

	pendingSaveID string

	width  int
	height int
	ready  bool
	err    error

	cursor      int
	detailVP    viewport.Model
	detailReady bool
}

type shellHistoryLoadedMsg struct {
	records []services.ShellHistoryRecord
}

func NewShellHistoryScreen(container *services.Container, width, height int) *ShellHistoryScreen {
	cwd, _ := os.Getwd()

	fi := textinput.New()
	fi.Placeholder = "regex filter..."
	fi.CharLimit = 200

	s := &ShellHistoryScreen{
		container:   container,
		cwd:         cwd,
		filterInput: fi,
		width:       width,
		height:      height,
	}
	if width > 0 && height > 0 {
		_, vpH := s.calcHeights(height)
		s.detailVP = viewport.New(width-4, max(1, vpH))
		s.detailReady = true
	}
	return s
}

func (s *ShellHistoryScreen) calcHeights(height int) (listHeight, vpHeight int) {
	available := height - 6
	listHeight = available / 2
	vpHeight = available - listHeight - 4
	if vpHeight < 1 {
		vpHeight = 1
	}
	return
}

// visibleRows is how many history rows fit in the list pane: the pane height
// minus the column header, its separator rule, and the search bar when open.
func (s *ShellHistoryScreen) visibleRows() int {
	listHeight, _ := s.calcHeights(s.height)
	rows := listHeight - 2
	if s.searchMode {
		rows--
	}
	return max(1, rows)
}

func (s *ShellHistoryScreen) contentWidth() int {
	return max(20, s.width-4)
}

func (s *ShellHistoryScreen) commandWidth() int {
	return max(10, s.contentWidth()-shellHistoryStatusWidth-shellHistoryTimeWidth-shellHistoryDirWidth-8)
}

func statusCell(r services.ShellHistoryRecord) string {
	if r.ExitCode == nil {
		return shellHistoryStatusUnknown
	}
	if *r.ExitCode == 0 {
		return shellHistoryStatusOK
	}
	return "✗" + strconv.Itoa(*r.ExitCode)
}

func statusStyle(r services.ShellHistoryRecord) lipgloss.Style {
	if r.ExitCode == nil {
		return lipgloss.NewStyle().Foreground(mutedTextColor)
	}
	if *r.ExitCode == 0 {
		return lipgloss.NewStyle().Foreground(successColor)
	}
	return lipgloss.NewStyle().Foreground(errorColor)
}

// truncateCell trims to a display width (not a rune count) and pads back out to
// it, so every cell occupies exactly `width` terminal columns. It must be given
// unstyled text: measuring happens before any ANSI sequences are added.
func truncateCell(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return runewidth.FillRight(runewidth.Truncate(value, width, "…"), width)
}

func shellHistoryDirDisplay(dir string) string {
	if dir == "" {
		return "-"
	}
	return filepath.Base(dir)
}

func historyCell(value string, width int, style lipgloss.Style) string {
	return style.Render(" " + truncateCell(value, width) + " ")
}

func (s *ShellHistoryScreen) renderHeaderRow() string {
	style := lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	row := historyCell("", shellHistoryStatusWidth, style) +
		historyCell("Time", shellHistoryTimeWidth, style) +
		historyCell("Dir", shellHistoryDirWidth, style) +
		historyCell("Command", s.commandWidth(), style)

	rule := lipgloss.NewStyle().
		Foreground(borderColor).
		Render(strings.Repeat("─", s.contentWidth()))

	return row + "\n" + rule
}

func (s *ShellHistoryScreen) renderRow(r services.ShellHistoryRecord, selected bool) string {
	base, highlight := highlightStyles(selected)

	status := statusCell(r)
	statusText := base
	if !selected {
		statusText = statusStyle(r)
	}

	ts := ""
	if r.StartedAt > 0 {
		ts = time.Unix(r.StartedAt, 0).Format("2006-01-02 15:04")
	}

	cmdWidth := s.commandWidth()
	cmd := truncateCell(strings.Join(strings.Fields(r.Command), " "), cmdWidth)

	return historyCell(status, shellHistoryStatusWidth, statusText) +
		historyCell(ts, shellHistoryTimeWidth, base) +
		historyCell(shellHistoryDirDisplay(r.WorkingDirectory), shellHistoryDirWidth, base) +
		base.Render(" ") + renderHighlighted(cmd, s.filterRe, base, highlight) + base.Render(" ")
}

func (s *ShellHistoryScreen) renderList() string {
	var lines []string

	if s.searchMode {
		s.filterInput.Width = max(10, s.width-10)
		if s.filterInvalid {
			s.filterInput.TextStyle = lipgloss.NewStyle().Foreground(errorColor)
		} else {
			s.filterInput.TextStyle = lipgloss.NewStyle()
		}
		prefix := lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render("/")
		lines = append(lines, prefix+" "+s.filterInput.View())
	}

	lines = append(lines, s.renderHeaderRow())

	visible := s.visibleRows()
	start, end := 0, len(s.records)
	if len(s.records) > visible {
		start, end = calculateScrollWindow(s.cursor, len(s.records), visible)
	}
	for i := start; i < end; i++ {
		lines = append(lines, s.renderRow(s.records[i], i == s.cursor))
	}

	return ListStyle.Width(s.width - 2).Render(strings.Join(lines, "\n"))
}

func (s *ShellHistoryScreen) Init() tea.Cmd {
	return tea.Batch(s.pruneHistory(), s.loadHistory())
}

func (s *ShellHistoryScreen) MarkPendingSaved(scriptID string) tea.Cmd {
	if s.pendingSaveID == "" || scriptID == "" || s.container.ShellHistoryService == nil {
		s.pendingSaveID = ""
		return nil
	}
	id := s.pendingSaveID
	s.pendingSaveID = ""
	return func() tea.Msg {
		if err := s.container.ShellHistoryService.MarkSaved(id, scriptID); err != nil {
			return ErrorMsg(err)
		}
		return nil
	}
}

func (s *ShellHistoryScreen) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if s.container.ShellHistoryService == nil {
			return shellHistoryLoadedMsg{records: nil}
		}

		// Running a script writes two rows: the launcher line the user typed
		// ("scripto deploy", source=shell) and the resolved command
		// (source=scripto). Only the launcher line gets a RecordFinish, so it
		// is the one carrying the exit code and duration - show that, and hide
		// the resolved duplicate.
		query := services.ShellHistoryQuery{
			Filter:         s.filter,
			FailuresOnly:   s.failuresOnly,
			ExcludeSources: []string{services.ShellHistorySourceScripto},
			Limit:          shellHistoryPageSize,
		}
		if s.cwdOnly {
			query.WorkingDirectory = s.cwd
		}

		records, err := s.container.ShellHistoryService.GetHistory(query)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to load shell history: %w", err))
		}
		return shellHistoryLoadedMsg{records: records}
	}
}

func (s *ShellHistoryScreen) pruneHistory() tea.Cmd {
	return func() tea.Msg {
		if s.container.ShellHistoryService == nil {
			return nil
		}
		days := shellHistoryRetentionDays
		if raw := os.Getenv("SCRIPTO_HISTORY_RETENTION_DAYS"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil {
				days = parsed
			}
		}
		_ = s.container.ShellHistoryService.Prune(days)
		return nil
	}
}

func (s *ShellHistoryScreen) selected() (services.ShellHistoryRecord, bool) {
	if s.cursor < 0 || s.cursor >= len(s.records) {
		return services.ShellHistoryRecord{}, false
	}
	return s.records[s.cursor], true
}

func (s *ShellHistoryScreen) moveCursor(delta int) {
	if len(s.records) == 0 {
		s.cursor = 0
		return
	}
	s.cursor += delta
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor >= len(s.records) {
		s.cursor = len(s.records) - 1
	}
	s.updateDetailContent()
}

// applyFilter compiles the query and, when it is valid, swaps in the new
// filter and reloads. An uncompilable query leaves the current results in
// place and only flags the input, so a half-typed pattern like "foo(" does not
// make the list flap back to unfiltered.
func (s *ShellHistoryScreen) applyFilter(query string) tea.Cmd {
	pattern := ""
	if query != "" {
		re, err := regexp.Compile("(?i)" + query)
		if err != nil {
			s.filterInvalid = true
			return nil
		}
		pattern = "(?i)" + query
		s.filterRe = re
	} else {
		s.filterRe = nil
	}

	wasInvalid := s.filterInvalid
	s.filterInvalid = false
	if pattern == s.filter && !wasInvalid {
		return nil
	}
	s.filter = pattern
	return s.loadHistory()
}

func (s *ShellHistoryScreen) exitSearch() tea.Cmd {
	s.searchMode = false
	s.filterInput.Blur()
	s.filterInvalid = false
	if s.filter == "" {
		s.filterRe = nil
		return nil
	}
	s.filter = ""
	s.filterRe = nil
	return s.loadHistory()
}

func (s *ShellHistoryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		_, vpH := s.calcHeights(s.height)
		if !s.detailReady {
			s.detailVP = viewport.New(s.width-4, max(1, vpH))
			s.detailReady = true
		} else {
			s.detailVP.Width = s.width - 4
			s.detailVP.Height = max(1, vpH)
		}
		s.updateDetailContent()
		return s, nil

	case shellHistoryLoadedMsg:
		s.records = msg.records
		s.ready = true
		s.cursor = 0
		s.updateDetailContent()
		return s, nil

	case ErrorMsg:
		s.err = error(msg)
		s.ready = true
		return s, nil

	case tea.KeyMsg:
		return s.handleKey(msg)
	}

	var cmd tea.Cmd
	s.detailVP, cmd = s.detailVP.Update(msg)
	return s, cmd
}

func (s *ShellHistoryScreen) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, s.exitSearch()

	case "tab":
		s.filterInput.Blur()
		return s, nil

	case "enter":
		s.filterInput.Blur()
		return s, nil

	case "down", "ctrl+n":
		s.moveCursor(1)
		return s, nil

	case "up", "ctrl+p":
		s.moveCursor(-1)
		return s, nil
	}

	before := s.filterInput.Value()
	var cmd tea.Cmd
	s.filterInput, cmd = s.filterInput.Update(msg)
	if s.filterInput.Value() != before {
		return s, tea.Batch(cmd, s.applyFilter(s.filterInput.Value()))
	}
	return s, cmd
}

func (s *ShellHistoryScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s.filterInput.Focused() {
		return s.handleSearchInput(msg)
	}

	if s.searchMode && msg.String() == "esc" {
		return s, s.exitSearch()
	}

	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return s, func() tea.Msg { return NavigateBackMsg{} }

	case "/":
		s.searchMode = true
		s.filterInput.SetValue("")
		s.filterInput.Focus()
		return s, s.applyFilter("")

	case "\\":
		s.searchMode = true
		s.filterInput.Focus()
		return s, s.applyFilter(s.filterInput.Value())

	case "f":
		s.cwdOnly = !s.cwdOnly
		return s, s.loadHistory()

	case "F":
		s.failuresOnly = !s.failuresOnly
		return s, s.loadHistory()

	case "j", "down":
		s.moveCursor(1)
		return s, nil

	case "k", "up":
		s.moveCursor(-1)
		return s, nil

	case "g":
		s.cursor = 0
		s.updateDetailContent()
		return s, nil

	case "G":
		s.cursor = max(0, len(s.records)-1)
		s.updateDetailContent()
		return s, nil

	case "ctrl+d", "pgdown":
		s.moveCursor(s.visibleRows() / 2)
		return s, nil

	case "ctrl+u", "pgup":
		s.moveCursor(-s.visibleRows() / 2)
		return s, nil

	case "enter", "x":
		record, ok := s.selected()
		if !ok {
			return s, nil
		}
		return s, s.reExecute(record)

	case "X":
		record, ok := s.selected()
		if !ok {
			return s, nil
		}
		return s, s.showExecutionForm(record)

	case "a":
		record, ok := s.selected()
		if !ok {
			return s, nil
		}
		s.pendingSaveID = record.ID
		return s, s.saveAsScript(record)

	case "d":
		record, ok := s.selected()
		if !ok {
			return s, nil
		}
		return s, s.deleteRecord(record)

	default:
		var cmd tea.Cmd
		s.detailVP, cmd = s.detailVP.Update(msg)
		return s, cmd
	}
}

func (s *ShellHistoryScreen) saveAsScript(record services.ShellHistoryRecord) tea.Cmd {
	return func() tea.Msg {
		scope := record.WorkingDirectory
		if scope == "" {
			scope = s.cwd
		}
		return ShowScriptEditorMsg{
			script:         &entities.Script{Scope: scope},
			initialCommand: record.Command,
			isNewScript:    true,
		}
	}
}

func (s *ShellHistoryScreen) reExecute(record services.ShellHistoryRecord) tea.Cmd {
	return func() tea.Msg {
		return ExecuteAppCommandMsg{
			command: s.container.TerminalService.PrepareScriptExecution(
				record.Command, "", nil, s.cwd, true),
			historyRecord: &services.ExecutionRecord{ExecutedScript: record.Command, WorkingDirectory: s.cwd},
		}
	}
}

func (s *ShellHistoryScreen) showExecutionForm(record services.ShellHistoryRecord) tea.Cmd {
	return func() tea.Msg {
		return ShowRawCommandExecutionMsg{
			command:    record.Command,
			workingDir: record.WorkingDirectory,
		}
	}
}

func (s *ShellHistoryScreen) deleteRecord(record services.ShellHistoryRecord) tea.Cmd {
	return func() tea.Msg {
		if s.container.ShellHistoryService != nil {
			if err := s.container.ShellHistoryService.Delete(record.ID); err != nil {
				return ErrorMsg(err)
			}
		}
		return s.loadHistory()()
	}
}

func (s *ShellHistoryScreen) updateDetailContent() {
	if !s.detailReady {
		return
	}
	record, ok := s.selected()
	if !ok {
		s.detailVP.SetContent("")
		return
	}

	base, highlight := highlightStyles(false)

	var sb strings.Builder
	sb.WriteString("Command:\n")
	sb.WriteString(renderHighlighted(record.Command, s.filterRe, base, highlight))
	sb.WriteString("\n\n")

	dir := record.WorkingDirectory
	if dir == "" {
		dir = "(unknown)"
	}
	sb.WriteString(fmt.Sprintf("Working Dir: %s\n", dir))

	style := statusStyle(record)
	if record.ExitCode == nil {
		sb.WriteString("Exit:        " + style.Render("unknown (still running or shell exited)") + "\n")
	} else {
		sb.WriteString("Exit:        " + style.Render(fmt.Sprintf("%s  (%d)", statusCell(record), *record.ExitCode)) + "\n")
	}

	if d := record.Duration(); d > 0 {
		sb.WriteString(fmt.Sprintf("Duration:    %s\n", d))
	}
	if record.Source == services.ShellHistorySourceScripto {
		sb.WriteString("Source:      scripto\n")
	}
	if record.SavedScriptID != "" {
		sb.WriteString(fmt.Sprintf("Saved as:    %s\n", s.savedScriptLabel(record.SavedScriptID)))
	}

	s.detailVP.SetContent(sb.String())
	s.detailVP.GotoTop()
}

func (s *ShellHistoryScreen) savedScriptLabel(scriptID string) string {
	if s.container.ScriptService == nil {
		return scriptID
	}
	scripts, err := s.container.ScriptService.FindAllScopesScriptsWithArchived()
	if err != nil {
		return scriptID
	}
	for _, script := range scripts {
		if script.ID == scriptID && script.Name != "" {
			return script.Name
		}
	}
	return scriptID
}

func (s *ShellHistoryScreen) View() string {
	if !s.ready {
		return LoadingStyle.Render("Loading shell history...")
	}
	if s.err != nil {
		return ErrorStyle.Render(fmt.Sprintf("Error: %v", s.err))
	}

	title := "Shell History"
	var badges []string
	if s.cwdOnly {
		badges = append(badges, "this dir")
	}
	if s.failuresOnly {
		badges = append(badges, "failures")
	}
	if len(badges) > 0 {
		title = fmt.Sprintf("%s  [%s]", title, strings.Join(badges, " • "))
	}
	header := TitleStyle.Render(title)

	if len(s.records) == 0 {
		body := NoScriptsStyle.Render(s.emptyMessage())
		footer := HelpStyle.Render("/: search • \\: resume search • f: this dir • F: failures • q/esc: back")
		if s.searchMode {
			return lipgloss.JoinVertical(lipgloss.Left, header, s.renderList(), body, footer)
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	}

	detailPane := PreviewStyle.Width(s.width - 2).Render(s.detailVP.View())
	footer := HelpStyle.Render("j/k: navigate • enter/x: run in cwd • X: run with options • a: add as script • d: delete • /: search • \\: resume • f: this dir • F: failures • q/esc: back")

	return lipgloss.JoinVertical(lipgloss.Left, header, s.renderList(), detailPane, footer)
}

func (s *ShellHistoryScreen) emptyMessage() string {
	if s.container.ShellHistoryService == nil {
		return "Shell history is unavailable."
	}
	if s.filter != "" || s.cwdOnly || s.failuresOnly {
		return "No commands match the current filters."
	}
	if os.Getenv("SCRIPTO_TRACK_HISTORY") == "" {
		return "No shell history recorded.\n\nTracking is off. Run `scripto install` and opt in,\nor add `export SCRIPTO_TRACK_HISTORY=1` to your ~/.zshrc."
	}
	return "No shell history recorded yet."
}

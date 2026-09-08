package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/services"
)

const (
	shellHistoryPageSize      = 200
	shellHistoryRetentionDays = 90
	shellHistoryStatusOK      = "✓"
	shellHistoryStatusUnknown = "·"
)

type ShellHistoryScreen struct {
	container *services.Container

	records []services.ShellHistoryRecord
	cwd     string

	filter       string
	filterInput  textinput.Model
	filterMode   bool
	cwdOnly      bool
	failuresOnly bool

	pendingSaveID string

	width  int
	height int
	ready  bool
	err    error

	table       table.Model
	detailVP    viewport.Model
	detailReady bool
}

type shellHistoryLoadedMsg struct {
	records []services.ShellHistoryRecord
}

func NewShellHistoryScreen(container *services.Container, width, height int) *ShellHistoryScreen {
	cwd, _ := os.Getwd()

	fi := textinput.New()
	fi.Placeholder = "filter commands"
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

func (s *ShellHistoryScreen) calcHeights(height int) (tableHeight, vpHeight int) {
	available := height - 6
	tableHeight = available / 2
	vpHeight = available - tableHeight - 4
	if vpHeight < 1 {
		vpHeight = 1
	}
	return
}

// statusCell must stay unstyled: bubbles/table clips cells with runewidth,
// which is not ANSI-aware and would cut escape sequences in half.
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

func (s *ShellHistoryScreen) buildTable(records []services.ShellHistoryRecord) table.Model {
	tableH, _ := s.calcHeights(s.height)

	const statusWidth = 5
	const tsWidth = 16
	const dirWidth = 18
	cmdWidth := max(10, s.width-4-statusWidth-tsWidth-dirWidth-8)

	cols := []table.Column{
		{Title: "", Width: statusWidth},
		{Title: "Time", Width: tsWidth},
		{Title: "Dir", Width: dirWidth},
		{Title: "Command", Width: cmdWidth},
	}

	rows := make([]table.Row, len(records))
	for i, r := range records {
		ts := ""
		if r.StartedAt > 0 {
			ts = time.Unix(r.StartedAt, 0).Format("2006-01-02 15:04")
		}
		dir := truncateCell(shellHistoryDirDisplay(r.WorkingDirectory), dirWidth)
		cmd := truncateCell(strings.Join(strings.Fields(r.Command), " "), cmdWidth)
		rows[i] = table.Row{statusCell(r), ts, dir, cmd}
	}

	tableStyle := table.DefaultStyles()
	tableStyle.Header = tableStyle.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		BorderBottom(true).
		Bold(true).
		Foreground(primaryColor)
	tableStyle.Selected = tableStyle.Selected.
		Foreground(selectedTextColor).
		Background(selectedBgColor).
		Bold(true)

	return table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableH),
		table.WithStyles(tableStyle),
	)
}

func truncateCell(value string, width int) string {
	if width <= 1 || len([]rune(value)) <= width {
		return value
	}
	return string([]rune(value)[:width-1]) + "…"
}

func shellHistoryDirDisplay(dir string) string {
	if dir == "" {
		return "-"
	}
	return filepath.Base(dir)
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

		query := services.ShellHistoryQuery{
			Filter:       s.filter,
			FailuresOnly: s.failuresOnly,
			Limit:        shellHistoryPageSize,
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
	cursor := s.table.Cursor()
	if cursor < 0 || cursor >= len(s.records) {
		return services.ShellHistoryRecord{}, false
	}
	return s.records[cursor], true
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
		s.filterInput.Width = max(10, s.width-12)
		if s.ready {
			s.table = s.buildTable(s.records)
		}
		s.updateDetailContent()
		return s, nil

	case shellHistoryLoadedMsg:
		s.records = msg.records
		s.ready = true
		s.table = s.buildTable(s.records)
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

func (s *ShellHistoryScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s.filterMode {
		switch msg.String() {
		case "esc":
			s.filterMode = false
			s.filterInput.Blur()
			s.filterInput.SetValue(s.filter)
			return s, nil
		case "enter":
			s.filterMode = false
			s.filterInput.Blur()
			return s, nil
		}
		var cmd tea.Cmd
		s.filterInput, cmd = s.filterInput.Update(msg)
		if s.filterInput.Value() != s.filter {
			s.filter = s.filterInput.Value()
			return s, tea.Batch(cmd, s.loadHistory())
		}
		return s, cmd
	}

	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return s, func() tea.Msg { return NavigateBackMsg{} }

	case "/":
		s.filterMode = true
		s.filterInput.Focus()
		return s, nil

	case "f":
		s.cwdOnly = !s.cwdOnly
		return s, s.loadHistory()

	case "F":
		s.failuresOnly = !s.failuresOnly
		return s, s.loadHistory()

	case "enter", "s":
		record, ok := s.selected()
		if !ok {
			return s, nil
		}
		s.pendingSaveID = record.ID
		return s, s.saveAsScript(record)

	case "x":
		record, ok := s.selected()
		if !ok {
			return s, nil
		}
		return s, s.reExecute(record)

	case "d":
		record, ok := s.selected()
		if !ok {
			return s, nil
		}
		return s, s.deleteRecord(record)

	default:
		var cmd tea.Cmd
		s.table, cmd = s.table.Update(msg)
		s.updateDetailContent()
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
		command := record.Command
		if record.WorkingDirectory != "" && record.WorkingDirectory != s.cwd {
			command = "cd " + shellQuote(record.WorkingDirectory) + " && " + command
		}
		return ExecuteAppCommandMsg{
			command: s.container.TerminalService.PrepareScriptExecution(
				command, "", nil, record.WorkingDirectory, false),
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

	var sb strings.Builder
	sb.WriteString("Command:\n")
	sb.WriteString(record.Command)
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
	if s.filter != "" {
		badges = append(badges, fmt.Sprintf("/%s", s.filter))
	}
	if len(badges) > 0 {
		title = fmt.Sprintf("%s  [%s]", title, strings.Join(badges, " • "))
	}
	header := TitleStyle.Render(title)

	if s.filterMode {
		header = lipgloss.JoinVertical(lipgloss.Left, header, "  "+s.filterInput.View())
	}

	if len(s.records) == 0 {
		body := NoScriptsStyle.Render(s.emptyMessage())
		footer := HelpStyle.Render("/: filter • f: this dir • F: failures • q/esc: back")
		return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	}

	tablePane := ListStyle.Width(s.width - 2).Render(s.table.View())
	detailPane := PreviewStyle.Width(s.width - 2).Render(s.detailVP.View())
	footer := HelpStyle.Render("j/k: navigate • enter/s: save as script • x: run • d: delete • /: filter • f: this dir • F: failures • q/esc: back")

	return lipgloss.JoinVertical(lipgloss.Left, header, tablePane, detailPane, footer)
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

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/internal/services"
)

const historyPageSize = 50

type ExecutionHistoryScreen struct {
	container   *services.Container
	scriptID    string
	records     []services.ExecutionRecord
	filter      string
	offset      int
	width       int
	height      int
	ready       bool
	err         error
	table       table.Model
	detailVP    viewport.Model
	detailReady bool
}

type executionHistoryLoadedMsg struct {
	records []services.ExecutionRecord
}

func NewExecutionHistoryScreen(container *services.Container, scriptID string, width, height int) *ExecutionHistoryScreen {
	s := &ExecutionHistoryScreen{
		container: container,
		scriptID:  scriptID,
		width:     width,
		height:    height,
	}
	if width > 0 && height > 0 {
		_, vpH := s.calcHeights(height)
		s.detailVP = viewport.New(width-4, max(1, vpH))
		s.detailReady = true
	}
	return s
}

func (s *ExecutionHistoryScreen) calcHeights(height int) (tableHeight, vpHeight int) {
	available := height - 6
	tableHeight = available / 2
	vpHeight = available - tableHeight - 4
	if vpHeight < 1 {
		vpHeight = 1
	}
	return
}

func (s *ExecutionHistoryScreen) buildTable(records []services.ExecutionRecord) table.Model {
	tableH, _ := s.calcHeights(s.height)

	const tsWidth = 16
	const scopeWidth = 16
	nameWidth := max(10, s.width-4-tsWidth-scopeWidth-6)

	cols := []table.Column{
		{Title: "Time", Width: tsWidth},
		{Title: "Scope", Width: scopeWidth},
		{Title: "Name", Width: nameWidth},
	}

	rows := make([]table.Row, len(records))
	for i, r := range records {
		ts := time.Unix(r.ExecutionTimestamp, 0).Format("2006-01-02 15:04")
		scope := scopeDisplay(r.ScriptScope)
		if len(scope) > scopeWidth {
			scope = scope[:scopeWidth-1] + "…"
		}
		name := r.ScriptName
		if len(name) > nameWidth {
			name = name[:max(0, nameWidth-1)] + "…"
		}
		rows[i] = table.Row{ts, scope, name}
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

func (s *ExecutionHistoryScreen) Init() tea.Cmd {
	return s.loadHistory()
}

func (s *ExecutionHistoryScreen) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if s.container.ExecutionHistoryService == nil {
			return executionHistoryLoadedMsg{records: nil}
		}
		var records []services.ExecutionRecord
		var err error
		if s.scriptID != "" {
			records, err = s.container.ExecutionHistoryService.GetScriptHistory(s.scriptID, historyPageSize)
		} else {
			records, err = s.container.ExecutionHistoryService.GetHistory(s.filter, historyPageSize, s.offset)
		}
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to load history: %w", err))
		}
		return executionHistoryLoadedMsg{records: records}
	}
}

func (s *ExecutionHistoryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		if s.ready {
			s.table = s.buildTable(s.records)
		}
		s.updateDetailContent()
		return s, nil

	case executionHistoryLoadedMsg:
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

func (s *ExecutionHistoryScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return s, func() tea.Msg { return NavigateBackMsg{} }

	case "enter":
		cursor := s.table.Cursor()
		if cursor < len(s.records) {
			record := s.records[cursor]
			return s, s.reExecute(record)
		}
		return s, nil

	default:
		var cmd tea.Cmd
		s.table, cmd = s.table.Update(msg)
		s.updateDetailContent()
		return s, cmd
	}
}

func (s *ExecutionHistoryScreen) reExecute(record services.ExecutionRecord) tea.Cmd {
	return func() tea.Msg {
		cwd, _ := os.Getwd()
		newRecord := services.ExecutionRecord{
			ScriptID:               record.ScriptID,
			ExecutedScript:         record.ExecutedScript,
			OriginalScript:         record.OriginalScript,
			PlaceholderValues:      record.PlaceholderValues,
			WorkingDirectory:       cwd,
			ScriptObjectDefinition: record.ScriptObjectDefinition,
		}
		return ExecuteAppCommandMsg{
			command:       s.container.TerminalService.PrepareScriptExecution(record.ExecutedScript),
			historyRecord: &newRecord,
		}
	}
}

func (s *ExecutionHistoryScreen) updateDetailContent() {
	if !s.detailReady || len(s.records) == 0 {
		return
	}
	cursor := s.table.Cursor()
	if cursor >= len(s.records) {
		return
	}
	r := s.records[cursor]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Working Dir: %s\n\n", r.WorkingDirectory))
	sb.WriteString("Command:\n")
	sb.WriteString(r.ExecutedScript)
	if len(r.PlaceholderValues) > 0 {
		sb.WriteString("\n\nPlaceholder Values:\n")
		for k, v := range r.PlaceholderValues {
			sb.WriteString(fmt.Sprintf("  %s = %s\n", k, v))
		}
	}
	s.detailVP.SetContent(sb.String())
	s.detailVP.GotoTop()
}

func scopeDisplay(scope string) string {
	if scope == "" || scope == "global" {
		return "global"
	}
	return filepath.Base(scope)
}

func (s *ExecutionHistoryScreen) View() string {
	if !s.ready {
		return LoadingStyle.Render("Loading history...")
	}
	if s.err != nil {
		return ErrorStyle.Render(fmt.Sprintf("Error: %v", s.err))
	}

	header := TitleStyle.Render("Execution History")
	if s.scriptID != "" {
		header = TitleStyle.Render(fmt.Sprintf("Execution History: %s", s.scriptID))
	}

	if len(s.records) == 0 {
		body := NoScriptsStyle.Render("No execution history found.")
		footer := HelpStyle.Render("q/esc: back")
		return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	}

	tablePane := ListStyle.Width(s.width - 2).Render(s.table.View())
	detailPane := PreviewStyle.Width(s.width - 2).Render(s.detailVP.View())
	footer := HelpStyle.Render("j/k: navigate • enter: re-execute • q/esc: back")

	return lipgloss.JoinVertical(lipgloss.Left, header, tablePane, detailPane, footer)
}

package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

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
	selected    int
	filter      string
	offset      int
	width       int
	height      int
	ready       bool
	err         error
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
		vpWidth := width/2 - 4
		vpHeight := max(1, height-8)
		s.detailVP = viewport.New(vpWidth, vpHeight)
		s.detailReady = true
	}
	return s
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
		vpHeight := max(1, s.height-8)
		vpWidth := s.width/2 - 4
		if !s.detailReady {
			s.detailVP = viewport.New(vpWidth, vpHeight)
			s.detailReady = true
		} else {
			s.detailVP.Width = vpWidth
			s.detailVP.Height = vpHeight
		}
		s.updateDetailContent()
		return s, nil

	case executionHistoryLoadedMsg:
		s.records = msg.records
		s.ready = true
		if s.selected >= len(s.records) {
			s.selected = max(0, len(s.records)-1)
		}
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

	case "j", "down":
		if s.selected < len(s.records)-1 {
			s.selected++
			s.updateDetailContent()
		}
		return s, nil

	case "k", "up":
		if s.selected > 0 {
			s.selected--
			s.updateDetailContent()
		}
		return s, nil

	case "enter":
		if s.selected < len(s.records) {
			record := s.records[s.selected]
			return s, s.reExecute(record)
		}
		return s, nil
	}
	return s, nil
}

func (s *ExecutionHistoryScreen) reExecute(record services.ExecutionRecord) tea.Cmd {
	return func() tea.Msg {
		cwd, _ := os.Getwd()
		newRecord := services.ExecutionRecord{
			ScriptID:              record.ScriptID,
			ExecutedScript:        record.ExecutedScript,
			OriginalScript:        record.OriginalScript,
			PlaceholderValues:     record.PlaceholderValues,
			WorkingDirectory:      cwd,
			ScriptObjectDefinition: record.ScriptObjectDefinition,
		}
		return ExecuteAppCommandMsg{
			command:       s.container.TerminalService.PrepareScriptExecution(record.ExecutedScript),
			historyRecord: &newRecord,
		}
	}
}

func (s *ExecutionHistoryScreen) updateDetailContent() {
	if !s.detailReady || s.selected >= len(s.records) {
		return
	}
	r := s.records[s.selected]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Script ID: %s\n", r.ScriptID))
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

	const tsWidth = 16
	const nameWidth = 16
	halfWidth := s.width / 2
	cmdWidth := max(0, halfWidth-2-tsWidth-nameWidth-4)

	listLines := make([]string, len(s.records))
	for i, r := range s.records {
		ts := time.Unix(r.ExecutionTimestamp, 0).Format("2006-01-02 15:04")
		name := r.ScriptName
		if len(name) > nameWidth {
			name = name[:nameWidth-1] + "…"
		}
		cmd := r.ExecutedScript
		if len(cmd) > cmdWidth {
			cmd = cmd[:max(0, cmdWidth-1)] + "…"
		}
		line := fmt.Sprintf("%-*s  %-*s  %s", tsWidth, ts, nameWidth, name, cmd)
		if i == s.selected {
			listLines[i] = ListItemSelectedStyle.Width(halfWidth - 2).Render(line)
		} else {
			listLines[i] = ListItemStyle.Width(halfWidth - 2).Render(line)
		}
	}

	listContent := strings.Join(listLines, "\n")
	listPane := ListStyle.Width(halfWidth - 2).Height(s.height - 6).Render(listContent)

	detailPane := PreviewStyle.Width(halfWidth - 2).Height(s.height - 6).Render(s.detailVP.View())

	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
	footer := HelpStyle.Render("j/k: navigate • enter: re-execute • q/esc: back")

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

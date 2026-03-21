package tui

import (
	"fmt"
	"log"
	"scripto/internal/services"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HistoryScreen struct {
	table        table.Model
	commands     []string
	active       bool
	width        int
	height       int
	errorMessage string
	container    *services.Container
}

type HistoryResult struct {
	Command   string
	Cancelled bool
}

type historyLoadedMsg struct {
	commands []string
}

func NewHistoryScreen(container *services.Container) *HistoryScreen {
	return &HistoryScreen{
		container: container,
		active:    true,
		width:     80,
		height:    24,
	}
}

func (h *HistoryScreen) buildTable(commands []string) table.Model {
	popupWidth := min(80, h.width-8)
	colWidth := popupWidth - 4
	if colWidth < 10 {
		colWidth = 10
	}

	cols := []table.Column{
		{Title: "Command", Width: colWidth},
	}

	rows := make([]table.Row, len(commands))
	for i, cmd := range commands {
		rows[i] = table.Row{strings.ReplaceAll(cmd, "\n", "↵")}
	}

	tableHeight := len(commands)
	if tableHeight > 20 {
		tableHeight = 20
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
		table.WithHeight(tableHeight+1),
		table.WithStyles(tableStyle),
	)
}

func (h *HistoryScreen) Init() tea.Cmd {
	return tea.Batch(h.loadHistory(), tea.EnterAltScreen)
}

func (h *HistoryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !h.active {
		return h, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.height = msg.Height
		if len(h.commands) > 0 {
			h.table = h.buildTable(h.commands)
		}
		return h, nil

	case historyLoadedMsg:
		if len(msg.commands) == 0 {
			h.active = false
			return h, func() tea.Msg {
				return NavigateBackMsg{}
			}
		}
		h.commands = msg.commands
		h.table = h.buildTable(msg.commands)
		return h, nil

	case tea.KeyMsg:
		return h.handleKeyPress(msg)
	}

	var cmd tea.Cmd
	h.table, cmd = h.table.Update(msg)
	return h, cmd
}

func (h *HistoryScreen) View() string {
	if !h.active {
		return ""
	}

	popupWidth := min(80, h.width-8)
	popupHeight := min(30, h.height-8)

	var content string

	if h.errorMessage != "" {
		errorText := ErrorStyle.Render(fmt.Sprintf("Error: %s", h.errorMessage))
		content = errorText + "\n\nPress any key to continue with empty command..."
	} else {
		content = h.table.View()
		helpText := HelpStyle.Render("↵: select • s: skip • esc: cancel")
		content += "\n\n" + helpText
	}

	return PopupStyle.
		UnsetBackground().
		Width(popupWidth).
		Height(popupHeight).
		Render(content)
}

func (h *HistoryScreen) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		h.active = false
		return h, func() tea.Msg {
			return NavigateBackMsg{}
		}

	case "enter":
		if row := h.table.SelectedRow(); row != nil {
			cursor := h.table.Cursor()
			if cursor < len(h.commands) {
				command := h.commands[cursor]
				h.active = false
				return h, func() tea.Msg {
					return HistoryCommandSelectedMsg{command: command}
				}
			}
		}
		return h, nil

	case "s":
		h.active = false
		return h, func() tea.Msg {
			return HistoryCommandSelectedMsg{command: ""}
		}

	default:
		var cmd tea.Cmd
		h.table, cmd = h.table.Update(msg)
		return h, cmd
	}
}

func (h *HistoryScreen) loadHistory() tea.Cmd {
	return func() tea.Msg {
		allCommands := h.container.HistoryService.GetHistoryCommands()
		log.Printf("loadHistory: total commands from service: %d", len(allCommands))

		seen := make(map[string]bool)
		commands := make([]string, 0, len(allCommands))
		for _, command := range allCommands {
			command = strings.TrimSpace(command)
			if strings.HasPrefix(command, "scripto") {
				log.Printf("loadHistory: filtering out: %q", command)
			} else if seen[command] {
				log.Printf("loadHistory: deduping: %q", command)
			} else {
				seen[command] = true
				log.Printf("loadHistory: keeping: %q", command)
				commands = append(commands, command)
			}
		}

		log.Printf("loadHistory: commands after filter: %d", len(commands))
		return historyLoadedMsg{commands: commands}
	}
}

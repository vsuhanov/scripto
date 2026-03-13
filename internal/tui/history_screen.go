package tui

import (
	"fmt"
	"io"
	"scripto/internal/services"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type commandItem struct {
	command string
}

func (i commandItem) FilterValue() string { return i.command }

func (i commandItem) Title() string {
	return strings.ReplaceAll(i.command, "\n", "↵")
}

func (i commandItem) Description() string { return "" }

type historyListItemCustomDelegate struct{}

func (d historyListItemCustomDelegate) Height() int                               { return 1 }
func (d historyListItemCustomDelegate) Spacing() int                              { return 0 }
func (d historyListItemCustomDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d historyListItemCustomDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(commandItem)
	if !ok {
		return
	}

	command := i.Title()
	if len(command) > m.Width()-4 {
		command = command[:m.Width()-7] + "..."
	}

	style := HistoryItemStyle
	if index == m.Index() {
		style = HistoryItemSelectedStyle
	}

	fmt.Fprint(w, style.Render(command))
}

type HistoryScreen struct {
	list         list.Model
	active       bool
	width        int
	height       int
	errorMessage string
	container     *services.Container
}

type HistoryResult struct {
	Command   string
	Cancelled bool
}

type historyLoadedMsg struct {
	items []list.Item
}

func NewHistoryScreen(container *services.Container) *HistoryScreen {
	return &HistoryScreen{
		container: container,
		active:   true,
		width:    80,
		height:   24,
	}
}

func (h *HistoryScreen) Init() tea.Cmd { // tea.Model
	delegate := historyListItemCustomDelegate{}
	h.list = list.New([]list.Item{}, delegate, h.width-4, h.height-8)
	h.list.Title = "Select Command from History"
	h.list.SetShowStatusBar(false)
	h.list.SetFilteringEnabled(true)

	return tea.Batch(
		h.loadHistory(),
		tea.EnterAltScreen,
	)
}

func (h *HistoryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) { // tea.Model
	if !h.active {
		return h, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.height = msg.Height
		h.list.SetWidth(msg.Width - 4)
		h.list.SetHeight(msg.Height - 8)
		return h, nil

	case historyLoadedMsg:
		if len(msg.items) == 0 {
			h.active = false
			return h, func() tea.Msg {
				return NavigateBackMsg{}
			}
		}
		h.list.SetItems(msg.items)
		return h, nil

	case tea.KeyMsg:
		return h.handleKeyPress(msg)
	}

	var cmd tea.Cmd
	h.list, cmd = h.list.Update(msg)
	return h, cmd
}

func (h *HistoryScreen) View() string { // tea.Model
	if !h.active {
		return ""
	}

	popupWidth := min(80, h.width-8)
	popupHeight := min(30, h.height-4)

	var content string

	if h.errorMessage != "" {
		errorText := ErrorStyle.Render(fmt.Sprintf("Error: %s", h.errorMessage))
		content = errorText + "\n\nPress any key to continue with empty command..."
	} else {
		content = h.list.View()

		helpText := HelpStyle.Render("↵: select • s: skip • esc: cancel")
		content += "\n\n" + helpText
	}

	return PopupStyle.
		Width(popupWidth).
		Height(popupHeight).
		Render(content)
}

func (h *HistoryScreen) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		h.active = false
		return h, func() tea.Msg {
			return NavigateBackMsg{}
		}

	case "enter":
		if selectedItem := h.list.SelectedItem(); selectedItem != nil {
			if item, ok := selectedItem.(commandItem); ok {
				h.active = false
				return h, func() tea.Msg {
					return HistoryCommandSelectedMsg{command: item.command}
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
		h.list, cmd = h.list.Update(msg)
		return h, cmd
	}
}

func (h *HistoryScreen) loadHistory() tea.Cmd {
	return func() tea.Msg {
		commands := h.container.HistoryService.GetHistoryCommands()

		items := make([]list.Item, len(commands))
		for i, command := range commands {
			items[i] = commandItem{command: command}
		}

		return historyLoadedMsg{items: items}
	}
}

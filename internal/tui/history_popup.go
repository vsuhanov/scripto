package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// commandItem represents a command history item for the list
type commandItem struct {
	command string
}

// FilterValue returns the string used for filtering
func (i commandItem) FilterValue() string { return i.command }

// Title returns the title of the command
func (i commandItem) Title() string {
	// Replace newlines with ↵ for display
	return strings.ReplaceAll(i.command, "\n", "↵")
}

// Description returns the description (empty for commands)
func (i commandItem) Description() string { return "" }

// customDelegate provides a compact, single-line display for commands
type customDelegate struct{}

func (d customDelegate) Height() int                               { return 1 }
func (d customDelegate) Spacing() int                              { return 0 }
func (d customDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d customDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(commandItem)
	if !ok {
		return
	}

	// Get the command and truncate if needed
	command := i.Title()
	if len(command) > m.Width()-4 {
		command = command[:m.Width()-7] + "..."
	}

	// Style based on selection
	style := lipgloss.NewStyle().PaddingLeft(2)
	if index == m.Index() {
		style = style.
			Background(selectedBgColor).
			Foreground(selectedTextColor).
			Bold(true)
	}

	fmt.Fprint(w, style.Render(command))
}

// HistoryPopup represents the command history selection popup
type HistoryPopup struct {
	list         list.Model
	active       bool
	width        int
	height       int
	errorMessage string
	result       HistoryPopupResult
}

// NewHistoryPopup creates a new history popup
func NewHistoryPopup(width, height int) HistoryPopup {
	// Create the list with custom delegate
	delegate := customDelegate{}
	l := list.New([]list.Item{}, delegate, width-4, height-8)
	l.Title = "Select Command from History"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return HistoryPopup{
		list:   l,
		active: true,
		width:  width,
		height: height,
	}
}

// LoadHistory loads command history from file provided by shell wrapper
func (h HistoryPopup) LoadHistory() (HistoryPopup, tea.Cmd) {
	return h, func() tea.Msg {
		// Check if shell history file path is provided
		historyFilePath := os.Getenv("SCRIPTO_SHELL_HISTORY_FILE_PATH")
		if historyFilePath == "" {
			return HistoryLoadedMsg{
				commands: []string{},
				error:    "No shell history file provided",
			}
		}

		// Try to read the history file
		content, err := os.ReadFile(historyFilePath)
		if err != nil {
			return HistoryLoadedMsg{
				commands: []string{},
				error:    "Failed to read shell history file",
			}
		}

		// Parse fc output (same format as before)
		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		var commands []string

		for _, line := range lines {
			// Skip empty lines
			if strings.TrimSpace(line) == "" {
				continue
			}

			// fc output format: "  123  command here"
			// We need to strip the line number and leading spaces
			parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
			if len(parts) >= 2 {
				command := parts[1]
				// Replace \\n with actual newlines for multiline commands
				command = strings.ReplaceAll(command, "\\n", "\n")
				commands = append(commands, command)
			}
		}

		// Reverse to show most recent first
		for i := len(commands)/2 - 1; i >= 0; i-- {
			opp := len(commands) - 1 - i
			commands[i], commands[opp] = commands[opp], commands[i]
		}

		// Convert to list items
		items := make([]list.Item, len(commands))
		for i, command := range commands {
			items[i] = commandItem{command: command}
		}

		return HistoryLoadedMsg{
			commands: commands,
			items:    items,
			error:    "",
		}
	}
}

// HistoryLoadedMsg represents the result of loading command history
type HistoryLoadedMsg struct {
	commands []string
	items    []list.Item
	error    string
}

// HistorySelectedMsg represents a selected command from history
type HistorySelectedMsg struct {
	command string
}

// HistoryPopupResult represents the result of running the history popup
type HistoryPopupResult struct {
	Command   string
	Cancelled bool
}

// Init initializes the history popup
func (h HistoryPopup) Init() tea.Cmd {
	_, cmd := h.LoadHistory()
	return cmd
}

// Update handles popup events and implements tea.Model
func (h HistoryPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !h.active {
		return h, nil
	}

	switch msg := msg.(type) {
	case HistoryLoadedMsg:
		h.errorMessage = msg.error
		if len(msg.items) == 0 {
			// No commands available, proceed with empty command
			h.active = false
			h.result = HistoryPopupResult{Command: "", Cancelled: false}
			return h, tea.Quit
		}
		// Set the items in the list
		h.list.SetItems(msg.items)
		return h, nil

	case tea.KeyMsg:
		// Handle our custom keys first
		switch msg.String() {
		case "esc":
			return h.handleKeyMsg(msg)
		case "enter":
			return h.handleKeyMsg(msg)
		case "s":
			return h.handleKeyMsg(msg)
		}
		// Let the list handle other keys (navigation, filtering, etc.)
	}

	// Update the list
	var cmd tea.Cmd
	h.list, cmd = h.list.Update(msg)
	return h, cmd
}

// handleKeyMsg handles keyboard input for the popup
func (h HistoryPopup) handleKeyMsg(msg tea.KeyMsg) (HistoryPopup, tea.Cmd) {
	switch msg.String() {
	case "esc":
		h.active = false
		h.result = HistoryPopupResult{Command: "", Cancelled: true}
		return h, tea.Quit

	case "enter":
		// Get the selected item from the list
		if selectedItem := h.list.SelectedItem(); selectedItem != nil {
			if cmdItem, ok := selectedItem.(commandItem); ok {
				h.active = false
				h.result = HistoryPopupResult{Command: cmdItem.command, Cancelled: false}
				return h, tea.Quit
			}
		}
		return h, nil

	case "s":
		// Skip history and proceed to add screen with empty command
		h.active = false
		h.result = HistoryPopupResult{Command: "", Cancelled: false}
		return h, tea.Quit
	}

	return h, nil
}

// View renders the history popup
func (h HistoryPopup) View() string {
	if !h.active {
		return ""
	}

	// Calculate popup dimensions
	popupWidth := min(100, h.width-8)
	popupHeight := min(22, h.height-4) // Increased height to accommodate help text

	var content string

	// Show error message if any
	if h.errorMessage != "" {
		errorText := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("Error: %s", h.errorMessage))
		content = errorText + "\n\nPress any key to continue with empty command..."
	} else if h.list.Items() == nil || len(h.list.Items()) == 0 {
		content = "Loading command history..."
	} else {
		// Update list size
		h.list.SetSize(popupWidth-4, popupHeight-8) // Leave room for help text
		listContent := h.list.View()
		
		// Add help text
		helpText := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render("Enter: select • s: skip history • Esc: cancel")
		
		content = listContent + "\n\n" + helpText
	}

	return PopupStyle.
		Width(popupWidth).
		Height(popupHeight).
		Render(content)
}

// GetResult returns the final result of the history popup
func (h HistoryPopup) GetResult() HistoryPopupResult {
	return h.result
}

// RunHistoryPopup runs the history popup TUI and returns the result
func RunHistoryPopup() (HistoryPopupResult, error) {
	// Get terminal size for proper sizing
	popup := NewHistoryPopup(80, 24)
	
	program := tea.NewProgram(popup, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return HistoryPopupResult{Cancelled: true}, fmt.Errorf("TUI error: %w", err)
	}

	// Extract result from final model
	if popup, ok := finalModel.(HistoryPopup); ok {
		return popup.GetResult(), nil
	}

	return HistoryPopupResult{Cancelled: true}, nil
}

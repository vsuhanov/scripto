package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
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
	style := HistoryItemStyle
	if index == m.Index() {
		style = HistoryItemSelectedStyle
	}

	fmt.Fprint(w, style.Render(command))
}

// HistoryScreen represents the embeddable history selection screen
type HistoryScreen struct {
	list         list.Model
	active       bool
	width        int
	height       int
	errorMessage string

	// Screen interface state
	result     ScreenResult
	isComplete bool
}

// HistoryResult represents the specific result of history selection
type HistoryResult struct {
	Command   string
	Cancelled bool
}

// historyLoadedMsg contains the loaded history items
type historyLoadedMsg struct {
	items []list.Item
}

// NewHistoryScreen creates a new history screen
func NewHistoryScreen() *HistoryScreen {
	return &HistoryScreen{
		active: true,
		width:  80,
		height: 24,
	}
}

// SetServices implements Screen interface
func (h *HistoryScreen) SetServices(services interface{}) {
	// History screen doesn't need services
}

// GetResult implements Screen interface
func (h *HistoryScreen) GetResult() ScreenResult {
	return h.result
}

// IsComplete implements Screen interface
func (h *HistoryScreen) IsComplete() bool {
	return h.isComplete
}

// GetHistoryResult returns the history-specific result
func (h *HistoryScreen) GetHistoryResult() HistoryResult {
	if h.result.Action == ActionSelectFromHistory {
		if actionData := ExtractActionData(h.result); actionData != nil {
			return HistoryResult{
				Command:   actionData.Command,
				Cancelled: false,
			}
		}
	}
	return HistoryResult{Cancelled: true}
}

// Init initializes the history screen
func (h *HistoryScreen) Init() tea.Cmd {
	// Create the list with custom delegate
	delegate := customDelegate{}
	h.list = list.New([]list.Item{}, delegate, h.width-4, h.height-8)
	h.list.Title = "Select Command from History"
	h.list.SetShowStatusBar(false)
	h.list.SetFilteringEnabled(true)
	
	return tea.Batch(
		h.loadHistory(),
		tea.EnterAltScreen,
	)
}

// Update handles events for the history screen
func (h *HistoryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			// No commands available, proceed with empty command
			h.result = ScreenResult{
				Action: ActionSelectFromHistory,
				Data:   NewActionDataWithCommand(""),
			}
			h.isComplete = true
			h.active = false
			return h, tea.Quit
		}
		// Set the items in the list
		h.list.SetItems(msg.items)
		return h, nil

	case tea.KeyMsg:
		return h.handleKeyPress(msg)
	}

	// Update the list
	var cmd tea.Cmd
	h.list, cmd = h.list.Update(msg)
	return h, cmd
}

// handleKeyPress handles keyboard input
func (h *HistoryScreen) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		h.result = ScreenResult{
			Action:     ActionNavigateBack,
			ShouldExit: true,
			ExitCode:   3,
		}
		h.isComplete = true
		h.active = false
		return h, tea.Quit

	case "enter":
		// Get the selected item from the list
		if selectedItem := h.list.SelectedItem(); selectedItem != nil {
			if cmdItem, ok := selectedItem.(commandItem); ok {
				h.result = ScreenResult{
					Action: ActionSelectFromHistory,
					Data:   NewActionDataWithCommand(cmdItem.command),
				}
				h.isComplete = true
				h.active = false
				return h, tea.Quit
			}
		}
		return h, nil

	case "s":
		// Skip history and proceed to add screen with empty command
		h.result = ScreenResult{
			Action: ActionSelectFromHistory,
			Data:   NewActionDataWithCommand(""),
		}
		h.isComplete = true
		h.active = false
		return h, tea.Quit

	default:
		// Pass other keys to the list
		var cmd tea.Cmd
		h.list, cmd = h.list.Update(msg)
		return h, cmd
	}
}

// View renders the history screen
func (h *HistoryScreen) View() string {
	if !h.active {
		return ""
	}

	// Calculate popup dimensions
	popupWidth := min(80, h.width-8)
	popupHeight := min(30, h.height-4)

	var content string

	if h.errorMessage != "" {
		errorText := ErrorStyle.Render(fmt.Sprintf("Error: %s", h.errorMessage))
		content = errorText + "\n\nPress any key to continue with empty command..."
	} else {
		// Show the list
		content = h.list.View()
		
		// Add help text
		helpText := HelpStyle.Render("↵: select • s: skip • esc: cancel")
		content += "\n\n" + helpText
	}

	return PopupStyle.
		Width(popupWidth).
		Height(popupHeight).
		Render(content)
}

// loadHistory loads command history from shell wrapper file
func (h *HistoryScreen) loadHistory() tea.Cmd {
	return func() tea.Msg {
		// Check if shell history file path is provided
		historyFilePath := os.Getenv("SCRIPTO_SHELL_HISTORY_FILE_PATH")
		if historyFilePath == "" {
			return historyLoadedMsg{items: []list.Item{}}
		}

		// Try to read the history file
		content, err := os.ReadFile(historyFilePath)
		if err != nil {
			return historyLoadedMsg{items: []list.Item{}}
		}

		// Parse fc output (same format as the removed popup)
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

		return historyLoadedMsg{items: items}
	}
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RunHistoryScreen runs the history screen as a standalone TUI (for backward compatibility)
func RunHistoryScreen() (HistoryResult, error) {
	screen := NewHistoryScreen()
	program := tea.NewProgram(screen, tea.WithAltScreen())
	
	finalModel, err := program.Run()
	if err != nil {
		return HistoryResult{Cancelled: true}, fmt.Errorf("TUI error: %w", err)
	}
	
	if historyScreen, ok := finalModel.(*HistoryScreen); ok {
		return historyScreen.GetHistoryResult(), nil
	}
	
	return HistoryResult{Cancelled: true}, nil
}
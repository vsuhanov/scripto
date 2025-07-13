package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/internal/script"
	"scripto/internal/storage"
)

// EditPopup represents the edit form state
type EditPopup struct {
	// Form fields
	name        string
	description string
	command     string
	isGlobal    bool

	// Form state
	focusedField int // 0=name, 1=description, 2=command, 3=global, 4=save, 5=cancel
	active       bool
	saving       bool // Prevent multiple saves

	// Original script data
	originalScript script.MatchResult
	width          int
	height         int
}

// Form field constants
const (
	FieldName = iota
	FieldDescription
	FieldCommand
	FieldGlobal
	FieldSave
	FieldCancel
	FieldCount
)

// NewEditPopup creates a new edit popup
func NewEditPopup(script script.MatchResult, width, height int) EditPopup {
	return EditPopup{
		name:           script.Script.Name,
		description:    script.Script.Description,
		command:        script.Script.Command,
		isGlobal:       script.Scope == "global",
		focusedField:   FieldName,
		active:         true,
		originalScript: script,
		width:          width,
		height:         height,
	}
}

// Update handles popup events
func (e EditPopup) Update(msg tea.Msg) (EditPopup, tea.Cmd) {
	if !e.active {
		return e, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return e.handleKeyMsg(msg)
	}

	return e, nil
}

// handleKeyMsg handles keyboard input for the popup
func (e EditPopup) handleKeyMsg(msg tea.KeyMsg) (EditPopup, tea.Cmd) {
	switch msg.String() {
	case "esc":
		e.active = false
		return e, func() tea.Msg { return StatusMsg("Edit cancelled") }

	case "tab":
		e.focusedField = (e.focusedField + 1) % FieldCount
		return e, nil

	case "shift+tab":
		e.focusedField = (e.focusedField - 1 + FieldCount) % FieldCount
		return e, nil

	case "enter":
		if e.focusedField == FieldSave && !e.saving {
			e.saving = true
			e.active = false // Close popup immediately
			return e, e.saveScript()
		} else if e.focusedField == FieldCancel {
			e.active = false
			return e, nil
		} else if e.focusedField == FieldGlobal {
			e.isGlobal = !e.isGlobal
			return e, nil
		}

	case "space":
		if e.focusedField == FieldGlobal {
			e.isGlobal = !e.isGlobal
			return e, nil
		}

	case "backspace":
		switch e.focusedField {
		case FieldName:
			if len(e.name) > 0 {
				e.name = e.name[:len(e.name)-1]
			}
		case FieldDescription:
			if len(e.description) > 0 {
				e.description = e.description[:len(e.description)-1]
			}
		case FieldCommand:
			if len(e.command) > 0 {
				e.command = e.command[:len(e.command)-1]
			}
		}
		return e, nil

	default:
		// Handle text input
		if len(msg.String()) == 1 {
			char := msg.String()
			switch e.focusedField {
			case FieldName:
				e.name += char
			case FieldDescription:
				e.description += char
			case FieldCommand:
				e.command += char
			}
		}
	}

	return e, nil
}

// saveScript saves the edited script
func (e EditPopup) saveScript() tea.Cmd {
	return func() tea.Msg {
		// Load current config
		configPath, err := storage.GetConfigPath()
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to get config path: %w", err))
		}

		config, err := storage.ReadConfig(configPath)
		if err != nil {
			return ErrorMsg(fmt.Errorf("failed to read config: %w", err))
		}

		// Remove the old script
		oldKey := e.originalScript.Directory
		if oldKey == "global" {
			oldKey = "global"
		}

		scriptRemoved := false
		if scripts, exists := config[oldKey]; exists {
			for i, script := range scripts {
				// Use more precise matching including description
				if script.Name == e.originalScript.Script.Name &&
					script.Command == e.originalScript.Script.Command &&
					script.FilePath == e.originalScript.Script.FilePath &&
					script.Description == e.originalScript.Script.Description {
					config[oldKey] = append(scripts[:i], scripts[i+1:]...)
					if len(config[oldKey]) == 0 {
						delete(config, oldKey)
					}
					scriptRemoved = true
					break
				}
			}
		}

		// Only proceed if we removed the old script (for existing scripts)
		// For new scripts (unnamed), we allow adding without removal
		if !scriptRemoved && e.originalScript.Script.Name != "" {
			return ErrorMsg(fmt.Errorf("could not find original script to update"))
		}

		// Determine new key based on global setting
		var newKey string
		if e.isGlobal {
			newKey = "global"
		} else {
			// Use current directory if not global
			cwd, err := os.Getwd()
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to get current directory: %w", err))
			}
			newKey = cwd
		}

		// Create updated script
		newScript := storage.Script{
			Name:         e.name,
			Command:      e.command,
			Description:  e.description,
			Placeholders: e.originalScript.Script.Placeholders, // Keep existing placeholders
			FilePath:     e.originalScript.Script.FilePath,     // Keep existing file path
		}

		// Check if script with same name already exists in target location
		// This prevents duplicates when changing scope
		if config[newKey] != nil {
			for _, existingScript := range config[newKey] {
				if existingScript.Name == newScript.Name && newScript.Name != "" {
					return ErrorMsg(fmt.Errorf("script with name '%s' already exists in target scope", newScript.Name))
				}
			}
		}

		// Add to config
		if config[newKey] == nil {
			config[newKey] = []storage.Script{}
		}
		config[newKey] = append(config[newKey], newScript)

		// Save config
		if err := storage.WriteConfig(configPath, config); err != nil {
			return ErrorMsg(fmt.Errorf("failed to save config: %w", err))
		}

		// Update script file if it exists
		if newScript.FilePath != "" {
			if err := os.WriteFile(newScript.FilePath, []byte(newScript.Command), 0644); err != nil {
				return ErrorMsg(fmt.Errorf("failed to update script file: %w", err))
			}
		}

		return StatusMsg("Script updated successfully")
	}
}

// View renders the edit popup
func (e EditPopup) View() string {
	if !e.active {
		return ""
	}

	// Calculate popup dimensions
	popupWidth := min(80, e.width-8)
	popupHeight := min(25, e.height-4)

	var sections []string

	// Title
	title := PopupTitleStyle.Width(popupWidth).Render("Edit Script")
	sections = append(sections, title)

	// Name field
	nameLabel := FieldLabelStyle.Render("Name:")
	nameInput := e.renderInput(e.name, e.focusedField == FieldName, popupWidth-2)
	sections = append(sections, nameLabel)
	sections = append(sections, nameInput)

	// Description field
	descLabel := FieldLabelStyle.Render("Description:")
	descInput := e.renderInput(e.description, e.focusedField == FieldDescription, popupWidth-2)
	sections = append(sections, descLabel)
	sections = append(sections, descInput)

	// Command field (textarea)
	cmdLabel := FieldLabelStyle.Render("Command:")
	cmdInput := e.renderTextArea(e.command, e.focusedField == FieldCommand, popupWidth-2, 6)
	sections = append(sections, cmdLabel)
	sections = append(sections, cmdInput)

	// Global checkbox
	globalCheckbox := e.renderCheckbox("Global script", e.isGlobal, e.focusedField == FieldGlobal)
	sections = append(sections, globalCheckbox)

	// Buttons
	buttons := e.renderButtons(popupWidth)
	sections = append(sections, buttons)

	// Help text
	help := HelpStyle.Render("Tab/Shift+Tab: navigate • Enter: save/toggle • Esc: cancel")
	sections = append(sections, help)

	content := strings.Join(sections, "\n")

	return PopupStyle.
		Width(popupWidth).
		Height(popupHeight).
		Render(content)
}

// renderInput renders a single-line input field
func (e EditPopup) renderInput(value string, focused bool, width int) string {
	style := FieldInputStyle
	if focused {
		style = FieldInputFocusedStyle
	}

	// Add cursor if focused
	displayValue := value
	if focused {
		displayValue += "│"
	}

	return style.Width(width).Render(displayValue)
}

// renderTextArea renders a multi-line text area
func (e EditPopup) renderTextArea(value string, focused bool, width, height int) string {
	style := TextAreaStyle
	if focused {
		style = TextAreaFocusedStyle
	}

	// Wrap text to fit width
	lines := strings.Split(value, "\n")
	var wrappedLines []string
	for _, line := range lines {
		if len(line) <= width-4 {
			wrappedLines = append(wrappedLines, line)
		} else {
			// Simple word wrapping
			words := strings.Fields(line)
			currentLine := ""
			for _, word := range words {
				if len(currentLine)+len(word)+1 <= width-4 {
					if currentLine != "" {
						currentLine += " "
					}
					currentLine += word
				} else {
					if currentLine != "" {
						wrappedLines = append(wrappedLines, currentLine)
					}
					currentLine = word
				}
			}
			if currentLine != "" {
				wrappedLines = append(wrappedLines, currentLine)
			}
		}
	}

	// Limit to height
	if len(wrappedLines) > height {
		wrappedLines = wrappedLines[:height]
	}

	content := strings.Join(wrappedLines, "\n")

	// Add cursor if focused
	if focused {
		content += "│"
	}

	return style.Width(width).Height(height).Render(content)
}

// renderCheckbox renders a checkbox
func (e EditPopup) renderCheckbox(label string, checked bool, focused bool) string {
	checkbox := "☐"
	style := CheckboxStyle
	if checked {
		checkbox = "☑"
		style = CheckboxCheckedStyle
	}

	if focused {
		style = style.Background(selectedBgColor)
	}

	return style.Render(fmt.Sprintf("%s %s", checkbox, label))
}

// renderButtons renders save/cancel buttons
func (e EditPopup) renderButtons(width int) string {
	saveStyle := FieldInputStyle
	cancelStyle := FieldInputStyle

	if e.focusedField == FieldSave {
		saveStyle = FieldInputFocusedStyle
	}
	if e.focusedField == FieldCancel {
		cancelStyle = FieldInputFocusedStyle
	}

	save := saveStyle.Render("Save")
	cancel := cancelStyle.Render("Cancel")

	buttons := lipgloss.JoinHorizontal(lipgloss.Top, save, "  ", cancel)
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(buttons)
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

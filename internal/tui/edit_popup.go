package tui

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"

	"scripto/internal/script"
	"scripto/internal/storage"
)

// EditPopup represents the edit form state
type EditPopup struct {
	// Form fields
	nameInput        textinput.Model
	descriptionInput textinput.Model
	commandTextarea  textarea.Model
	isGlobal         bool

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
	// Calculate component width
	componentWidth := min(70, width-15)

	// Create name input
	nameInput := textinput.New()
	nameInput.Placeholder = "Script name"
	nameInput.SetValue(script.Script.Name)
	nameInput.CharLimit = 100
	nameInput.Width = componentWidth
	nameInput.Focus()

	// Create description input
	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "Script description"
	descriptionInput.SetValue(script.Script.Description)
	descriptionInput.CharLimit = 200
	descriptionInput.Width = componentWidth

	// Create command textarea
	commandTextarea := textarea.New()
	commandTextarea.Placeholder = "Enter command here..."
	commandTextarea.SetValue(script.Script.Command)
	commandTextarea.SetWidth(componentWidth)
	commandTextarea.SetHeight(6)

	return EditPopup{
		nameInput:        nameInput,
		descriptionInput: descriptionInput,
		commandTextarea:  commandTextarea,
		isGlobal:         script.Scope == "global",
		focusedField:     FieldName,
		active:           true,
		originalScript:   script,
		width:            width,
		height:           height,
	}
}

// updateComponentSizes updates the sizes of all components when popup is resized
func (e EditPopup) updateComponentSizes() {
	componentWidth := min(70, e.width-15)

	e.nameInput.Width = componentWidth
	e.descriptionInput.Width = componentWidth
	e.commandTextarea.SetWidth(componentWidth)
}

// Update handles popup events
func (e EditPopup) Update(msg tea.Msg) (EditPopup, tea.Cmd) {
	if !e.active {
		return e, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle navigation keys first
		switch msg.String() {
		case "tab", "shift+tab", "esc":
			return e.handleKeyMsg(msg)
		case "enter":
			// Only handle enter for non-input fields
			if e.focusedField == FieldSave || e.focusedField == FieldCancel || e.focusedField == FieldGlobal {
				return e.handleKeyMsg(msg)
			}
			// For input fields, let the component handle enter
		case "space":
			// Only handle space for checkbox
			if e.focusedField == FieldGlobal {
				return e.handleKeyMsg(msg)
			}
			// For input fields, let the component handle space
		}
	}

	// Update the focused component
	var cmd tea.Cmd
	switch e.focusedField {
	case FieldName:
		e.nameInput, cmd = e.nameInput.Update(msg)
	case FieldDescription:
		e.descriptionInput, cmd = e.descriptionInput.Update(msg)
	case FieldCommand:
		e.commandTextarea, cmd = e.commandTextarea.Update(msg)
	}

	return e, cmd
}

// handleKeyMsg handles keyboard input for the popup
func (e EditPopup) handleKeyMsg(msg tea.KeyMsg) (EditPopup, tea.Cmd) {
	switch msg.String() {
	case "esc":
		e.active = false
		return e, func() tea.Msg { return NavigateBackMsg{} }

	case "tab":
		e.focusedField = (e.focusedField + 1) % FieldCount
		(&e).updateFocus()
		return e, nil

	case "shift+tab":
		e.focusedField = (e.focusedField - 1 + FieldCount) % FieldCount
		(&e).updateFocus()
		return e, nil

	case "enter":
		// Only handle enter for non-input fields
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
		// For input fields, let the component handle enter

	case "space":
		if e.focusedField == FieldGlobal {
			e.isGlobal = !e.isGlobal
			return e, nil
		}
		// For input fields, let the component handle space
	}

	return e, nil
}

// updateFocus updates the focus state of all components
func (e *EditPopup) updateFocus() {
	e.nameInput.Blur()
	e.descriptionInput.Blur()
	e.commandTextarea.Blur()

	switch e.focusedField {
	case FieldName:
		e.nameInput.Focus()
	case FieldDescription:
		e.descriptionInput.Focus()
	case FieldCommand:
		e.commandTextarea.Focus()
	}
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

		// Check if this is a new script (empty original script) or an existing one
		isNewScript := e.originalScript.Script.Name == "" && e.originalScript.Script.Command == "" && e.originalScript.Script.FilePath == ""

		if !isNewScript {
			// Remove the old script for existing scripts
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
			if !scriptRemoved {
				return ErrorMsg(fmt.Errorf("could not find original script to update"))
			}
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

		// Get values from components
		name := e.nameInput.Value()
		description := e.descriptionInput.Value()
		command := e.commandTextarea.Value()

		// Parse placeholders from the command
		placeholders := ParsePlaceholders(command)

		// Create or update script file
		var filePath string
		if e.originalScript.Script.FilePath != "" {
			filePath = e.originalScript.Script.FilePath
		} else {
			// Create new script file for new scripts
			var err error
			filePath, err = storage.SaveScriptToFile(name, command)
			if err != nil {
				return ErrorMsg(fmt.Errorf("failed to save script to file: %w", err))
			}
		}

		// Create updated script
		newScript := storage.Script{
			Name:         name,
			Command:      command,
			Description:  description,
			Placeholders: placeholders,
			FilePath:     filePath,
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

		// Return appropriate success message
		if isNewScript {
			return StatusMsg("Script added successfully")
		} else {
			return StatusMsg("Script updated successfully")
		}
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

	// Title - show different title for new vs existing scripts
	var titleText string
	if e.originalScript.Script.Name == "" && e.originalScript.Script.Command == "" && e.originalScript.Script.FilePath == "" {
		titleText = "Add New Script"
	} else {
		titleText = "Edit Script"
	}
	title := PopupTitleStyle.Width(popupWidth).Render(titleText)
	sections = append(sections, title)

	// Name field
	nameLabel := FieldLabelStyle.Render("Name:")
	if e.focusedField == FieldName {
		nameLabel = FieldLabelStyle.Foreground(primaryColor).Render("Name:")
	}
	sections = append(sections, nameLabel)
	sections = append(sections, e.nameInput.View())

	// Description field
	descLabel := FieldLabelStyle.Render("Description:")
	if e.focusedField == FieldDescription {
		descLabel = FieldLabelStyle.Foreground(primaryColor).Render("Description:")
	}
	sections = append(sections, descLabel)
	sections = append(sections, e.descriptionInput.View())

	// Command field (textarea)
	cmdLabel := FieldLabelStyle.Render("Command:")
	if e.focusedField == FieldCommand {
		cmdLabel = FieldLabelStyle.Foreground(primaryColor).Render("Command:")
	}
	sections = append(sections, cmdLabel)
	sections = append(sections, e.commandTextarea.View())

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

// ParsePlaceholders extracts placeholders in the format %variable:description% from a command
func ParsePlaceholders(command string) []string {
	re := regexp.MustCompile(`%([^:%]+):[^%]*%`)
	matches := re.FindAllStringSubmatch(command, -1)

	placeholders := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			placeholders = append(placeholders, match[1])
		}
	}

	return placeholders
}

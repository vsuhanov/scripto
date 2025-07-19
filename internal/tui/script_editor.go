package tui

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/entities"
)

// ScriptEditorResult represents the result of the script editor
type ScriptEditorResult struct {
	Script    entities.Script
	Command   string
	Cancelled bool
}

// ScriptEditor represents the unified script editing component
type ScriptEditor struct {
	// Form fields
	nameInput        textinput.Model
	descriptionInput textinput.Model
	commandTextarea  textarea.Model
	scopeInput       textinput.Model

	// Form state
	focusedField int // 0=name, 1=description, 2=command, 3=scope, 4=save, 5=cancel
	active       bool
	saving       bool

	// Original script data
	originalScript entities.Script
	isNewScript    bool
	width          int
	height         int
}

// Form field constants
const (
	EditorFieldName = iota
	EditorFieldDescription
	EditorFieldCommand
	EditorFieldScope
	EditorFieldSave
	EditorFieldCancel
	EditorFieldCount
)

// NewScriptEditor creates a new script editor
func NewScriptEditor(script entities.Script, isNewScript bool, width, height int) ScriptEditor {
	// Calculate component width
	componentWidth := min(70, width-15)

	// Create name input
	nameInput := textinput.New()
	nameInput.Placeholder = "Script name"
	nameInput.SetValue(script.Name)
	nameInput.CharLimit = 100
	nameInput.Width = componentWidth
	nameInput.Focus()

	// Create description input
	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "Script description"
	descriptionInput.SetValue(script.Description)
	descriptionInput.CharLimit = 200
	descriptionInput.Width = componentWidth

	// Create command textarea
	commandTextarea := textarea.New()
	commandTextarea.Placeholder = "Enter command here..."
	// Read command from file if available
	var commandContent string
	if script.FilePath != "" {
		if content, err := os.ReadFile(script.FilePath); err == nil {
			commandContent = strings.TrimSpace(string(content))
		}
	}
	commandTextarea.SetValue(commandContent)
	commandTextarea.SetWidth(componentWidth)
	commandTextarea.SetHeight(6)

	// Create scope input
	scopeInput := textinput.New()
	scopeInput.Placeholder = "Directory path or 'global'"
	scopeInput.SetValue(script.Scope)
	scopeInput.CharLimit = 500
	scopeInput.Width = componentWidth

	// Set default scope for new scripts
	if isNewScript && script.Scope == "" {
		if cwd, err := os.Getwd(); err == nil {
			scopeInput.SetValue(cwd)
		}
	}

	return ScriptEditor{
		nameInput:        nameInput,
		descriptionInput: descriptionInput,
		commandTextarea:  commandTextarea,
		scopeInput:       scopeInput,
		focusedField:     EditorFieldName,
		active:           true,
		originalScript:   script,
		isNewScript:      isNewScript,
		width:            width,
		height:           height,
	}
}

// Update handles script editor events
func (e ScriptEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if e.focusedField == EditorFieldSave || e.focusedField == EditorFieldCancel {
				return e.handleKeyMsg(msg)
			}
		}
	}

	// Update the focused component
	var cmd tea.Cmd
	switch e.focusedField {
	case EditorFieldName:
		e.nameInput, cmd = e.nameInput.Update(msg)
	case EditorFieldDescription:
		e.descriptionInput, cmd = e.descriptionInput.Update(msg)
	case EditorFieldCommand:
		e.commandTextarea, cmd = e.commandTextarea.Update(msg)
	case EditorFieldScope:
		e.scopeInput, cmd = e.scopeInput.Update(msg)
	}

	return e, cmd
}

// handleKeyMsg handles keyboard input for the editor
func (e ScriptEditor) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		e.active = false
		return e, tea.Quit

	case "tab":
		e.focusedField = (e.focusedField + 1) % EditorFieldCount
		(&e).updateFocus()
		return e, nil

	case "shift+tab":
		e.focusedField = (e.focusedField - 1 + EditorFieldCount) % EditorFieldCount
		(&e).updateFocus()
		return e, nil

	case "enter":
		if e.focusedField == EditorFieldSave && !e.saving {
			e.saving = true
			e.active = false
			return e, tea.Quit
		} else if e.focusedField == EditorFieldCancel {
			e.active = false
			return e, tea.Quit
		}
	}

	return e, nil
}

// updateFocus updates the focus state of all components
func (e *ScriptEditor) updateFocus() {
	e.nameInput.Blur()
	e.descriptionInput.Blur()
	e.commandTextarea.Blur()
	e.scopeInput.Blur()

	switch e.focusedField {
	case EditorFieldName:
		e.nameInput.Focus()
	case EditorFieldDescription:
		e.descriptionInput.Focus()
	case EditorFieldCommand:
		e.commandTextarea.Focus()
	case EditorFieldScope:
		e.scopeInput.Focus()
	}
}

// GetResult returns the editor result
func (e ScriptEditor) GetResult() ScriptEditorResult {
	if !e.saving {
		return ScriptEditorResult{Cancelled: true}
	}

	// Get values from components
	name := e.nameInput.Value()
	description := e.descriptionInput.Value()
	command := e.commandTextarea.Value()
	scope := e.scopeInput.Value()

	script := entities.Script{
		Name:        name,
		Description: description,
		FilePath:    e.originalScript.FilePath,
		Scope:       scope,
	}

	return ScriptEditorResult{
		Script:    script,
		Command:   command,
		Cancelled: false,
	}
}

// GetCommand returns the command content from the textarea
func (e ScriptEditor) GetCommand() string {
	return e.commandTextarea.Value()
}

// Init initializes the script editor
func (e ScriptEditor) Init() tea.Cmd {
	return tea.EnterAltScreen
}

// View renders the script editor
func (e ScriptEditor) View() string {
	if !e.active {
		return ""
	}

	// Calculate popup dimensions
	popupWidth := min(80, e.width-8)
	popupHeight := min(30, e.height-4)

	var sections []string

	// Title
	var titleText string
	if e.isNewScript {
		titleText = "Add New Script"
	} else {
		titleText = "Edit Script"
	}
	title := PopupTitleStyle.Width(popupWidth).Render(titleText)
	sections = append(sections, title)

	// Name field
	nameLabel := FieldLabelStyle.Render("Name:")
	if e.focusedField == EditorFieldName {
		nameLabel = FieldLabelStyle.Foreground(primaryColor).Render("Name:")
	}
	sections = append(sections, nameLabel)
	sections = append(sections, e.nameInput.View())

	// Description field
	descLabel := FieldLabelStyle.Render("Description:")
	if e.focusedField == EditorFieldDescription {
		descLabel = FieldLabelStyle.Foreground(primaryColor).Render("Description:")
	}
	sections = append(sections, descLabel)
	sections = append(sections, e.descriptionInput.View())

	// Command field (textarea)
	cmdLabel := FieldLabelStyle.Render("Command:")
	if e.focusedField == EditorFieldCommand {
		cmdLabel = FieldLabelStyle.Foreground(primaryColor).Render("Command:")
	}
	sections = append(sections, cmdLabel)
	sections = append(sections, e.commandTextarea.View())

	// Scope field
	scopeLabel := FieldLabelStyle.Render("Scope (directory path or 'global'):")
	if e.focusedField == EditorFieldScope {
		scopeLabel = FieldLabelStyle.Foreground(primaryColor).Render("Scope (directory path or 'global'):")
	}
	sections = append(sections, scopeLabel)
	sections = append(sections, e.scopeInput.View())

	// Buttons
	buttons := e.renderButtons(popupWidth)
	sections = append(sections, buttons)

	// Help text
	help := HelpStyle.Render("Tab/Shift+Tab: navigate • Enter: save • Esc: cancel")
	sections = append(sections, help)

	content := strings.Join(sections, "\n")

	return PopupStyle.
		Width(popupWidth).
		Height(popupHeight).
		Render(content)
}

// renderButtons renders save/cancel buttons
func (e ScriptEditor) renderButtons(width int) string {
	saveStyle := FieldInputStyle
	cancelStyle := FieldInputStyle

	if e.focusedField == EditorFieldSave {
		saveStyle = FieldInputFocusedStyle
	}
	if e.focusedField == EditorFieldCancel {
		cancelStyle = FieldInputFocusedStyle
	}

	save := saveStyle.Render("Save")
	cancel := cancelStyle.Render("Cancel")

	buttons := lipgloss.JoinHorizontal(lipgloss.Top, save, "  ", cancel)
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(buttons)
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

// RunScriptEditor runs the script editor TUI and returns the result
func RunScriptEditor(script entities.Script, isNewScript bool) (ScriptEditorResult, error) {
	// Get terminal size for proper sizing
	program := tea.NewProgram(NewScriptEditor(script, isNewScript, 80, 24), tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return ScriptEditorResult{Cancelled: true}, fmt.Errorf("TUI error: %w", err)
	}

	// Extract result from final model
	if editor, ok := finalModel.(ScriptEditor); ok {
		return editor.GetResult(), nil
	}

	return ScriptEditorResult{Cancelled: true}, nil
}
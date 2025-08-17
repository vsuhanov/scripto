package tui

import (
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/entities"
	"scripto/internal/services"
)

// ScriptEditorScreen represents the embeddable script editing screen
type ScriptEditorScreen struct {
	// Form fields
	nameInput        textinput.Model
	descriptionInput textinput.Model
	commandTextarea  textarea.Model
	scopeInput       textinput.Model

	// Form state
	focusedField int // 0=name, 1=description, 2=command, 3=scope, 4=save, 5=cancel
	active       bool

	// Original script data
	originalScript entities.Script
	isNewScript    bool
	width          int
	height         int

	// Services
	scriptService *services.ScriptService

	// Screen interface state
	result       ScreenResult
	isComplete   bool
	errorMessage string
}

// Form field constants for ScriptEditorScreen
const (
	EditorScreenFieldName = iota
	EditorScreenFieldDescription
	EditorScreenFieldCommand
	EditorScreenFieldScope
	EditorScreenFieldSave
	EditorScreenFieldCancel
	EditorScreenFieldCount
)


// NewScriptEditorScreen creates a new script editor screen
func NewScriptEditorScreen(script entities.Script, isNewScript bool) *ScriptEditorScreen {
	return &ScriptEditorScreen{
		originalScript: script,
		isNewScript:    isNewScript,
		active:         true,
		width:          80,
		height:         24,
	}
}

// SetServices implements Screen interface
func (e *ScriptEditorScreen) SetServices(svcs interface{}) {
	if scriptService, ok := svcs.(*services.ScriptService); ok {
		e.scriptService = scriptService
	}
}

// GetResult implements Screen interface
func (e *ScriptEditorScreen) GetResult() ScreenResult {
	return e.result
}

// IsComplete implements Screen interface
func (e *ScriptEditorScreen) IsComplete() bool {
	return e.isComplete
}

// GetEditorResult returns the editor-specific result
func (e *ScriptEditorScreen) GetEditorResult() ScriptEditorResult {
	if e.result.Action == ActionScriptEditorCancel {
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

// GetOriginalScript returns the original script
func (e *ScriptEditorScreen) GetOriginalScript() entities.Script {
	return e.originalScript
}

// SetErrorMessage sets an error message to display
func (e *ScriptEditorScreen) SetErrorMessage(msg string) {
	e.errorMessage = msg
}

// Init initializes the script editor screen
func (e *ScriptEditorScreen) Init() tea.Cmd {
	e.initializeComponents()
	return tea.EnterAltScreen
}

// initializeComponents initializes the form components
func (e *ScriptEditorScreen) initializeComponents() {
	// Calculate component width
	componentWidth := min(70, e.width-15)

	// Create name input
	e.nameInput = textinput.New()
	e.nameInput.Placeholder = "Script name"
	e.nameInput.SetValue(e.originalScript.Name)
	e.nameInput.CharLimit = 100
	e.nameInput.Width = componentWidth

	// Create description input
	e.descriptionInput = textinput.New()
	e.descriptionInput.Placeholder = "Script description"
	e.descriptionInput.SetValue(e.originalScript.Description)
	e.descriptionInput.CharLimit = 200
	e.descriptionInput.Width = componentWidth

	// Create command textarea
	e.commandTextarea = textarea.New()
	e.commandTextarea.Placeholder = "Enter command here..."
	
	// Read command from file if available
	var commandContent string
	if e.originalScript.FilePath != "" {
		if content, err := os.ReadFile(e.originalScript.FilePath); err == nil {
			commandContent = strings.TrimSpace(string(content))
		}
	}
	e.commandTextarea.SetValue(commandContent)
	e.commandTextarea.SetWidth(componentWidth)
	e.commandTextarea.SetHeight(6)

	// Create scope input
	e.scopeInput = textinput.New()
	e.scopeInput.Placeholder = "Directory path or 'global'"
	e.scopeInput.SetValue(e.originalScript.Scope)
	e.scopeInput.CharLimit = 500
	e.scopeInput.Width = componentWidth

	// Set default scope for new scripts
	if e.isNewScript && e.originalScript.Scope == "" {
		if cwd, err := os.Getwd(); err == nil {
			e.scopeInput.SetValue(cwd)
		}
	}

	// Set initial focus
	e.focusedField = EditorScreenFieldName
	e.updateFocus()
}

// Update handles script editor events
func (e *ScriptEditorScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !e.active {
		return e, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.width = msg.Width
		e.height = msg.Height
		e.initializeComponents() // Reinitialize with new size
		return e, nil

	case tea.KeyMsg:
		return e.handleKeyMsg(msg)
	}

	// For non-KeyMsg events, update the focused component
	var cmd tea.Cmd
	switch e.focusedField {
	case EditorScreenFieldName:
		e.nameInput, cmd = e.nameInput.Update(msg)
	case EditorScreenFieldDescription:
		e.descriptionInput, cmd = e.descriptionInput.Update(msg)
	case EditorScreenFieldCommand:
		e.commandTextarea, cmd = e.commandTextarea.Update(msg)
	case EditorScreenFieldScope:
		e.scopeInput, cmd = e.scopeInput.Update(msg)
	}

	return e, cmd
}

// handleKeyMsg handles keyboard input for the editor
func (e *ScriptEditorScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		e.result = ScreenResult{
			Action: ActionScriptEditorCancel,
		}
		e.isComplete = true
		e.active = false
		return e, tea.Quit

	case "tab":
		e.focusedField = (e.focusedField + 1) % EditorScreenFieldCount
		e.updateFocus()
		return e, nil

	case "shift+tab":
		e.focusedField = (e.focusedField - 1 + EditorScreenFieldCount) % EditorScreenFieldCount
		e.updateFocus()
		return e, nil

	case "enter":
		if e.focusedField == EditorScreenFieldSave {
			e.result = ScreenResult{
				Action: ActionScriptEditorSave,
			}
			e.isComplete = true
			e.active = false
			return e, tea.Quit
		} else if e.focusedField == EditorScreenFieldCancel {
			e.result = ScreenResult{
				Action: ActionScriptEditorCancel,
			}
			e.isComplete = true
			e.active = false
			return e, tea.Quit
		}
		// For input fields, let them handle enter
		fallthrough

	default:
		// Pass other keys to the focused component
		var cmd tea.Cmd
		switch e.focusedField {
		case EditorScreenFieldName:
			e.nameInput, cmd = e.nameInput.Update(msg)
		case EditorScreenFieldDescription:
			e.descriptionInput, cmd = e.descriptionInput.Update(msg)
		case EditorScreenFieldCommand:
			e.commandTextarea, cmd = e.commandTextarea.Update(msg)
		case EditorScreenFieldScope:
			e.scopeInput, cmd = e.scopeInput.Update(msg)
		}
		return e, cmd
	}
}

// updateFocus updates the focus state of all components
func (e *ScriptEditorScreen) updateFocus() {
	e.nameInput.Blur()
	e.descriptionInput.Blur()
	e.commandTextarea.Blur()
	e.scopeInput.Blur()

	switch e.focusedField {
	case EditorScreenFieldName:
		e.nameInput.Focus()
	case EditorScreenFieldDescription:
		e.descriptionInput.Focus()
	case EditorScreenFieldCommand:
		e.commandTextarea.Focus()
	case EditorScreenFieldScope:
		e.scopeInput.Focus()
	}
}

// View renders the script editor screen
func (e *ScriptEditorScreen) View() string {
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

	// Error message if any
	if e.errorMessage != "" {
		errorMsg := ErrorStyle.Render("Error: " + e.errorMessage)
		sections = append(sections, errorMsg)
	}

	// Name field
	nameLabel := FieldLabelStyle.Render("Name:")
	if e.focusedField == EditorScreenFieldName {
		nameLabel = FieldLabelStyle.Foreground(primaryColor).Render("Name:")
	}
	sections = append(sections, nameLabel)
	sections = append(sections, e.nameInput.View())

	// Description field
	descLabel := FieldLabelStyle.Render("Description:")
	if e.focusedField == EditorScreenFieldDescription {
		descLabel = FieldLabelStyle.Foreground(primaryColor).Render("Description:")
	}
	sections = append(sections, descLabel)
	sections = append(sections, e.descriptionInput.View())

	// Command field (textarea)
	cmdLabel := FieldLabelStyle.Render("Command:")
	if e.focusedField == EditorScreenFieldCommand {
		cmdLabel = FieldLabelStyle.Foreground(primaryColor).Render("Command:")
	}
	sections = append(sections, cmdLabel)
	
	// Apply focused/unfocused styling to textarea
	textareaView := e.commandTextarea.View()
	if e.focusedField == EditorScreenFieldCommand {
		textareaView = TextAreaFocusedStyle.Render(textareaView)
	} else {
		textareaView = TextAreaStyle.Render(textareaView)
	}
	sections = append(sections, textareaView)

	// Scope field
	scopeLabel := FieldLabelStyle.Render("Scope (directory path or 'global'):")
	if e.focusedField == EditorScreenFieldScope {
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
func (e *ScriptEditorScreen) renderButtons(width int) string {
	saveStyle := FieldInputStyle
	cancelStyle := FieldInputStyle

	if e.focusedField == EditorScreenFieldSave {
		saveStyle = FieldInputFocusedStyle
	}
	if e.focusedField == EditorScreenFieldCancel {
		cancelStyle = FieldInputFocusedStyle
	}

	save := saveStyle.Render("Save")
	cancel := cancelStyle.Render("Cancel")

	buttons := lipgloss.JoinHorizontal(lipgloss.Top, save, "  ", cancel)
	return ButtonContainerStyle.Width(width).Render(buttons)
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
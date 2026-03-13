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

type ScriptEditorScreen struct {
	nameInput        textinput.Model
	descriptionInput textinput.Model
	commandTextarea  textarea.Model
	scopeInput       textinput.Model

	focusedField int // 0=name, 1=description, 2=command, 3=scope, 4=save, 5=cancel
	active       bool

	originalScript *entities.Script
	initialCommand string
	isNewScript    bool
	width          int
	height         int

	container *services.Container

	errorMessage string
}

const (
	EditorScreenFieldName = iota
	EditorScreenFieldDescription
	EditorScreenFieldCommand
	EditorScreenFieldScope
	EditorScreenFieldSave
	EditorScreenFieldCancel
	EditorScreenFieldCount
)


func NewScriptEditorScreen(script *entities.Script, isNewScript bool, container *services.Container) *ScriptEditorScreen {
	return &ScriptEditorScreen{
		originalScript: script,
		isNewScript:    isNewScript,
		container:      container,
		active:         true,
		width:          80,
		height:         24,
	}
}

func (e *ScriptEditorScreen) GetEditorValues() (name, description, command, scope string) {
	name = e.nameInput.Value()
	description = e.descriptionInput.Value()
	command = e.commandTextarea.Value()
	scope = e.scopeInput.Value()
	return
}

func (e *ScriptEditorScreen) SetErrorMessage(msg string) {
	e.errorMessage = msg
}

func (e *ScriptEditorScreen) Init() tea.Cmd {
	e.initializeComponents()
	return tea.EnterAltScreen
}

func (e *ScriptEditorScreen) initializeComponents() {
	componentWidth := min(70, e.width-15)

	e.nameInput = textinput.New()
	e.nameInput.Placeholder = "Script name"
	e.nameInput.SetValue(e.originalScript.Name)
	e.nameInput.CharLimit = 100
	e.nameInput.Width = componentWidth

	e.descriptionInput = textinput.New()
	e.descriptionInput.Placeholder = "Script description"
	e.descriptionInput.SetValue(e.originalScript.Description)
	e.descriptionInput.CharLimit = 200
	e.descriptionInput.Width = componentWidth

	e.commandTextarea = textarea.New()
	e.commandTextarea.Placeholder = "Enter command here..."
	
	var commandContent string
	if e.originalScript.FilePath != "" {
		if content, err := os.ReadFile(e.originalScript.FilePath); err == nil {
			commandContent = strings.TrimSpace(string(content))
		}
	} else {
		commandContent = e.initialCommand
	}
	e.commandTextarea.SetValue(commandContent)
	e.commandTextarea.SetWidth(componentWidth)
	e.commandTextarea.SetHeight(6)

	e.scopeInput = textinput.New()
	e.scopeInput.Placeholder = "Directory path or 'global'"
	e.scopeInput.SetValue(e.originalScript.Scope)
	e.scopeInput.CharLimit = 500
	e.scopeInput.Width = componentWidth

	if e.isNewScript && e.originalScript.Scope == "" {
		if cwd, err := os.Getwd(); err == nil {
			e.scopeInput.SetValue(cwd)
		}
	}

	e.focusedField = EditorScreenFieldName
	e.updateFocus()
}

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

	case ErrorMsg:
		e.active = true
		e.errorMessage = msg.Error()
		return e, nil

	case tea.KeyMsg:
		return e.handleKeyMsg(msg)
	}

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

func (e *ScriptEditorScreen) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		e.active = false
		return e, func() tea.Msg {
			return NavigateBackMsg{}
		}

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
			e.active = false
			name, description, command, scope := e.GetEditorValues()
			script := &entities.Script{
				Name:        name,
				Description: description,
				FilePath:    e.originalScript.FilePath,
				Scope:       scope,
			}
			var original *entities.Script
			if !e.isNewScript {
				original = e.originalScript
			}
			return e, func() tea.Msg {
				return SaveScriptMsg{
					script:   script,
					command:  command,
					original: original,
				}
			}
		} else if e.focusedField == EditorScreenFieldCancel {
			e.active = false
			return e, func() tea.Msg {
				return NavigateBackMsg{}
			}
		}
		fallthrough

	default:
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

func (e *ScriptEditorScreen) View() string {
	if !e.active {
		return ""
	}

	popupWidth := min(80, e.width-8)
	popupHeight := min(30, e.height-4)

	var sections []string

	var titleText string
	if e.isNewScript {
		titleText = "Add New Script"
	} else {
		titleText = "Edit Script"
	}
	title := PopupTitleStyle.Width(popupWidth).Render(titleText)
	sections = append(sections, title)

	if e.errorMessage != "" {
		errorMsg := ErrorStyle.Render("Error: " + e.errorMessage)
		sections = append(sections, errorMsg)
	}

	nameLabel := FieldLabelStyle.Render("Name:")
	if e.focusedField == EditorScreenFieldName {
		nameLabel = FieldLabelStyle.Foreground(primaryColor).Render("Name:")
	}
	sections = append(sections, nameLabel)
	sections = append(sections, e.nameInput.View())

	descLabel := FieldLabelStyle.Render("Description:")
	if e.focusedField == EditorScreenFieldDescription {
		descLabel = FieldLabelStyle.Foreground(primaryColor).Render("Description:")
	}
	sections = append(sections, descLabel)
	sections = append(sections, e.descriptionInput.View())

	cmdLabel := FieldLabelStyle.Render("Command:")
	if e.focusedField == EditorScreenFieldCommand {
		cmdLabel = FieldLabelStyle.Foreground(primaryColor).Render("Command:")
	}
	sections = append(sections, cmdLabel)
	
	textareaView := e.commandTextarea.View()
	if e.focusedField == EditorScreenFieldCommand {
		textareaView = TextAreaFocusedStyle.Render(textareaView)
	} else {
		textareaView = TextAreaStyle.Render(textareaView)
	}
	sections = append(sections, textareaView)

	scopeLabel := FieldLabelStyle.Render("Scope (directory path or 'global'):")
	if e.focusedField == EditorScreenFieldScope {
		scopeLabel = FieldLabelStyle.Foreground(primaryColor).Render("Scope (directory path or 'global'):")
	}
	sections = append(sections, scopeLabel)
	sections = append(sections, e.scopeInput.View())

	buttons := e.renderButtons(popupWidth)
	sections = append(sections, buttons)

	help := HelpStyle.Render("Tab/Shift+Tab: navigate • Enter: save • Esc: cancel")
	sections = append(sections, help)

	content := strings.Join(sections, "\n")

	return PopupStyle.
		Width(popupWidth).
		Height(popupHeight).
		Render(content)
}

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

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
	nameInput              textinput.Model
	descriptionInput       textinput.Model
	commandTextarea        textarea.Model
	scopeInput             textinput.Model
	startDemarcatorInput   textinput.Model
	endDemarcatorInput     textinput.Model
	globalCheckbox         bool

	focusedField int
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
	EditorScreenFieldName             = 0
	EditorScreenFieldDescription      = 1
	EditorScreenFieldCommand          = 2
	EditorScreenFieldGlobal           = 3
	EditorScreenFieldScope            = 4
	EditorScreenFieldStartDemarcator  = 5
	EditorScreenFieldEndDemarcator    = 6
	EditorScreenFieldSave             = 7
	EditorScreenFieldCancel           = 8
	EditorScreenFieldCount            = 9
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

func (e *ScriptEditorScreen) GetEditorValues() (name, description, command, scope, startDemarcator, endDemarcator string) {
	name = e.nameInput.Value()
	description = e.descriptionInput.Value()
	command = e.commandTextarea.Value()
	if e.globalCheckbox {
		scope = "global"
	} else {
		scope = e.scopeInput.Value()
	}
	startDemarcator = e.startDemarcatorInput.Value()
	endDemarcator = e.endDemarcatorInput.Value()
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

	plain := lipgloss.NewStyle()
	e.commandTextarea.FocusedStyle.Base = plain
	e.commandTextarea.FocusedStyle.CursorLine = plain
	e.commandTextarea.FocusedStyle.CursorLineNumber = plain.Foreground(mutedTextColor)
	e.commandTextarea.FocusedStyle.LineNumber = plain.Foreground(mutedTextColor)
	e.commandTextarea.FocusedStyle.Prompt = plain.Foreground(primaryColor)
	e.commandTextarea.FocusedStyle.Text = plain
	e.commandTextarea.BlurredStyle.Base = plain
	e.commandTextarea.BlurredStyle.CursorLine = plain
	e.commandTextarea.BlurredStyle.CursorLineNumber = plain.Foreground(mutedTextColor)
	e.commandTextarea.BlurredStyle.LineNumber = plain.Foreground(mutedTextColor)
	e.commandTextarea.BlurredStyle.Prompt = plain.Foreground(mutedTextColor)
	e.commandTextarea.BlurredStyle.Text = plain

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

	e.globalCheckbox = e.originalScript.Scope == "global"

	e.scopeInput = textinput.New()
	e.scopeInput.Placeholder = "Directory path"
	e.scopeInput.CharLimit = 500
	e.scopeInput.Width = componentWidth

	if e.globalCheckbox {
		if cwd, err := os.Getwd(); err == nil {
			e.scopeInput.SetValue(cwd)
		}
	} else {
		e.scopeInput.SetValue(e.originalScript.Scope)
		if e.isNewScript && e.originalScript.Scope == "" {
			if cwd, err := os.Getwd(); err == nil {
				e.scopeInput.SetValue(cwd)
			}
		}
	}

	e.startDemarcatorInput = textinput.New()
	e.startDemarcatorInput.Placeholder = "% (default)"
	e.startDemarcatorInput.SetValue(e.originalScript.PlaceholderStartDemarcator)
	e.startDemarcatorInput.CharLimit = 10
	e.startDemarcatorInput.Width = componentWidth / 2

	e.endDemarcatorInput = textinput.New()
	e.endDemarcatorInput.Placeholder = "% (default)"
	e.endDemarcatorInput.SetValue(e.originalScript.PlaceholderEndDemarcator)
	e.endDemarcatorInput.CharLimit = 10
	e.endDemarcatorInput.Width = componentWidth / 2

	e.focusedField = EditorScreenFieldName
	e.updateFocus()
}

func (e *ScriptEditorScreen) nextField() int {
	next := (e.focusedField + 1) % EditorScreenFieldCount
	if next == EditorScreenFieldScope && e.globalCheckbox {
		next = (next + 1) % EditorScreenFieldCount
	}
	return next
}

func (e *ScriptEditorScreen) prevField() int {
	prev := (e.focusedField - 1 + EditorScreenFieldCount) % EditorScreenFieldCount
	if prev == EditorScreenFieldScope && e.globalCheckbox {
		prev = (prev - 1 + EditorScreenFieldCount) % EditorScreenFieldCount
	}
	return prev
}

func (e *ScriptEditorScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !e.active {
		return e, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.width = msg.Width
		e.height = msg.Height
		e.initializeComponents()
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
	case EditorScreenFieldStartDemarcator:
		e.startDemarcatorInput, cmd = e.startDemarcatorInput.Update(msg)
	case EditorScreenFieldEndDemarcator:
		e.endDemarcatorInput, cmd = e.endDemarcatorInput.Update(msg)
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
		e.focusedField = e.nextField()
		e.updateFocus()
		return e, nil

	case "shift+tab":
		e.focusedField = e.prevField()
		e.updateFocus()
		return e, nil

	case "enter":
		if e.focusedField == EditorScreenFieldSave {
			name, description, command, scope, startDemarcator, endDemarcator := e.GetEditorValues()
			if name == "" {
				e.errorMessage = "Name is required"
				return e, nil
			}
			if scope == "" {
				e.errorMessage = "Scope is required"
				return e, nil
			}
			e.active = false
			script := &entities.Script{
				ID:                         e.originalScript.ID,
				Name:                       name,
				Description:                description,
				FilePath:                   e.originalScript.FilePath,
				Scope:                      scope,
				PlaceholderStartDemarcator: startDemarcator,
				PlaceholderEndDemarcator:   endDemarcator,
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
		} else if e.focusedField == EditorScreenFieldGlobal {
			e.globalCheckbox = !e.globalCheckbox
			e.updateFocus()
			return e, nil
		}
		fallthrough

	case " ":
		if e.focusedField == EditorScreenFieldGlobal {
			e.globalCheckbox = !e.globalCheckbox
			e.updateFocus()
			return e, nil
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
		case EditorScreenFieldStartDemarcator:
			e.startDemarcatorInput, cmd = e.startDemarcatorInput.Update(msg)
		case EditorScreenFieldEndDemarcator:
			e.endDemarcatorInput, cmd = e.endDemarcatorInput.Update(msg)
		}
		return e, cmd
	}
}

func (e *ScriptEditorScreen) updateFocus() {
	e.nameInput.Blur()
	e.descriptionInput.Blur()
	e.commandTextarea.Blur()
	e.scopeInput.Blur()
	e.startDemarcatorInput.Blur()
	e.endDemarcatorInput.Blur()

	switch e.focusedField {
	case EditorScreenFieldName:
		e.nameInput.Focus()
	case EditorScreenFieldDescription:
		e.descriptionInput.Focus()
	case EditorScreenFieldCommand:
		e.commandTextarea.Focus()
	case EditorScreenFieldScope:
		if !e.globalCheckbox {
			e.scopeInput.Focus()
		}
	case EditorScreenFieldStartDemarcator:
		e.startDemarcatorInput.Focus()
	case EditorScreenFieldEndDemarcator:
		e.endDemarcatorInput.Focus()
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

	checkboxLabel := "☐ Global"
	checkboxStyle := FieldLabelStyle
	if e.globalCheckbox {
		checkboxLabel = "☑ Global"
		checkboxStyle = FieldLabelStyle.Foreground(primaryColor)
	}
	if e.focusedField == EditorScreenFieldGlobal {
		checkboxStyle = FieldLabelStyle.Foreground(primaryColor).Bold(true)
	}
	sections = append(sections, checkboxStyle.Render(checkboxLabel))

	if !e.globalCheckbox {
		scopeLabel := FieldLabelStyle.Render("Scope (directory path):")
		if e.focusedField == EditorScreenFieldScope {
			scopeLabel = FieldLabelStyle.Foreground(primaryColor).Render("Scope (directory path):")
		}
		sections = append(sections, scopeLabel)
		sections = append(sections, e.scopeInput.View())
	}

	startLabel := FieldLabelStyle.Render("Placeholder start:")
	if e.focusedField == EditorScreenFieldStartDemarcator {
		startLabel = FieldLabelStyle.Foreground(primaryColor).Render("Placeholder start:")
	}
	sections = append(sections, startLabel)
	sections = append(sections, e.startDemarcatorInput.View())

	endLabel := FieldLabelStyle.Render("Placeholder end:")
	if e.focusedField == EditorScreenFieldEndDemarcator {
		endLabel = FieldLabelStyle.Foreground(primaryColor).Render("Placeholder end:")
	}
	sections = append(sections, endLabel)
	sections = append(sections, e.endDemarcatorInput.View())

	buttons := e.renderButtons(popupWidth)
	sections = append(sections, buttons)

	help := HelpStyle.Render("Tab/Shift+Tab: navigate • Space/Enter: toggle • Esc: cancel")
	sections = append(sections, help)

	content := strings.Join(sections, "\n")

	return lipgloss.NewStyle().
		Padding(1).
		Width(popupWidth).
		Height(popupHeight).
		Render(content)
}

func (e *ScriptEditorScreen) renderButtons(width int) string {
	saveStyle := PrimaryButtonStyle
	cancelStyle := DangerButtonStyle

	if e.focusedField == EditorScreenFieldSave {
		saveStyle = PrimaryButtonFocusedStyle
	}
	if e.focusedField == EditorScreenFieldCancel {
		cancelStyle = DangerButtonFocusedStyle
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

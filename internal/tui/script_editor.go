package tui

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripto/entities"
)

type ScriptEditor struct {
	nameInput        textinput.Model
	descriptionInput textinput.Model
	commandTextarea  textarea.Model
	scopeInput       textinput.Model

	focusedField int // 0=name, 1=description, 2=command, 3=scope, 4=save, 5=cancel
	active       bool
	saving       bool

	originalScript *entities.Script
	isNewScript    bool
	width          int
	height         int
}

const (
	EditorFieldName = iota
	EditorFieldDescription
	EditorFieldCommand
	EditorFieldScope
	EditorFieldSave
	EditorFieldCancel
	EditorFieldCount
)

func NewScriptEditor(script *entities.Script, isNewScript bool, width, height int) ScriptEditor {
	componentWidth := min(70, width-15)

	nameInput := textinput.New()
	nameInput.Placeholder = "Script name"
	nameInput.SetValue(script.Name)
	nameInput.CharLimit = 100
	nameInput.Width = componentWidth
	nameInput.Focus()

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "Script description"
	descriptionInput.SetValue(script.Description)
	descriptionInput.CharLimit = 200
	descriptionInput.Width = componentWidth

	commandTextarea := textarea.New()
	commandTextarea.Placeholder = "Enter command here..."
	var commandContent string
	if script.FilePath != "" {
		if content, err := os.ReadFile(script.FilePath); err == nil {
			commandContent = strings.TrimSpace(string(content))
		}
	}
	commandTextarea.SetValue(commandContent)
	commandTextarea.SetWidth(componentWidth)
	commandTextarea.SetHeight(6)

	scopeInput := textinput.New()
	scopeInput.Placeholder = "Directory path or 'global'"
	scopeInput.SetValue(script.Scope)
	scopeInput.CharLimit = 500
	scopeInput.Width = componentWidth

	if isNewScript && script.Scope == "" {
		if cwd, err := os.Getwd(); err == nil {
			scopeInput.SetValue(cwd)
		}
	}

	editor := ScriptEditor{
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

	(&editor).validateAndUpdateFocus()

	return editor
}

func (e ScriptEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !e.active {
		return e, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Printf("[ScriptEditor] Update received key: %s", msg.String())
		switch msg.String() {
		case "tab", "shift+tab", "esc":
			log.Printf("[ScriptEditor] Delegating to handleKeyMsg: %s", msg.String())
			return (&e).handleKeyMsg(msg)
		case "enter":
			if e.focusedField == EditorFieldSave || e.focusedField == EditorFieldCancel {
				return (&e).handleKeyMsg(msg)
			}
		}

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

func (e *ScriptEditor) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	log.Printf("[ScriptEditor] handleKeyMsg: key=%s, currentFocus=%d, active=%t", msg.String(), e.focusedField, e.active)

	switch msg.String() {
	case "esc":
		log.Printf("[ScriptEditor] ESC pressed, deactivating editor")
		e.active = false
		return e, tea.Quit

	case "tab":
		oldFocus := e.focusedField
		e.focusedField = (e.focusedField + 1) % EditorFieldCount
		log.Printf("[ScriptEditor] TAB pressed, focus: %d -> %d", oldFocus, e.focusedField)
		e.validateAndUpdateFocus()
		return e, nil

	case "shift+tab":
		oldFocus := e.focusedField
		e.focusedField = (e.focusedField - 1 + EditorFieldCount) % EditorFieldCount
		log.Printf("[ScriptEditor] SHIFT+TAB pressed, focus: %d -> %d", oldFocus, e.focusedField)
		e.validateAndUpdateFocus()
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

func (e *ScriptEditor) updateFocus() {
	log.Printf("[ScriptEditor] updateFocus: focusing field %d", e.focusedField)

	e.nameInput.Blur()
	e.descriptionInput.Blur()
	e.commandTextarea.Blur()
	e.scopeInput.Blur()

	switch e.focusedField {
	case EditorFieldName:
		log.Printf("[ScriptEditor] Focusing name input")
		e.nameInput.Focus()
	case EditorFieldDescription:
		log.Printf("[ScriptEditor] Focusing description input")
		e.descriptionInput.Focus()
	case EditorFieldCommand:
		log.Printf("[ScriptEditor] Focusing command textarea")
		e.commandTextarea.Focus()
	case EditorFieldScope:
		log.Printf("[ScriptEditor] Focusing scope input")
		e.scopeInput.Focus()
	default:
		log.Printf("[ScriptEditor] No component focused (field %d)", e.focusedField)
	}
}

func (e *ScriptEditor) validateAndUpdateFocus() {
	oldFocus := e.focusedField

	if e.focusedField < 0 {
		e.focusedField = 0
	} else if e.focusedField >= EditorFieldCount {
		e.focusedField = EditorFieldCount - 1
	}

	log.Printf("[ScriptEditor] validateAndUpdateFocus: %d -> %d (bounds: 0-%d)", oldFocus, e.focusedField, EditorFieldCount-1)

	e.updateFocus()
}

func (e ScriptEditor) GetResult() ScriptEditorResult {
	if !e.saving {
		return ScriptEditorResult{Cancelled: true}
	}

	name := e.nameInput.Value()
	description := e.descriptionInput.Value()
	command := e.commandTextarea.Value()
	scope := e.scopeInput.Value()

	script := &entities.Script{
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

func (e ScriptEditor) GetCommand() string {
	return e.commandTextarea.Value()
}

func (e ScriptEditor) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (e ScriptEditor) View() string {
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

	nameLabel := FieldLabelStyle.Render("Name:")
	if e.focusedField == EditorFieldName {
		nameLabel = FieldLabelStyle.Foreground(primaryColor).Render("Name:")
	}
	sections = append(sections, nameLabel)
	sections = append(sections, e.nameInput.View())

	descLabel := FieldLabelStyle.Render("Description:")
	if e.focusedField == EditorFieldDescription {
		descLabel = FieldLabelStyle.Foreground(primaryColor).Render("Description:")
	}
	sections = append(sections, descLabel)
	sections = append(sections, e.descriptionInput.View())

	cmdLabel := FieldLabelStyle.Render("Command:")
	if e.focusedField == EditorFieldCommand {
		cmdLabel = FieldLabelStyle.Foreground(primaryColor).Render("Command:")
	}
	sections = append(sections, cmdLabel)

	textareaView := e.commandTextarea.View()
	if e.focusedField == EditorFieldCommand {
		textareaView = TextAreaFocusedStyle.Render(textareaView)
	} else {
		textareaView = TextAreaStyle.Render(textareaView)
	}
	sections = append(sections, textareaView)

	scopeLabel := FieldLabelStyle.Render("Scope (directory path or 'global'):")
	if e.focusedField == EditorFieldScope {
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
	return ButtonContainerStyle.Width(width).Render(buttons)
}

func RunScriptEditor(script *entities.Script, isNewScript bool) (ScriptEditorResult, error) {
	program := tea.NewProgram(NewScriptEditor(script, isNewScript, 80, 24), tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		log.Printf("TUI error: %s", err)
		return ScriptEditorResult{Cancelled: true}, fmt.Errorf("TUI error: %w", err)
	}

	if editor, ok := finalModel.(ScriptEditor); ok {
		return editor.GetResult(), nil
	}

	return ScriptEditorResult{Cancelled: true}, nil
}

package tui

import (
	"fmt"
	"strings"

	"scripto/internal/args"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PlaceholderFormModel struct {
	placeholders []args.PlaceholderValue
	inputs       []textinput.Model
	focused      int
	submitted    bool
	cancelled    bool
	values       map[string]string
	buttonFocus  int // 0 = inputs, 1 = Execute button, 2 = Cancel button
}

func NewPlaceholderForm(placeholders []args.PlaceholderValue) PlaceholderFormModel {
	inputs := make([]textinput.Model, len(placeholders))

	for i, placeholder := range placeholders {
		input := textinput.New()
		input.Placeholder = placeholder.DefaultValue
		input.Width = 50

		if i == 0 {
			input.Focus()
		}

		inputs[i] = input
	}

	return PlaceholderFormModel{
		placeholders: placeholders,
		inputs:       inputs,
		focused:      0,
		values:       make(map[string]string),
		buttonFocus:  0, // Start with inputs focused
	}
}

func (m PlaceholderFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m PlaceholderFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }

		case "enter":
			if m.buttonFocus == 1 {
				m.submitted = true

				for i, placeholder := range m.placeholders {
					value := m.inputs[i].Value()
					if value == "" && placeholder.DefaultValue != "" {
						value = placeholder.DefaultValue
					}
					m.values[placeholder.Name] = value
				}

				values := m.values
				return m, func() tea.Msg { return PlaceholderFormDoneMsg{values: values} }
			} else if m.buttonFocus == 2 {
				m.cancelled = true
				return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }
			} else {
				if m.focused == len(m.inputs)-1 {
					return m.nextFocus()
				}
				return m.nextInput()
			}

		case "tab", "down":
			return m.nextFocus()

		case "shift+tab", "up":
			return m.prevFocus()
		}
	}

	if m.buttonFocus == 0 {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m PlaceholderFormModel) View() string {
	if m.submitted || m.cancelled {
		return ""
	}

	var b strings.Builder

	b.WriteString(FormTitleStyle.Render("Enter Placeholder Values"))
	b.WriteString("\n\n")

	for i, placeholder := range m.placeholders {
		label := placeholder.Name
		if placeholder.IsPositional {
			label = fmt.Sprintf("Argument %d", i+1)
		}

		b.WriteString(FieldLabelStyle.Render(label))
		b.WriteString("\n")

		if len(placeholder.Descriptions) > 1 {
			for i, desc := range placeholder.Descriptions {
				b.WriteString(" ")
				b.WriteString(DescriptionStyle.Render(fmt.Sprintf("%d. %s", i+1, desc)))
				b.WriteString("\n")
			}
		} else if placeholder.Description != "" {
			b.WriteString(" ")
			b.WriteString(DescriptionStyle.Render(fmt.Sprintf("(%s)", placeholder.Description)))
			b.WriteString("\n")
		}

		inputStyle := PlaceholderInputStyle
		if i == m.focused && m.buttonFocus == 0 {
			inputStyle = PlaceholderInputFocusedStyle
		}

		b.WriteString(inputStyle.Render(m.inputs[i].View()))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	executeStyle := PrimaryButtonStyle
	cancelStyle := DangerButtonStyle

	if m.buttonFocus == 1 {
		executeStyle = PrimaryButtonFocusedStyle
	}
	if m.buttonFocus == 2 {
		cancelStyle = DangerButtonFocusedStyle
	}

	executeButton := executeStyle.Render("Execute")
	cancelButton := cancelStyle.Render("Cancel")

	buttonsRow := lipgloss.JoinHorizontal(lipgloss.Left, executeButton, cancelButton)
	b.WriteString(buttonsRow)
	b.WriteString("\n\n")

	b.WriteString(InstructionStyle.Render("Tab/↓: Next • Shift+Tab/↑: Previous • Enter: Activate • Esc: Cancel"))

	return b.String()
}

func (m PlaceholderFormModel) nextFocus() (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 { // Currently in inputs
		if m.focused < len(m.inputs)-1 {
			m.inputs[m.focused].Blur()
			m.focused++
			return m, m.inputs[m.focused].Focus()
		} else {
			m.inputs[m.focused].Blur()
			m.buttonFocus = 1
			return m, nil
		}
	} else if m.buttonFocus == 1 { // Currently on Execute button
		m.buttonFocus = 2 // Move to Cancel button
		return m, nil
	} else { // Currently on Cancel button
		m.buttonFocus = 0
		m.focused = 0
		return m, m.inputs[m.focused].Focus()
	}
}

func (m PlaceholderFormModel) prevFocus() (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 { // Currently in inputs
		if m.focused > 0 {
			m.inputs[m.focused].Blur()
			m.focused--
			return m, m.inputs[m.focused].Focus()
		} else {
			m.inputs[m.focused].Blur()
			m.buttonFocus = 2
			return m, nil
		}
	} else if m.buttonFocus == 2 { // Currently on Cancel button
		m.buttonFocus = 1 // Move to Execute button
		return m, nil
	} else { // Currently on Execute button
		m.buttonFocus = 0
		m.focused = len(m.inputs) - 1
		return m, m.inputs[m.focused].Focus()
	}
}

func (m PlaceholderFormModel) nextInput() (PlaceholderFormModel, tea.Cmd) {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused + 1) % len(m.inputs)
	return m, m.inputs[m.focused].Focus()
}



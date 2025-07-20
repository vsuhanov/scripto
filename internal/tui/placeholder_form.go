package tui

import (
	"fmt"
	"strings"

	"scripto/internal/args"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PlaceholderFormModel represents the state of the placeholder collection form
type PlaceholderFormModel struct {
	placeholders []args.PlaceholderValue
	inputs       []textinput.Model
	focused      int
	submitted    bool
	cancelled    bool
	values       map[string]string
	buttonFocus  int // 0 = inputs, 1 = Execute button, 2 = Cancel button
}

// PlaceholderFormResult represents the result of the placeholder form
type PlaceholderFormResult struct {
	Values    map[string]string
	Cancelled bool
}

// NewPlaceholderForm creates a new placeholder collection form
func NewPlaceholderForm(placeholders []args.PlaceholderValue) PlaceholderFormModel {
	inputs := make([]textinput.Model, len(placeholders))
	
	for i, placeholder := range placeholders {
		input := textinput.New()
		input.Placeholder = placeholder.DefaultValue
		input.Width = 50
		
		// Auto-focus first input
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

// Init initializes the placeholder form
func (m PlaceholderFormModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the placeholder form
func (m PlaceholderFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
			
		case "enter":
			if m.buttonFocus == 1 { // Execute button focused
				m.submitted = true
				
				// Collect all values
				for i, placeholder := range m.placeholders {
					value := m.inputs[i].Value()
					if value == "" && placeholder.DefaultValue != "" {
						value = placeholder.DefaultValue
					}
					m.values[placeholder.Name] = value
				}
				
				return m, tea.Quit
			} else if m.buttonFocus == 2 { // Cancel button focused
				m.cancelled = true
				return m, tea.Quit
			} else {
				// In input field, move to next input or to buttons if at last input
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

	// Update the focused input only if we're in input mode
	if m.buttonFocus == 0 {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}
	
	return m, nil
}

// View renders the placeholder form
func (m PlaceholderFormModel) View() string {
	if m.submitted || m.cancelled {
		return ""
	}

	var b strings.Builder
	
	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)
	
	b.WriteString(titleStyle.Render("Enter Placeholder Values"))
	b.WriteString("\n\n")
	
	// Input fields
	for i, placeholder := range m.placeholders {
		// Label
		labelStyle := lipgloss.NewStyle().Bold(true)
		label := placeholder.Name
		if placeholder.IsPositional {
			label = fmt.Sprintf("Argument %d", i+1)
		}
		
		b.WriteString(labelStyle.Render(label))
		
		// Description
		if placeholder.Description != "" {
			descStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Italic(true)
			b.WriteString(" ")
			b.WriteString(descStyle.Render(fmt.Sprintf("(%s)", placeholder.Description)))
		}
		
		b.WriteString("\n")
		
		// Input field
		inputStyle := lipgloss.NewStyle().MarginBottom(1)
		if i == m.focused && m.buttonFocus == 0 {
			inputStyle = inputStyle.BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))
		}
		
		b.WriteString(inputStyle.Render(m.inputs[i].View()))
		b.WriteString("\n")
	}
	
	// Buttons
	b.WriteString("\n")
	executeStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Margin(0, 1).
		Background(lipgloss.Color("34")).
		Foreground(lipgloss.Color("255"))
		
	cancelStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Margin(0, 1).
		Background(lipgloss.Color("196")).
		Foreground(lipgloss.Color("255"))
	
	// Highlight focused button
	if m.buttonFocus == 1 {
		executeStyle = executeStyle.BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))
	}
	if m.buttonFocus == 2 {
		cancelStyle = cancelStyle.BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))
	}
	
	executeButton := executeStyle.Render("Execute")
	cancelButton := cancelStyle.Render("Cancel")
	
	buttonsRow := lipgloss.JoinHorizontal(lipgloss.Left, executeButton, cancelButton)
	b.WriteString(buttonsRow)
	b.WriteString("\n\n")
	
	// Instructions
	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)
	
	b.WriteString(instructionStyle.Render("Tab/↓: Next • Shift+Tab/↑: Previous • Enter: Activate • Esc: Cancel"))
	
	return b.String()
}

// nextFocus moves focus to the next element (input or button)
func (m PlaceholderFormModel) nextFocus() (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 { // Currently in inputs
		if m.focused < len(m.inputs)-1 {
			// Move to next input
			m.inputs[m.focused].Blur()
			m.focused++
			return m, m.inputs[m.focused].Focus()
		} else {
			// Move to Execute button
			m.inputs[m.focused].Blur()
			m.buttonFocus = 1
			return m, nil
		}
	} else if m.buttonFocus == 1 { // Currently on Execute button
		m.buttonFocus = 2 // Move to Cancel button
		return m, nil
	} else { // Currently on Cancel button
		// Move back to first input
		m.buttonFocus = 0
		m.focused = 0
		return m, m.inputs[m.focused].Focus()
	}
}

// prevFocus moves focus to the previous element (input or button)
func (m PlaceholderFormModel) prevFocus() (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 { // Currently in inputs
		if m.focused > 0 {
			// Move to previous input
			m.inputs[m.focused].Blur()
			m.focused--
			return m, m.inputs[m.focused].Focus()
		} else {
			// Move to Cancel button
			m.inputs[m.focused].Blur()
			m.buttonFocus = 2
			return m, nil
		}
	} else if m.buttonFocus == 2 { // Currently on Cancel button
		m.buttonFocus = 1 // Move to Execute button
		return m, nil
	} else { // Currently on Execute button
		// Move to last input
		m.buttonFocus = 0
		m.focused = len(m.inputs) - 1
		return m, m.inputs[m.focused].Focus()
	}
}

// nextInput moves focus to the next input (within inputs only)
func (m PlaceholderFormModel) nextInput() (PlaceholderFormModel, tea.Cmd) {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused + 1) % len(m.inputs)
	return m, m.inputs[m.focused].Focus()
}

// GetResult returns the form result
func (m PlaceholderFormModel) GetResult() PlaceholderFormResult {
	return PlaceholderFormResult{
		Values:    m.values,
		Cancelled: m.cancelled,
	}
}

// RunPlaceholderForm runs the placeholder form and returns the result
func RunPlaceholderForm(placeholders []args.PlaceholderValue) (PlaceholderFormResult, error) {
	if len(placeholders) == 0 {
		return PlaceholderFormResult{
			Values:    make(map[string]string),
			Cancelled: false,
		}, nil
	}

	model := NewPlaceholderForm(placeholders)
	p := tea.NewProgram(model)
	
	finalModel, err := p.Run()
	if err != nil {
		return PlaceholderFormResult{}, err
	}
	
	if m, ok := finalModel.(PlaceholderFormModel); ok {
		return m.GetResult(), nil
	}
	
	return PlaceholderFormResult{Cancelled: true}, nil
}
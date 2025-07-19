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
			// If on the last input, submit the form
			if m.focused == len(m.inputs)-1 {
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
			}
			
			// Move to next input
			return m.nextInput()
			
		case "tab", "shift+tab", "up", "down":
			if msg.String() == "shift+tab" || msg.String() == "up" {
				return m.prevInput()
			}
			return m.nextInput()
		}
	}

	// Update the focused input
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	
	return m, cmd
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
		if i == m.focused {
			inputStyle = inputStyle.BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))
		}
		
		b.WriteString(inputStyle.Render(m.inputs[i].View()))
		b.WriteString("\n")
	}
	
	// Instructions
	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)
	
	b.WriteString(instructionStyle.Render("Tab/Enter: Next field • Shift+Tab: Previous field • Esc: Cancel"))
	
	return b.String()
}

// nextInput moves focus to the next input
func (m PlaceholderFormModel) nextInput() (PlaceholderFormModel, tea.Cmd) {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused + 1) % len(m.inputs)
	return m, m.inputs[m.focused].Focus()
}

// prevInput moves focus to the previous input
func (m PlaceholderFormModel) prevInput() (PlaceholderFormModel, tea.Cmd) {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
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
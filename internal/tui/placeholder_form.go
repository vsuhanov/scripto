package tui

import (
	"crypto/sha256"
	"fmt"
	"log"
	"strings"
	"time"

	"scripto/entities"
	"scripto/internal/args"
	"scripto/internal/services"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PlaceholderFormModel struct {
	placeholders   []args.PlaceholderValue
	inputs         []textinput.Model
	focused        int
	submitted      bool
	cancelled      bool
	values         map[string]string
	buttonFocus    int
	script         *entities.Script
	viewport       viewport.Model
	width          int
	height         int
	container      *services.Container
	originalScript string
	historyRecords []services.ExecutionRecord
	historyTable      table.Model
	historyFocused    bool
	historyLoaded     bool
	savedInputValues  []string
}

type placeholderHistoryLoadedMsg struct {
	records []services.ExecutionRecord
}

const leftPaneWidth = 54

func NewPlaceholderForm(script *entities.Script, placeholders []args.PlaceholderValue,
	width, height int, container *services.Container, originalScript string) PlaceholderFormModel {
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

	rightPaneWidth := width - leftPaneWidth - 2
	if rightPaneWidth < 10 {
		rightPaneWidth = 10
	}
	vpWidth := rightPaneWidth - 4
	vpHeight := max(5, height-6)

	m := PlaceholderFormModel{
		placeholders:   placeholders,
		inputs:         inputs,
		focused:        0,
		values:         make(map[string]string),
		buttonFocus:    0,
		script:         script,
		viewport:       viewport.New(vpWidth, vpHeight),
		width:          width,
		height:         height,
		container:      container,
		originalScript: originalScript,
		historyFocused: false,
		historyLoaded:  false,
	}

	log.Printf("PlaceholderForm Init - Width: %d, Height: %d, RightPaneWidth: %d, ViewportWidth: %d, ViewportHeight: %d", width, height, rightPaneWidth, vpWidth, vpHeight)
	m.viewport.SetContent(m.buildPreviewContent(map[string]string{}))
	return m
}

func (m PlaceholderFormModel) buildPreviewContent(values map[string]string) string {
	if m.script == nil {
		return ""
	}
	return args.NewArgumentProcessor(m.script).BuildPreviewCommand(values)
}

func (m PlaceholderFormModel) currentValues() map[string]string {
	vals := make(map[string]string)
	for i, placeholder := range m.placeholders {
		vals[placeholder.Name] = m.inputs[i].Value()
	}
	return vals
}

func (m PlaceholderFormModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadHistory())
}

func (m PlaceholderFormModel) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if m.container == nil || m.container.ExecutionHistoryService == nil || m.script == nil || m.script.ID == "" {
			return placeholderHistoryLoadedMsg{records: nil}
		}
		records, err := m.container.ExecutionHistoryService.GetScriptHistory(m.script.ID, 50)
		if err != nil {
			return placeholderHistoryLoadedMsg{records: nil}
		}
		h := sha256.Sum256([]byte(m.originalScript))
		currentHash := fmt.Sprintf("%x", h)
		seen := map[string]bool{}
		filtered := records[:0]
		for _, r := range records {
			if r.OriginalScriptHash != currentHash {
				continue
			}
			key := fmt.Sprintf("%v", r.PlaceholderValues)
			if seen[key] {
				continue
			}
			seen[key] = true
			filtered = append(filtered, r)
		}
		return placeholderHistoryLoadedMsg{records: filtered}
	}
}

func (m PlaceholderFormModel) buildHistoryTable(records []services.ExecutionRecord, width int) table.Model {
	timeWidth := 17

	placeholderWidths := make([]int, len(m.placeholders))
	for i, p := range m.placeholders {
		placeholderWidths[i] = len(p.Name)
	}
	for _, r := range records {
		for i, p := range m.placeholders {
			if v := r.PlaceholderValues[p.Name]; len(v) > placeholderWidths[i] {
				placeholderWidths[i] = len(v)
			}
		}
	}

	cols := []table.Column{{Title: "Time", Width: timeWidth}}
	for i, p := range m.placeholders {
		cols = append(cols, table.Column{Title: p.Name, Width: placeholderWidths[i] + 2})
	}

	usedWidth := (timeWidth + 2)
	for i := range m.placeholders {
		usedWidth += placeholderWidths[i] + 2 + 2
	}
	if fillerWidth := width - usedWidth; fillerWidth > 0 {
		cols = append(cols, table.Column{Title: "", Width: fillerWidth})
	}

	rows := make([]table.Row, len(records))
	for i, r := range records {
		ts := time.Unix(r.ExecutionTimestamp, 0).Format("2006-01-02 15:04")
		row := table.Row{ts}
		for _, p := range m.placeholders {
			row = append(row, r.PlaceholderValues[p.Name])
		}
		rows[i] = row
	}

	tableHeight := len(records)
	if tableHeight > 10 {
		tableHeight = 10
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		BorderBottom(true).
		Bold(true).
		Foreground(primaryColor)
	s.Selected = s.Selected.
		Foreground(selectedTextColor).
		Background(selectedBgColor).
		Bold(true)

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableHeight+1),
		table.WithStyles(s),
	)
	return t
}

func (m *PlaceholderFormModel) saveInputValues() {
	m.savedInputValues = make([]string, len(m.inputs))
	for i, input := range m.inputs {
		m.savedInputValues[i] = input.Value()
	}
}

func (m *PlaceholderFormModel) restoreInputValues() {
	for i := range m.inputs {
		if i < len(m.savedInputValues) {
			m.inputs[i].SetValue(m.savedInputValues[i])
		}
	}
	m.viewport.SetContent(m.buildPreviewContent(m.currentValues()))
}

func (m *PlaceholderFormModel) fillFromSelectedRow() {
	row := m.historyTable.SelectedRow()
	if row == nil || len(row) < 1 {
		return
	}
	for i := range m.placeholders {
		colIdx := i + 1
		if colIdx < len(row) {
			m.inputs[i].SetValue(row[colIdx])
		}
	}
	m.viewport.SetContent(m.buildPreviewContent(m.currentValues()))
}

func (m PlaceholderFormModel) tableHeight() int {
	h := len(m.historyRecords)
	if h > 10 {
		h = 10
	}
	return h + 4
}

func (m PlaceholderFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		rightPaneWidth := m.width - leftPaneWidth - 2
		if rightPaneWidth < 10 {
			rightPaneWidth = 10
		}
		m.viewport.Width = rightPaneWidth - 4
		vpHeight := max(5, m.height-6)
		if m.historyLoaded && len(m.historyRecords) > 0 {
			vpHeight = max(5, m.height-6-m.tableHeight())
		}
		m.viewport.Height = vpHeight
		log.Printf("PlaceholderForm WindowSize - Width: %d, Height: %d, LeftPaneWidth: %d, RightPaneWidth: %d, ViewportWidth: %d, ViewportHeight: %d", m.width, m.height, leftPaneWidth, rightPaneWidth, m.viewport.Width, m.viewport.Height)
		return m, nil

	case placeholderHistoryLoadedMsg:
		m.historyLoaded = true
		if len(msg.records) > 0 {
			m.historyRecords = msg.records
			m.historyTable = m.buildHistoryTable(msg.records, m.width-4)
			m.historyFocused = true
			m.inputs[0].Blur()
			m.saveInputValues()
			m.fillFromSelectedRow()
			vpHeight := max(5, m.height-6-m.tableHeight())
			m.viewport.Height = vpHeight
		}
		return m, nil

	case tea.KeyMsg:
		if m.historyFocused {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.cancelled = true
				return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }

			case "j", "down", "k", "up":
				var cmd tea.Cmd
				m.historyTable, cmd = m.historyTable.Update(msg)
				m.fillFromSelectedRow()
				return m, cmd

			case "enter":
				m.fillFromSelectedRow()
				m.saveInputValues()
				m.historyFocused = false
				return m, m.inputs[0].Focus()

			case "x":
				m.fillFromSelectedRow()
				values := m.currentValues()
				for _, placeholder := range m.placeholders {
					if values[placeholder.Name] == "" && placeholder.DefaultValue != "" {
						values[placeholder.Name] = placeholder.DefaultValue
					}
				}
				return m, func() tea.Msg { return PlaceholderFormDoneMsg{values: values} }

			case "tab":
				m.restoreInputValues()
				m.historyFocused = false
				return m, m.inputs[0].Focus()
			}
			return m, nil
		}

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
			if m.historyLoaded && len(m.historyRecords) > 0 && m.focused == 0 && m.buttonFocus == 0 {
				m.inputs[m.focused].Blur()
				m.saveInputValues()
				m.historyFocused = true
				return m, nil
			}
			return m.prevFocus()
		}
	}

	if !m.historyFocused && m.buttonFocus == 0 {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		m.viewport.SetContent(m.buildPreviewContent(m.currentValues()))
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
		if i == m.focused && m.buttonFocus == 0 && !m.historyFocused {
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

	instructions := "Tab/↓: Next • Shift+Tab/↑: Previous • Enter: Activate • Esc: Cancel"
	if m.historyFocused {
		instructions = "j/k: Navigate • Enter: Edit values • x: Execute • Esc: Cancel"
	}
	b.WriteString(InstructionStyle.Render(instructions))

	leftPane := PreviewStyle.Width(leftPaneWidth).Render(b.String())

	rightWidth := m.width - leftPaneWidth - 4
	if rightWidth < 10 {
		rightWidth = 10
	}
	log.Printf("PlaceholderForm View - Width: %d, Height: %d, LeftPaneWidth: %d, RightWidth: %d, ViewportWidth: %d, ViewportHeight: %d, RenderedLeftWidth: %d", m.width, m.height, leftPaneWidth, rightWidth, m.viewport.Width, m.viewport.Height, lipgloss.Width(leftPane))

	previewTitle := PreviewTitleStyle.Render("Preview")
	previewContent := previewTitle + "\n" + m.viewport.View()
	rightPane := PreviewStyle.Width(rightWidth).Render(previewContent)

	formRow := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	if m.historyLoaded && len(m.historyRecords) > 0 {
		tableTitle := PreviewTitleStyle.Render("Recent Executions")
		tableView := m.historyTable.View()
		historySection := lipgloss.JoinVertical(lipgloss.Left, tableTitle, tableView)
		return lipgloss.JoinVertical(lipgloss.Left, historySection, formRow)
	}

	return formRow
}

func (m PlaceholderFormModel) nextFocus() (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 {
		if m.focused < len(m.inputs)-1 {
			m.inputs[m.focused].Blur()
			m.focused++
			return m, m.inputs[m.focused].Focus()
		} else {
			m.inputs[m.focused].Blur()
			m.buttonFocus = 1
			return m, nil
		}
	} else if m.buttonFocus == 1 {
		m.buttonFocus = 2
		return m, nil
	} else {
		m.buttonFocus = 0
		m.focused = 0
		return m, m.inputs[m.focused].Focus()
	}
}

func (m PlaceholderFormModel) prevFocus() (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 {
		if m.focused > 0 {
			m.inputs[m.focused].Blur()
			m.focused--
			return m, m.inputs[m.focused].Focus()
		} else {
			m.inputs[m.focused].Blur()
			m.buttonFocus = 2
			return m, nil
		}
	} else if m.buttonFocus == 2 {
		m.buttonFocus = 1
		return m, nil
	} else {
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

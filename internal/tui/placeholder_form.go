package tui

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"scripto/entities"
	"scripto/internal/args"
	"scripto/internal/services"
	"scripto/internal/templatex"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type selectItem struct{ value string }

func (i selectItem) FilterValue() string { return i.value }
func (i selectItem) Title() string       { return i.value }
func (i selectItem) Description() string { return "" }

type fieldControl struct {
	isSelect bool
	input    textinput.Model
	picker   list.Model
}

func (f fieldControl) Value() string {
	if f.isSelect {
		if sel, ok := f.picker.SelectedItem().(selectItem); ok {
			return sel.value
		}
		return ""
	}
	return f.input.Value()
}

func (f *fieldControl) SetValue(v string) {
	if f.isSelect {
		items := f.picker.Items()
		for i, it := range items {
			if it.(selectItem).value == v {
				f.picker.Select(i)
				return
			}
		}
		newItems := make([]list.Item, 0, len(items)+1)
		newItems = append(newItems, selectItem{value: v})
		newItems = append(newItems, items...)
		f.picker.SetItems(newItems)
		f.picker.Select(0)
		return
	}
	f.input.SetValue(v)
}

func (f *fieldControl) Focus() tea.Cmd {
	if f.isSelect {
		return nil
	}
	return f.input.Focus()
}

func (f *fieldControl) Blur() {
	if !f.isSelect {
		f.input.Blur()
	}
}

func buildSelectItems(allowedValues []string, defaultValue string) []list.Item {
	items := make([]list.Item, 0, len(allowedValues))
	found := false
	for _, v := range allowedValues {
		if v == defaultValue {
			found = true
		}
		items = append(items, selectItem{value: v})
	}
	if defaultValue != "" && !found {
		items = append([]list.Item{selectItem{value: defaultValue}}, items...)
	}
	return items
}

func newSelectPicker(meta templatex.VariableMeta) list.Model {
	items := buildSelectItems(meta.AllowedValues, meta.DefaultValue)

	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetHeight(1)
	d.SetSpacing(0)
	d.Styles.NormalTitle = d.Styles.NormalTitle.Foreground(mutedTextColor)
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.
		Foreground(selectedTextColor).
		Background(selectedBgColor).
		BorderForeground(selectedBgColor)

	picker := list.New(items, d, leftPaneWidth-6, len(items))
	picker.SetShowTitle(false)
	picker.SetShowFilter(false)
	picker.SetShowStatusBar(false)
	picker.SetShowPagination(false)
	picker.SetShowHelp(false)
	picker.DisableQuitKeybindings()

	if meta.DefaultValue != "" {
		for i, it := range items {
			if it.(selectItem).value == meta.DefaultValue {
				picker.Select(i)
				break
			}
		}
	}
	return picker
}

type PlaceholderFormModel struct {
	placeholders  []templatex.VariableMeta
	fields        []fieldControl
	focused       int
	submitted     bool
	cancelled     bool
	values        map[string]string
	buttonFocus   int
	script        *entities.Script
	viewport      viewport.Model
	width         int
	height        int
	container     *services.Container
	originalScript string
	historyRecords   []services.ExecutionRecord
	historyTable     table.Model
	historyFocused   bool
	historyLoaded    bool
	savedInputValues []string

	showWorkingDir    bool
	workingDirInput   textinput.Model
	workingDirFocused bool
	useCwdFocused     bool
}

type placeholderHistoryLoadedMsg struct {
	records []services.ExecutionRecord
}

const leftPaneWidth = 54

func NewPlaceholderForm(script *entities.Script, placeholders []templatex.VariableMeta,
	width, height int, container *services.Container, originalScript string, workingDir string) PlaceholderFormModel {
	fields := make([]fieldControl, len(placeholders))

	for i, placeholder := range placeholders {
		if len(placeholder.AllowedValues) > 0 {
			fields[i] = fieldControl{isSelect: true, picker: newSelectPicker(placeholder)}
		} else {
			input := textinput.New()
			input.Placeholder = placeholder.DefaultValue
			input.Width = 50
			fields[i] = fieldControl{isSelect: false, input: input}
		}
	}

	wdInput := textinput.New()
	wdInput.Placeholder = "working directory..."
	wdInput.Width = leftPaneWidth - 20

	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	wdInput.SetValue(workingDir)

	workingDirFocused := true
	wdInput.Focus()
	if len(fields) > 0 {
		fields[0].Focus()
		workingDirFocused = false
	}

	rightPaneWidth := width - leftPaneWidth - 2
	if rightPaneWidth < 10 {
		rightPaneWidth = 10
	}
	vpWidth := rightPaneWidth - 4
	vpHeight := max(5, height-6)

	m := PlaceholderFormModel{
		placeholders:      placeholders,
		fields:            fields,
		focused:           0,
		values:            make(map[string]string),
		buttonFocus:       0,
		script:            script,
		viewport:          viewport.New(vpWidth, vpHeight),
		width:             width,
		height:            height,
		container:         container,
		originalScript:    originalScript,
		historyFocused:    false,
		historyLoaded:     false,
		showWorkingDir:    true,
		workingDirInput:   wdInput,
		workingDirFocused: workingDirFocused,
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
		vals[placeholder.Name] = m.fields[i].Value()
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
		if len(m.placeholders) == 0 {
			return placeholderHistoryLoadedMsg{records: records}
		}
		seen := map[string]bool{}
		filtered := records[:0]
		for _, r := range records {
			parts := make([]string, len(m.placeholders))
			for i, p := range m.placeholders {
				parts[i] = p.Name + "=" + r.PlaceholderValues[p.Name]
			}
			key := strings.Join(parts, ",")
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
	wdWidth := 0
	if m.showWorkingDir {
		wdWidth = 15
	}

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
	if m.showWorkingDir {
		cols = append(cols, table.Column{Title: "Working Dir", Width: wdWidth})
	}
	for i, p := range m.placeholders {
		cols = append(cols, table.Column{Title: p.Name, Width: placeholderWidths[i] + 2})
	}

	usedWidth := timeWidth + 2 + wdWidth
	if wdWidth > 0 {
		usedWidth += 2
	}
	for i := range m.placeholders {
		usedWidth += placeholderWidths[i] + 2 + 2
	}
	hasFiller := false
	if fillerWidth := width - usedWidth - 2; fillerWidth > 0 {
		cols = append(cols, table.Column{Title: "", Width: fillerWidth})
		hasFiller = true
	}

	rows := make([]table.Row, len(records))
	for i, r := range records {
		ts := time.Unix(r.ExecutionTimestamp, 0).Format("2006-01-02 15:04")
		row := table.Row{ts}
		if m.showWorkingDir {
			wd := filepath.Base(r.WorkingDirectory)
			if len(wd) > wdWidth {
				wd = wd[:wdWidth-1] + "…"
			}
			row = append(row, wd)
		}
		for _, p := range m.placeholders {
			row = append(row, r.PlaceholderValues[p.Name])
		}
		if hasFiller {
			row = append(row, "")
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
	m.savedInputValues = make([]string, len(m.fields))
	for i := range m.fields {
		m.savedInputValues[i] = m.fields[i].Value()
	}
}

func (m *PlaceholderFormModel) restoreInputValues() {
	for i := range m.fields {
		if i < len(m.savedInputValues) {
			m.fields[i].SetValue(m.savedInputValues[i])
		}
	}
	m.viewport.SetContent(m.buildPreviewContent(m.currentValues()))
}

func (m *PlaceholderFormModel) fillFromSelectedRow() {
	cursor := m.historyTable.Cursor()
	if cursor >= len(m.historyRecords) {
		return
	}
	r := m.historyRecords[cursor]
	for i, p := range m.placeholders {
		m.fields[i].SetValue(r.PlaceholderValues[p.Name])
	}
	if m.showWorkingDir && r.WorkingDirectory != "" {
		m.workingDirInput.SetValue(r.WorkingDirectory)
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
			m.workingDirFocused = false
			m.workingDirInput.Blur()
			if len(m.fields) > 0 {
				m.fields[0].Blur()
			}
			m.saveInputValues()
			m.fillFromSelectedRow()
			vpHeight := max(5, m.height-6-m.tableHeight())
			m.viewport.Height = vpHeight
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+u" && m.showWorkingDir {
			cwd, _ := os.Getwd()
			m.workingDirInput.SetValue(cwd)
			return m, nil
		}

		if m.historyFocused {
			return m.handleHistoryKey(msg)
		}

		if m.workingDirFocused {
			return m.handleWorkingDirKey(msg)
		}

		if m.useCwdFocused {
			return m.handleUseCwdKey(msg)
		}

		return m.handleFormKey(msg)
	}

	if !m.historyFocused && !m.workingDirFocused && m.buttonFocus == 0 && len(m.fields) > 0 {
		if !m.fields[m.focused].isSelect {
			var cmd tea.Cmd
			m.fields[m.focused].input, cmd = m.fields[m.focused].input.Update(msg)
			m.viewport.SetContent(m.buildPreviewContent(m.currentValues()))
			return m, cmd
		}
	}

	if m.workingDirFocused {
		var cmd tea.Cmd
		m.workingDirInput, cmd = m.workingDirInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m PlaceholderFormModel) handleHistoryKey(msg tea.KeyMsg) (PlaceholderFormModel, tea.Cmd) {
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
		if m.showWorkingDir {
			m.workingDirFocused = true
			return m, m.workingDirInput.Focus()
		}
		if len(m.fields) > 0 {
			return m, m.fields[0].Focus()
		}
		m.buttonFocus = 1
		return m, nil

	case "x":
		m.fillFromSelectedRow()
		values := m.currentValues()
		for _, placeholder := range m.placeholders {
			if values[placeholder.Name] == "" && placeholder.DefaultValue != "" {
				values[placeholder.Name] = placeholder.DefaultValue
			}
		}
		workingDir := m.workingDirInput.Value()
		return m, func() tea.Msg { return PlaceholderFormDoneMsg{values: values, workingDir: workingDir} }

	case "tab":
		m.restoreInputValues()
		m.historyFocused = false
		if m.showWorkingDir {
			m.workingDirFocused = true
			return m, m.workingDirInput.Focus()
		}
		if len(m.fields) > 0 {
			return m, m.fields[0].Focus()
		}
		m.buttonFocus = 1
		return m, nil
	}
	return m, nil
}

func (m PlaceholderFormModel) handleWorkingDirKey(msg tea.KeyMsg) (PlaceholderFormModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }

	case "tab", "down":
		m.workingDirInput.Blur()
		m.workingDirFocused = false
		m.useCwdFocused = true
		return m, nil

	case "enter":
		m.workingDirInput.Blur()
		m.workingDirFocused = false
		if len(m.fields) > 0 {
			m.focused = 0
			m.buttonFocus = 0
			return m, m.fields[0].Focus()
		}
		m.buttonFocus = 1
		return m, nil

	case "shift+tab", "up":
		m.workingDirInput.Blur()
		m.workingDirFocused = false
		if m.historyLoaded && len(m.historyRecords) > 0 {
			m.historyFocused = true
			return m, nil
		}
		m.buttonFocus = 2
		return m, nil

	default:
		var cmd tea.Cmd
		m.workingDirInput, cmd = m.workingDirInput.Update(msg)
		return m, cmd
	}
}

func (m PlaceholderFormModel) handleUseCwdKey(msg tea.KeyMsg) (PlaceholderFormModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }

	case "enter", " ":
		cwd, _ := os.Getwd()
		m.workingDirInput.SetValue(cwd)
		m.useCwdFocused = false
		m.workingDirFocused = true
		return m, m.workingDirInput.Focus()

	case "tab", "down":
		m.useCwdFocused = false
		if len(m.fields) > 0 {
			m.focused = 0
			m.buttonFocus = 0
			return m, m.fields[0].Focus()
		}
		m.buttonFocus = 1
		return m, nil

	case "shift+tab", "up":
		m.useCwdFocused = false
		m.workingDirFocused = true
		return m, m.workingDirInput.Focus()
	}
	return m, nil
}

func (m PlaceholderFormModel) handleFormKey(msg tea.KeyMsg) (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 && len(m.fields) > 0 && m.fields[m.focused].isSelect {
		switch msg.String() {
		case "j", "down", "l", "ctrl+n":
			idx := m.fields[m.focused].picker.Index()
			nitems := len(m.fields[m.focused].picker.Items())
			if idx < nitems-1 {
				m.fields[m.focused].picker.Select(idx + 1)
			}
			m.viewport.SetContent(m.buildPreviewContent(m.currentValues()))
			return m, nil
		case "k", "up", "h", "ctrl+p":
			idx := m.fields[m.focused].picker.Index()
			if idx > 0 {
				m.fields[m.focused].picker.Select(idx - 1)
			}
			m.viewport.SetContent(m.buildPreviewContent(m.currentValues()))
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }

	case "enter":
		if m.buttonFocus == 1 {
			m.submitted = true
			for i, placeholder := range m.placeholders {
				value := m.fields[i].Value()
				if value == "" && placeholder.DefaultValue != "" {
					value = placeholder.DefaultValue
				}
				m.values[placeholder.Name] = value
			}
			values := m.values
			workingDir := m.workingDirInput.Value()
			return m, func() tea.Msg { return PlaceholderFormDoneMsg{values: values, workingDir: workingDir} }
		} else if m.buttonFocus == 2 {
			m.cancelled = true
			return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }
		} else {
			if len(m.fields) == 0 {
				return m, nil
			}
			if m.focused == len(m.fields)-1 {
				return m.nextFocus()
			}
			return m.nextInput()
		}

	case "tab", "down":
		return m.nextFocus()

	case "shift+tab", "up":
		if m.showWorkingDir && m.buttonFocus == 0 && m.focused == 0 {
			if len(m.fields) > 0 {
				m.fields[m.focused].Blur()
			}
			m.useCwdFocused = true
			return m, nil
		}
		if m.historyLoaded && len(m.historyRecords) > 0 && m.focused == 0 && m.buttonFocus == 0 && !m.showWorkingDir {
			if len(m.fields) > 0 {
				m.fields[m.focused].Blur()
			}
			m.saveInputValues()
			m.historyFocused = true
			return m, nil
		}
		return m.prevFocus()

	default:
		if m.buttonFocus == 0 && len(m.fields) > 0 && !m.fields[m.focused].isSelect {
			var cmd tea.Cmd
			m.fields[m.focused].input, cmd = m.fields[m.focused].input.Update(msg)
			m.viewport.SetContent(m.buildPreviewContent(m.currentValues()))
			return m, cmd
		}
	}
	return m, nil
}

func (m PlaceholderFormModel) nextFocus() (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 {
		if len(m.fields) > 0 && m.focused < len(m.fields)-1 {
			m.fields[m.focused].Blur()
			m.focused++
			return m, m.fields[m.focused].Focus()
		} else {
			if len(m.fields) > 0 {
				m.fields[m.focused].Blur()
			}
			m.buttonFocus = 1
			return m, nil
		}
	} else if m.buttonFocus == 1 {
		m.buttonFocus = 2
		return m, nil
	} else {
		m.buttonFocus = 0
		if m.showWorkingDir {
			m.workingDirFocused = true
			return m, m.workingDirInput.Focus()
		}
		if len(m.fields) > 0 {
			m.focused = 0
			return m, m.fields[0].Focus()
		}
		m.buttonFocus = 1
		return m, nil
	}
}

func (m PlaceholderFormModel) prevFocus() (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 {
		if len(m.fields) > 0 && m.focused > 0 {
			m.fields[m.focused].Blur()
			m.focused--
			return m, m.fields[m.focused].Focus()
		} else {
			if len(m.fields) > 0 {
				m.fields[m.focused].Blur()
			}
			m.buttonFocus = 2
			return m, nil
		}
	} else if m.buttonFocus == 2 {
		m.buttonFocus = 1
		return m, nil
	} else {
		// Execute button → go back to last input, or useCwd, or workingDir
		m.buttonFocus = 0
		if len(m.fields) > 0 {
			m.focused = len(m.fields) - 1
			return m, m.fields[m.focused].Focus()
		}
		if m.showWorkingDir {
			m.useCwdFocused = true
			return m, nil
		}
		m.buttonFocus = 2
		return m, nil
	}
}

func (m PlaceholderFormModel) nextInput() (PlaceholderFormModel, tea.Cmd) {
	m.fields[m.focused].Blur()
	m.focused = (m.focused + 1) % len(m.fields)
	return m, m.fields[m.focused].Focus()
}

func (m PlaceholderFormModel) View() string {
	if m.submitted || m.cancelled {
		return ""
	}

	var b strings.Builder

	b.WriteString(FormTitleStyle.Render("Enter Placeholder Values"))
	b.WriteString("\n\n")

	if m.showWorkingDir {
		b.WriteString(FieldLabelStyle.Render("Working Dir"))
		b.WriteString("\n")
		cwdBtnStyle := PrimaryButtonStyle.Margin(0)
		if m.useCwdFocused {
			cwdBtnStyle = PrimaryButtonFocusedStyle.Margin(0)
		}
		b.WriteString(m.workingDirInput.View())
		b.WriteString("  ")
		b.WriteString(cwdBtnStyle.Render("use cwd"))
		b.WriteString("\n\n")
	}

	for i, placeholder := range m.placeholders {
		b.WriteString(FieldLabelStyle.Render(placeholder.Label))
		b.WriteString("\n")

		field := m.fields[i]
		focused := i == m.focused && m.buttonFocus == 0 && !m.historyFocused && !m.workingDirFocused

		fieldStyle := PlaceholderInputStyle
		if focused {
			fieldStyle = PlaceholderInputFocusedStyle
		}

		if field.isSelect {
			b.WriteString(fieldStyle.Render(field.picker.View()))
		} else {
			b.WriteString(fieldStyle.Render(field.input.View()))
		}
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

	instructions := "Tab/↓: Next • Shift+Tab/↑: Prev • Enter: Activate • Esc: Cancel"
	if m.historyFocused {
		instructions = "j/k: Navigate • Enter: Edit • x: Execute • Esc: Cancel"
	} else if m.workingDirFocused {
		instructions = "Tab: use cwd button • Enter: Next • Shift+Tab: Prev • ctrl+u: Use cwd • Esc: Cancel"
	} else if m.useCwdFocused {
		instructions = "Enter/Space: Set cwd • Tab: Next • Shift+Tab: Back • Esc: Cancel"
	} else if m.buttonFocus == 0 && len(m.fields) > 0 && m.fields[m.focused].isSelect {
		instructions = "j/k: Select • Tab: Next • Shift+Tab: Prev • Enter: Submit • Esc: Cancel"
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

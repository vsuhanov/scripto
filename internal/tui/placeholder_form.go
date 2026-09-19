package tui

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/args"
	"github.com/vsuhanov/scripto/internal/services"
	"github.com/vsuhanov/scripto/internal/templatex"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
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

// placeholderHistoryRow is a display-agnostic history entry, built either from
// a script's ExecutionRecord or from a raw command's ShellHistoryRecord, so
// the history table/selection logic doesn't need two parallel implementations.
type placeholderHistoryRow struct {
	timestamp         int64
	workingDir        string
	placeholderValues map[string]string
}

type PlaceholderFormModel struct {
	placeholders   []templatex.VariableMeta
	fields         []fieldControl
	focused        int
	submitted      bool
	cancelled      bool
	values         map[string]string
	buttonFocus    int
	script         *entities.Script
	rawCommand     string
	viewport       viewport.Model
	width          int
	height         int
	container      *services.Container
	originalScript string

	historyRecords   []placeholderHistoryRow
	historyTable     table.Model
	historyFocused   bool
	historyLoaded    bool
	savedInputValues []string

	showWorkingDir    bool
	wdChoice          int // 0 = execute in cwd, 1 = execute in a different directory
	wdButtonsFocused  bool
	workingDirInput   textinput.Model
	workingDirFocused bool

	previewText     string
	commandOverride string
	previewFocused  bool
	previewEditing  bool
	previewTextarea textarea.Model
}

type placeholderHistoryLoadedMsg struct {
	records []placeholderHistoryRow
}

const leftPaneWidth = 54

func NewPlaceholderForm(script *entities.Script, placeholders []templatex.VariableMeta,
	width, height int, container *services.Container, originalScript string, workingDir string, rawCommand string) PlaceholderFormModel {
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
	wdWidth := width - 22
	if wdWidth < 20 {
		wdWidth = 20
	}
	wdInput.Width = wdWidth

	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	wdInput.SetValue(workingDir)

	showWorkingDir := true
	if script != nil {
		realScope := script.Scope
		if script.OriginalScope != "" {
			realScope = script.OriginalScope
		}
		if realScope == "global" {
			showWorkingDir = false
			cwd, _ := os.Getwd()
			wdInput.SetValue(cwd)
		}
	}

	wdButtonsFocused := false
	if len(fields) > 0 {
		fields[0].Focus()
	} else if showWorkingDir {
		wdButtonsFocused = true
	}

	vpWidth := width - 6
	if vpWidth < 10 {
		vpWidth = 10
	}
	vpHeight := max(3, height-6)

	ta := textarea.New()
	plain := lipgloss.NewStyle()
	ta.FocusedStyle.Base = plain
	ta.FocusedStyle.CursorLine = plain
	ta.FocusedStyle.CursorLineNumber = plain.Foreground(mutedTextColor)
	ta.FocusedStyle.LineNumber = plain.Foreground(mutedTextColor)
	ta.FocusedStyle.Prompt = plain.Foreground(primaryColor)
	ta.FocusedStyle.Text = plain
	ta.BlurredStyle.Base = plain
	ta.BlurredStyle.CursorLine = plain
	ta.BlurredStyle.CursorLineNumber = plain.Foreground(mutedTextColor)
	ta.BlurredStyle.LineNumber = plain.Foreground(mutedTextColor)
	ta.BlurredStyle.Prompt = plain.Foreground(mutedTextColor)
	ta.BlurredStyle.Text = plain
	ta.ShowLineNumbers = false

	m := PlaceholderFormModel{
		placeholders:      placeholders,
		fields:            fields,
		focused:           0,
		values:            make(map[string]string),
		buttonFocus:       0,
		script:            script,
		rawCommand:        rawCommand,
		viewport:          viewport.New(vpWidth, vpHeight),
		width:             width,
		height:            height,
		container:         container,
		originalScript:    originalScript,
		historyFocused:    false,
		historyLoaded:     false,
		showWorkingDir:    showWorkingDir,
		workingDirInput:   wdInput,
		workingDirFocused: false,
		wdButtonsFocused:  wdButtonsFocused,
		previewTextarea:   ta,
	}

	log.Printf("PlaceholderForm Init - Width: %d, Height: %d, ViewportWidth: %d, ViewportHeight: %d", width, height, vpWidth, vpHeight)
	m.previewText = m.buildPreviewContent(map[string]string{})
	m.viewport.SetContent(m.previewText)
	return m
}

func (m PlaceholderFormModel) buildPreviewContent(values map[string]string) string {
	if m.script == nil {
		return m.rawCommand
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

// currentPreviewText returns what the preview pane should show: a manual
// override always wins once set, otherwise it's recomputed from the current
// placeholder/raw command state.
func (m PlaceholderFormModel) currentPreviewText() string {
	if m.commandOverride != "" {
		return m.commandOverride
	}
	return m.buildPreviewContent(m.currentValues())
}

func (m *PlaceholderFormModel) refreshPreview() {
	m.previewText = m.currentPreviewText()
	m.viewport.SetContent(m.previewText)
}

func (m PlaceholderFormModel) resolvedWorkingDir() string {
	if !m.showWorkingDir || m.wdChoice == 0 {
		cwd, _ := os.Getwd()
		return cwd
	}
	return m.workingDirInput.Value()
}

func (m PlaceholderFormModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadHistory())
}

func (m PlaceholderFormModel) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if m.script == nil {
			return m.loadRawHistory()
		}

		if m.container == nil || m.container.ExecutionHistoryService == nil || m.script.ID == "" {
			return placeholderHistoryLoadedMsg{records: nil}
		}
		records, err := m.container.ExecutionHistoryService.GetScriptHistory(m.script.ID, 50)
		if err != nil {
			return placeholderHistoryLoadedMsg{records: nil}
		}

		rows := make([]placeholderHistoryRow, 0, len(records))
		if len(m.placeholders) == 0 {
			for _, r := range records {
				rows = append(rows, placeholderHistoryRow{
					timestamp:         r.ExecutionTimestamp,
					workingDir:        r.WorkingDirectory,
					placeholderValues: r.PlaceholderValues,
				})
			}
			return placeholderHistoryLoadedMsg{records: rows}
		}

		seen := map[string]bool{}
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
			rows = append(rows, placeholderHistoryRow{
				timestamp:         r.ExecutionTimestamp,
				workingDir:        r.WorkingDirectory,
				placeholderValues: r.PlaceholderValues,
			})
		}
		return placeholderHistoryLoadedMsg{records: rows}
	}
}

// loadRawHistory finds past shell-history runs of this exact command text, so
// a scriptless re-execution can still offer "here's where you ran this
// before" the same way a saved script's execution history does.
func (m PlaceholderFormModel) loadRawHistory() tea.Msg {
	if m.container == nil || m.container.ShellHistoryService == nil || strings.TrimSpace(m.rawCommand) == "" {
		return placeholderHistoryLoadedMsg{records: nil}
	}

	query := services.ShellHistoryQuery{
		Filter: "^" + regexp.QuoteMeta(m.rawCommand) + "$",
		Limit:  50,
	}
	records, err := m.container.ShellHistoryService.GetHistory(query)
	if err != nil {
		return placeholderHistoryLoadedMsg{records: nil}
	}

	seen := map[string]bool{}
	rows := make([]placeholderHistoryRow, 0, len(records))
	for _, r := range records {
		if seen[r.WorkingDirectory] {
			continue
		}
		seen[r.WorkingDirectory] = true
		rows = append(rows, placeholderHistoryRow{timestamp: r.StartedAt, workingDir: r.WorkingDirectory})
	}
	return placeholderHistoryLoadedMsg{records: rows}
}

func (m PlaceholderFormModel) buildHistoryTable(records []placeholderHistoryRow, width int) table.Model {
	timeWidth := 17
	wdWidth := 15

	placeholderWidths := make([]int, len(m.placeholders))
	for i, p := range m.placeholders {
		placeholderWidths[i] = len(p.Name)
	}
	for _, r := range records {
		for i, p := range m.placeholders {
			if v := r.placeholderValues[p.Name]; len(v) > placeholderWidths[i] {
				placeholderWidths[i] = len(v)
			}
		}
	}

	cols := []table.Column{{Title: "Time", Width: timeWidth}}
	cols = append(cols, table.Column{Title: "Working Dir", Width: wdWidth})
	for i, p := range m.placeholders {
		cols = append(cols, table.Column{Title: p.Name, Width: placeholderWidths[i] + 2})
	}

	usedWidth := timeWidth + 2 + wdWidth + 2
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
		ts := time.Unix(r.timestamp, 0).Format("2006-01-02 15:04")
		row := table.Row{ts}
		wd := filepath.Base(r.workingDir)
		if len(wd) > wdWidth {
			wd = wd[:wdWidth-1] + "…"
		}
		row = append(row, wd)
		for _, p := range m.placeholders {
			row = append(row, r.placeholderValues[p.Name])
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
	m.refreshPreview()
}

func (m *PlaceholderFormModel) fillFromSelectedRow() {
	cursor := m.historyTable.Cursor()
	if cursor >= len(m.historyRecords) {
		return
	}
	r := m.historyRecords[cursor]
	for i, p := range m.placeholders {
		m.fields[i].SetValue(r.placeholderValues[p.Name])
	}
	if m.showWorkingDir && r.workingDir != "" {
		m.workingDirInput.SetValue(r.workingDir)
	}
	m.refreshPreview()
}

func (m PlaceholderFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpWidth := m.width - 6
		if vpWidth < 10 {
			vpWidth = 10
		}
		m.viewport.Width = vpWidth
		wdWidth := m.width - 22
		if wdWidth < 20 {
			wdWidth = 20
		}
		m.workingDirInput.Width = wdWidth
		if m.previewEditing {
			m.previewTextarea.SetWidth(vpWidth)
			m.previewTextarea.SetHeight(max(3, m.viewport.Height))
		}
		log.Printf("PlaceholderForm WindowSize - Width: %d, Height: %d, ViewportWidth: %d, ViewportHeight: %d", m.width, m.height, m.viewport.Width, m.viewport.Height)
		return m, nil

	case placeholderHistoryLoadedMsg:
		m.historyLoaded = true
		if len(msg.records) > 0 {
			m.historyRecords = msg.records
			m.historyTable = m.buildHistoryTable(msg.records, m.width-4)
			m.historyFocused = true
			m.wdButtonsFocused = false
			m.workingDirFocused = false
			m.workingDirInput.Blur()
			if len(m.fields) > 0 {
				m.fields[0].Blur()
			}
			m.saveInputValues()
			m.fillFromSelectedRow()
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

		if m.wdButtonsFocused {
			return m.handleWdButtonsKey(msg)
		}

		if m.previewFocused {
			if m.previewEditing {
				return m.handlePreviewEditKey(msg)
			}
			return m.handlePreviewKey(msg)
		}

		return m.handleFormKey(msg)
	}

	if !m.historyFocused && !m.workingDirFocused && !m.wdButtonsFocused && !m.previewFocused && m.buttonFocus == 0 && len(m.fields) > 0 {
		if !m.fields[m.focused].isSelect {
			var cmd tea.Cmd
			m.fields[m.focused].input, cmd = m.fields[m.focused].input.Update(msg)
			m.refreshPreview()
			return m, cmd
		}
	}

	if m.workingDirFocused {
		var cmd tea.Cmd
		m.workingDirInput, cmd = m.workingDirInput.Update(msg)
		return m, cmd
	}

	if m.previewEditing {
		var cmd tea.Cmd
		m.previewTextarea, cmd = m.previewTextarea.Update(msg)
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
			m.wdButtonsFocused = true
			return m, nil
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
		return m, func() tea.Msg {
			return PlaceholderFormDoneMsg{values: values, workingDir: workingDir, commandOverride: m.commandOverride}
		}

	case "tab":
		m.restoreInputValues()
		m.historyFocused = false
		if m.showWorkingDir {
			m.wdButtonsFocused = true
			return m, nil
		}
		if len(m.fields) > 0 {
			return m, m.fields[0].Focus()
		}
		m.buttonFocus = 1
		return m, nil
	}
	return m, nil
}

func (m PlaceholderFormModel) handleWdButtonsKey(msg tea.KeyMsg) (PlaceholderFormModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }

	case "left", "h", "right", "l":
		m.wdChoice = 1 - m.wdChoice
		return m, nil

	case "tab", "down", "enter":
		m.wdButtonsFocused = false
		if m.wdChoice == 1 {
			m.workingDirFocused = true
			return m, m.workingDirInput.Focus()
		}
		if len(m.fields) > 0 {
			m.focused = 0
			return m, m.fields[0].Focus()
		}
		m.buttonFocus = 1
		return m, nil

	case "shift+tab", "up":
		m.wdButtonsFocused = false
		if m.historyLoaded && len(m.historyRecords) > 0 {
			m.historyFocused = true
			return m, nil
		}
		m.buttonFocus = 2
		return m, nil
	}
	return m, nil
}

func (m PlaceholderFormModel) handleWorkingDirKey(msg tea.KeyMsg) (PlaceholderFormModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }

	case "tab", "down", "enter":
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
		m.wdButtonsFocused = true
		return m, nil

	default:
		var cmd tea.Cmd
		m.workingDirInput, cmd = m.workingDirInput.Update(msg)
		return m, cmd
	}
}

func (m PlaceholderFormModel) handlePreviewKey(msg tea.KeyMsg) (PlaceholderFormModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.cancelled = true
		return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }

	case "enter":
		m.previewEditing = true
		m.previewTextarea.SetValue(m.previewText)
		m.previewTextarea.SetWidth(m.viewport.Width)
		m.previewTextarea.SetHeight(max(3, m.viewport.Height))
		m.previewTextarea.CursorEnd()
		return m, m.previewTextarea.Focus()

	case "tab", "down":
		return m.focusTopSection()

	case "shift+tab", "up":
		m.previewFocused = false
		m.buttonFocus = 2
		return m, nil
	}
	return m, nil
}

// handlePreviewEditKey handles keys while the preview pane is an editable
// textarea. Esc here only discards the in-progress edit and drops back to a
// read-only, focused preview - unlike everywhere else in this form, it does
// NOT cancel the whole execution, since throwing away a typo fix shouldn't
// cost the user the entire form.
func (m PlaceholderFormModel) handlePreviewEditKey(msg tea.KeyMsg) (PlaceholderFormModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, func() tea.Msg { return PlaceholderFormDoneMsg{cancelled: true} }

	case "esc":
		m.previewTextarea.Blur()
		m.previewEditing = false
		return m, nil

	case "tab":
		m.commandOverride = m.previewTextarea.Value()
		m.previewTextarea.Blur()
		m.previewEditing = false
		m.refreshPreview()
		return m.focusTopSection()

	case "shift+tab":
		m.commandOverride = m.previewTextarea.Value()
		m.previewTextarea.Blur()
		m.previewEditing = false
		m.refreshPreview()
		m.previewFocused = false
		m.buttonFocus = 2
		return m, nil
	}

	var cmd tea.Cmd
	m.previewTextarea, cmd = m.previewTextarea.Update(msg)
	return m, cmd
}

// focusTopSection wraps focus back around to the first section of the form
// (history, then working-dir buttons, then the first field, then Execute),
// used both from the bottom of the field/button chain and from the preview
// pane at the very end.
func (m PlaceholderFormModel) focusTopSection() (PlaceholderFormModel, tea.Cmd) {
	m.previewFocused = false
	if m.historyLoaded && len(m.historyRecords) > 0 {
		m.historyFocused = true
		return m, nil
	}
	if m.showWorkingDir {
		m.wdButtonsFocused = true
		return m, nil
	}
	if len(m.fields) > 0 {
		m.focused = 0
		return m, m.fields[0].Focus()
	}
	m.buttonFocus = 1
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
			m.refreshPreview()
			return m, nil
		case "k", "up", "h", "ctrl+p":
			idx := m.fields[m.focused].picker.Index()
			if idx > 0 {
				m.fields[m.focused].picker.Select(idx - 1)
			}
			m.refreshPreview()
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
			workingDir := m.resolvedWorkingDir()
			override := m.commandOverride
			return m, func() tea.Msg {
				return PlaceholderFormDoneMsg{values: values, workingDir: workingDir, commandOverride: override}
			}
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
		if m.buttonFocus == 0 && (len(m.fields) == 0 || m.focused == 0) {
			if len(m.fields) > 0 {
				m.fields[m.focused].Blur()
			}
			if m.showWorkingDir {
				m.wdButtonsFocused = true
				return m, nil
			}
			if m.historyLoaded && len(m.historyRecords) > 0 {
				m.saveInputValues()
				m.historyFocused = true
				return m, nil
			}
			m.previewFocused = true
			return m, nil
		}
		return m.prevFocus()

	default:
		if m.buttonFocus == 0 && len(m.fields) > 0 && !m.fields[m.focused].isSelect {
			var cmd tea.Cmd
			m.fields[m.focused].input, cmd = m.fields[m.focused].input.Update(msg)
			m.refreshPreview()
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
		// Cancel button → forward into the preview pane.
		m.buttonFocus = 0
		m.previewFocused = true
		return m, nil
	}
}

func (m PlaceholderFormModel) prevFocus() (PlaceholderFormModel, tea.Cmd) {
	if m.buttonFocus == 0 {
		if len(m.fields) > 0 && m.focused > 0 {
			m.fields[m.focused].Blur()
			m.focused--
			return m, m.fields[m.focused].Focus()
		}
		return m, nil

	} else if m.buttonFocus == 2 {
		m.buttonFocus = 1
		return m, nil
	} else {
		// Execute button → go back to the last field, wd buttons, history,
		// or wrap around to the preview pane if none of those exist.
		m.buttonFocus = 0
		if len(m.fields) > 0 {
			m.focused = len(m.fields) - 1
			return m, m.fields[m.focused].Focus()
		}
		if m.showWorkingDir {
			m.wdButtonsFocused = true
			return m, nil
		}
		if m.historyLoaded && len(m.historyRecords) > 0 {
			m.saveInputValues()
			m.historyFocused = true
			return m, nil
		}
		m.previewFocused = true
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

		cwdLabel := "Execute in CWD"
		diffLabel := "Execute in different directory"
		if m.wdChoice == 0 {
			cwdLabel = "● " + cwdLabel
			diffLabel = "○ " + diffLabel
		} else {
			cwdLabel = "○ " + cwdLabel
			diffLabel = "● " + diffLabel
		}

		cwdStyle := PrimaryButtonStyle
		diffStyle := PrimaryButtonStyle
		if m.wdButtonsFocused {
			if m.wdChoice == 0 {
				cwdStyle = PrimaryButtonFocusedStyle
			} else {
				diffStyle = PrimaryButtonFocusedStyle
			}
		}

		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, cwdStyle.Render(cwdLabel), diffStyle.Render(diffLabel)))
		b.WriteString("\n")

		if m.wdChoice == 1 {
			wdFieldStyle := PlaceholderInputStyle
			if m.workingDirFocused {
				wdFieldStyle = PlaceholderInputFocusedStyle
			}
			b.WriteString(wdFieldStyle.Render(m.workingDirInput.View()))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	for i, placeholder := range m.placeholders {
		b.WriteString(FieldLabelStyle.Render(placeholder.Label))
		b.WriteString("\n")

		field := m.fields[i]
		focused := i == m.focused && m.buttonFocus == 0 && !m.historyFocused && !m.workingDirFocused && !m.wdButtonsFocused && !m.previewFocused

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

	if m.buttonFocus == 1 && !m.previewFocused {
		executeStyle = PrimaryButtonFocusedStyle
	}
	if m.buttonFocus == 2 && !m.previewFocused {
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
	} else if m.wdButtonsFocused {
		instructions = "←/→: Toggle • Tab: Next • Shift+Tab: Prev • Esc: Cancel"
	} else if m.workingDirFocused {
		instructions = "Tab: Next • Shift+Tab: Prev • ctrl+u: Use cwd • Esc: Cancel"
	} else if m.previewEditing {
		instructions = "Tab/Shift+Tab: Save edit & move • Esc: Discard edit • Ctrl+C: Cancel"
	} else if m.previewFocused {
		instructions = "Enter: Edit command • Tab: Next • Shift+Tab: Prev • Esc: Cancel"
	} else if m.buttonFocus == 0 && len(m.fields) > 0 && m.fields[m.focused].isSelect {
		instructions = "j/k: Select • Tab: Next • Shift+Tab: Prev • Enter: Submit • Esc: Cancel"
	}
	b.WriteString(InstructionStyle.Render(instructions))

	formWidth := m.width - 4
	if formWidth < 20 {
		formWidth = 20
	}
	formPane := PreviewStyle.Width(formWidth).Render(b.String())

	var historySection string
	if m.historyLoaded && len(m.historyRecords) > 0 {
		tableTitle := PreviewTitleStyle.Render("Recent Executions")
		historySection = lipgloss.JoinVertical(lipgloss.Left, tableTitle, m.historyTable.View())
	}

	vpHeight := m.height - lipgloss.Height(formPane) - lipgloss.Height(historySection) - 4
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.Height = vpHeight

	previewTitle := PreviewTitleStyle.Render("Preview")
	previewPaneStyle := PreviewStyle
	if m.previewFocused {
		previewPaneStyle = PreviewFocusedStyle
	}

	var previewBody string
	if m.previewEditing {
		m.previewTextarea.SetWidth(formWidth - 4)
		m.previewTextarea.SetHeight(vpHeight)
		previewBody = m.previewTextarea.View()
	} else {
		previewBody = m.viewport.View()
	}
	previewPane := previewPaneStyle.Width(formWidth).Render(previewTitle + "\n" + previewBody)

	log.Printf("PlaceholderForm View - Width: %d, Height: %d, FormWidth: %d, ViewportWidth: %d, ViewportHeight: %d", m.width, m.height, formWidth, m.viewport.Width, m.viewport.Height)

	if historySection != "" {
		return lipgloss.JoinVertical(lipgloss.Left, historySection, formPane, previewPane)
	}

	return lipgloss.JoinVertical(lipgloss.Left, formPane, previewPane)
}

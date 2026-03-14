package tui

import (
	"log"
	"scripto/entities"
	"scripto/internal/utils"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func (m *MainListScreen) renderList(maxWidth, maxHeight int) string {
	totalVerticalBorder := 2
	totalHorizontalBorder := 2
	maxListItemWidth := maxWidth - totalHorizontalBorder
	scripts := m.activeScripts()
	if len(scripts) == 0 {
		emptyMsg := "No scripts found.\nUse 'scripto add' to create some scripts."
		return ListStyle.
			Width(maxWidth).
			Height(maxHeight).
			Render(emptyMsg)
	}
	// return ListStyle.
	// 	Width(width).
	// 	Height(height).
	// 	Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9900")).Bold(false).Render("mytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nmytext \n my text \nytext \n my text \n"))

	listItems := m.buildListItems()
	var items []string

	for i, item := range listItems {
		if item.script == nil {
			var header string
			if i == m.selectedItemIndex && item.isSelectableHeader() {
				header = ListItemSelectedStyle.Width(maxListItemWidth - 4).Render(scopeHeaderRawText(item.scope))
			} else {
				scopeType := getScopeType(item.scope)
				if scopeType == "other" {
					header = formatOtherScopeHeader(item.scope)
				} else {
					header = formatScopeHeader(item.scope)
				}
			}
			items = append(items, header)
		} else {
			items = append(items, m.formatScriptItem(*item.script, i, maxListItemWidth, 3))
		}
	}

	content := strings.Join(items, "\n")

	maxPossibleContentHeight := max(1, maxHeight-totalVerticalBorder)

	var start, end int
	if len(items) > maxPossibleContentHeight {
		start, end = calculateScrollWindow(m.selectedItemIndex, len(items), maxPossibleContentHeight)
		items = items[start:end]
		content = strings.Join(items, "\n")
	}

	log.Printf("renderList - maxWidth: %v, maxHeight: %v, len(m.scripts): %v, maxHeight-totalVerticalBorder: %v, maxPossibleContentHeight: %v, start: %v, end: %v", maxWidth, maxHeight, len(m.scripts), maxHeight-totalVerticalBorder, maxPossibleContentHeight, start, end)

	style := ListStyle
	if m.focusedPane == "list" {
		style = ListFocusedStyle
	}

	style = style.
		Width(maxListItemWidth).
		MaxWidth(maxWidth).
		Height(maxHeight - totalVerticalBorder).
		MaxHeight(maxHeight)

	rendered := style.Render(content)
	// renderedHeight := lipgloss.Height(rendered)

	// for renderedHeight > contentHeight && len(lines) > 1 {
	// 	lines = lines[:len(lines)-1]
	// 	content = strings.Join(lines, "\n")
	// 	rendered = style.Render(content)
	// 	renderedHeight = lipgloss.Height(rendered)
	// }

	return rendered
}

func (m *MainListScreen) formatScriptItem(script entities.Script, index int, maxWidth int, indent int) string {
	var parts []string

	scopeIndicator := FormatScopeIndicator(script.Scope)
	parts = append(parts, scopeIndicator)

	var displayName string
	if script.Name != "" {
		displayName = script.Name
	} else {
		displayName = utils.TruncateString(script.FilePath, 60)
	}

	if utf8.RuneCountInString(displayName) > maxWidth {
		displayName = utils.TruncateString(displayName, (maxWidth-4-indent)) + "…"
	}

	item := ListItemStyle.Bold(false).Width(maxWidth - 4).Render(displayName)

	if index == m.selectedItemIndex {
		item = ListItemSelectedStyle.Width(maxWidth - 4).Render(displayName)
	}

	parts = append(parts, item)

	// return lipgloss.NewStyle().Width(maxWidth-4).Background(lipgloss.Color("#ff9900")).Render(strings.Join(parts, " "))
	return strings.Join(parts, " ")
	// return lipgloss.NewStyle().Width(maxWidth-4).Background(lipgloss.Color("#ff9900")).Render()
}

func formatScopeHeader(scope string) string {
	var header string
	scopeType := getScopeType(scope)
	style := GetScopeStyle(scopeType)

	switch scopeType {
	case "local":
		header = "● " + formatDirectoryName(scope)
	case "parent":
		header = "◐ " + formatDirectoryName(scope)
	case "global":
		header = "○ Global Scripts"
	default:
		header = formatDirectoryName(scope)
	}

	return style.Bold(true).Render(header)
}

func calculateScrollWindow(selectedLine, totalLines, visibleHeight int) (int, int) {
	halfWindow := visibleHeight / 2
	start := selectedLine - halfWindow
	end := selectedLine + halfWindow

	if start < 0 {
		start = 0
	}
	if end > totalLines {
		end = totalLines
	}
	if end-start < visibleHeight && totalLines > visibleHeight {
		if start > 0 {
			start = end - visibleHeight
		} else {
			end = start + visibleHeight
		}
	}
	if start < 0 {
		start = 0
	}

	return start, end
}

func scopeHeaderRawText(scope string) string {
	scopeType := getScopeType(scope)
	switch scopeType {
	case "local":
		return "● " + formatDirectoryName(scope)
	case "parent":
		return "◐ " + formatDirectoryName(scope)
	case "other":
		return "◌ " + formatDirectoryName(scope)
	default:
		return formatDirectoryName(scope)
	}
}

func formatOtherScopeHeader(scope string) string {
	return lipgloss.NewStyle().Foreground(mutedTextColor).Bold(true).Render("◌ " + formatDirectoryName(scope))
}

func formatDirectoryName(dir string) string {
	if dir == "global" {
		return "Global Scripts"
	}

	fullPath := dir

	if len(fullPath) > 100 {
	}

	return fullPath
}


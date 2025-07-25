package tui

import (
	"strings"

	"scripto/internal/script"
)

// renderList renders the script list pane
func (m MainModel) renderList(width, height int) string {
	if len(m.scripts) == 0 {
		emptyMsg := "No scripts found.\nUse 'scripto add' to create some scripts."
		return ListStyle.
			Width(width).
			Height(height).
			Render(emptyMsg)
	}

	var items []string
	var currentScope string

	for i, script := range m.scripts {
		// Add scope header if scope changes
		if script.Script.Scope != currentScope {
			if currentScope != "" {
				items = append(items, "") // Add spacing between scopes
			}

			scopeHeader := formatScopeHeader(script.Script.Scope)
			items = append(items, scopeHeader)
			currentScope = script.Script.Scope
		}

		// Format script item
		item := m.formatScriptItem(script, i)
		items = append(items, item)
	}

	// Join all items
	content := strings.Join(items, "\n")

	// Calculate available height for scrolling
	visibleHeight := height - 2 // Account for borders

	// Simple scrolling: show a window around the selected item
	lines := strings.Split(content, "\n")
	if len(lines) > visibleHeight {
		start, end := m.calculateScrollWindow(lines, visibleHeight)
		content = strings.Join(lines[start:end], "\n")
	}

	// Apply list styling
	style := ListStyle.Width(width).Height(height)

	// Highlight focused pane
	if m.focusedPane == "list" {
		style = style.BorderForeground(primaryColor)
	}

	return style.Render(content)
}

// formatScriptItem formats a single script item for display
func (m MainModel) formatScriptItem(script script.MatchResult, index int) string {
	var parts []string

	// Add scope indicator
	scopeIndicator := FormatScopeIndicator(script.Script.Scope)
	parts = append(parts, scopeIndicator)

	// Add script name or command
	var displayName string
	if script.Script.Name != "" {
		displayName = script.Script.Name
	} else {
		// Show truncated file path for unnamed scripts (Command field removed)
		displayName = truncateString(script.Script.FilePath, 60)
	}

	parts = append(parts, displayName)

	item := strings.Join(parts, " ")

	// Apply selection styling
	if index == m.selectedIdx {
		return SelectedItemStyle.Render(item)
	}

	return ItemStyle.Render(item)
}

// formatScopeHeader formats a scope section header with directory name
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

// formatDirectoryName formats a directory path for display with full paths
func formatDirectoryName(dir string) string {
	if dir == "global" {
		return "Global Scripts"
	}
	
	// Use the full absolute path
	fullPath := dir
	
	// Truncate from the left if longer than 100 characters
	if len(fullPath) > 100 {
		return "..." + fullPath[len(fullPath)-97:] // 97 + 3 ("...") = 100
	}
	
	return fullPath
}

// calculateScrollWindow calculates which lines to show for scrolling
func (m MainModel) calculateScrollWindow(lines []string, visibleHeight int) (int, int) {
	// Find the line index of the selected item
	selectedLine := m.findSelectedLine(lines)

	// Calculate scroll window
	halfWindow := visibleHeight / 2
	start := selectedLine - halfWindow
	end := selectedLine + halfWindow

	// Adjust bounds
	if start < 0 {
		start = 0
		end = visibleHeight
	}
	if end > len(lines) {
		end = len(lines)
		start = end - visibleHeight
		if start < 0 {
			start = 0
		}
	}

	return start, end
}

// findSelectedLine finds the line index of the currently selected script
func (m MainModel) findSelectedLine(lines []string) int {
	// This is a simplified approach - in a real implementation,
	// we'd need to track which line corresponds to which script
	// For now, estimate based on selected index

	// Count scope headers and estimate position
	scopeHeaders := 0
	for i := 0; i <= m.selectedIdx && i < len(m.scripts); i++ {
		if i == 0 || m.scripts[i].Script.Scope != m.scripts[i-1].Script.Scope {
			scopeHeaders++
		}
	}

	// Rough estimate: selected index + scope headers + spacing
	estimatedLine := m.selectedIdx + scopeHeaders
	if estimatedLine >= len(lines) {
		estimatedLine = len(lines) - 1
	}

	return estimatedLine
}

// truncateString truncates a string to the specified length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

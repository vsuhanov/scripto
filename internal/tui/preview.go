package tui

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"scripto/internal/script"
)

// renderPreview renders the script preview pane
func (m Model) renderPreview(width, height int) string {
	if len(m.scripts) == 0 {
		emptyMsg := "No script selected"
		style := PreviewStyle.Width(width).Height(height)

		if m.focusedPane == "preview" {
			style = style.BorderForeground(primaryColor)
		}

		return style.Render(emptyMsg)
	}

	selected := m.scripts[m.selectedIdx]
	content := m.formatPreviewContent(selected, width-4) // Account for padding

	style := PreviewStyle.Width(width).Height(height)

	// Highlight focused pane
	if m.focusedPane == "preview" {
		style = style.BorderForeground(primaryColor)
	}

	return style.Render(content)
}

// formatPreviewContent formats the content for the preview pane
func (m Model) formatPreviewContent(selected script.MatchResult, maxWidth int) string {
	var sections []string

	// Script title
	title := m.formatPreviewTitle(selected)
	sections = append(sections, title)

	// Script metadata
	metadata := m.formatPreviewMetadata(selected)
	sections = append(sections, metadata)

	// Command section removed - no longer displayed

	// Placeholders
	if len(selected.Script.Placeholders) > 0 {
		placeholdersSection := m.formatPreviewPlaceholders(selected.Script.Placeholders)
		sections = append(sections, placeholdersSection)
	}

	// Description
	if selected.Script.Description != "" {
		descSection := m.formatPreviewDescription(selected.Script.Description, maxWidth)
		sections = append(sections, descSection)
	}

	// File content (if available)
	if selected.Script.FilePath != "" {
		fileSection := m.formatPreviewFileContent(selected.Script.FilePath, maxWidth)
		if fileSection != "" {
			sections = append(sections, fileSection)
		}
	}

	return strings.Join(sections, "\n\n")
}

// formatPreviewTitle formats the script title
func (m Model) formatPreviewTitle(selected script.MatchResult) string {
	scopeIndicator := FormatScopeIndicator(selected.Scope)

	var title string
	if selected.Script.Name != "" {
		title = selected.Script.Name
	} else {
		title = "Unnamed Script"
	}

	return PreviewTitleStyle.Render(fmt.Sprintf("%s %s", scopeIndicator, title))
}

// formatPreviewMetadata formats script metadata
func (m Model) formatPreviewMetadata(selected script.MatchResult) string {
	var metadata []string

	// Scope
	metadata = append(metadata, fmt.Sprintf("Scope: %s", selected.Scope))

	// Directory (for non-global scripts)
	if selected.Scope != "global" {
		dir := selected.Directory
		if len(dir) > 50 {
			dir = "..." + dir[len(dir)-47:]
		}
		metadata = append(metadata, fmt.Sprintf("Directory: %s", dir))
	}

	// File path
	if selected.Script.FilePath != "" {
		filename := filepath.Base(selected.Script.FilePath)
		metadata = append(metadata, fmt.Sprintf("File: %s", filename))
	}

	return PreviewContentStyle.Render(strings.Join(metadata, "\n"))
}

// formatPreviewCommand formats the script command (DEPRECATED - Command section removed)
func (m Model) formatPreviewCommand(command string, maxWidth int) string {
	// This function is deprecated and should not be used
	// The Command: section has been removed from the preview
	return ""
}

// formatPreviewPlaceholders formats the script placeholders
func (m Model) formatPreviewPlaceholders(placeholders []string) string {
	if len(placeholders) == 0 {
		return ""
	}

	title := PreviewTitleStyle.Render("Placeholders:")

	var items []string
	for _, placeholder := range placeholders {
		items = append(items, "  • "+placeholder)
	}

	content := PreviewContentStyle.Render(strings.Join(items, "\n"))
	return title + "\n" + content
}

// formatPreviewDescription formats the script description
func (m Model) formatPreviewDescription(description string, maxWidth int) string {
	title := PreviewTitleStyle.Render("Description:")

	wrappedDesc := wrapText(description, maxWidth)
	content := PreviewContentStyle.Render(wrappedDesc)

	return title + "\n" + content
}

// formatPreviewFileContent formats the script file content preview
func (m Model) formatPreviewFileContent(filePath string, maxWidth int) string {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return PreviewContentStyle.Render(fmt.Sprintf("Error reading file: %v", err))
	}

	title := PreviewTitleStyle.Render("File Content:")

	// Limit preview to first 10 lines
	lines := strings.Split(string(content), "\n")
	if len(lines) > 10 {
		lines = lines[:10]
		lines = append(lines, "...")
	}

	// Wrap long lines
	var wrappedLines []string
	for _, line := range lines {
		if len(line) > maxWidth {
			wrapped := strings.Split(wrapText(line, maxWidth), "\n")
			wrappedLines = append(wrappedLines, wrapped...)
		} else {
			wrappedLines = append(wrappedLines, line)
		}
	}

	fileContent := strings.Join(wrappedLines, "\n")
	styledContent := PreviewCommandStyle.Render(fileContent)

	return title + "\n" + styledContent
}

// wrapText wraps text to the specified width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine string

	for _, word := range words {
		// If adding this word would exceed the width, start a new line
		if len(currentLine)+len(word)+1 > width && currentLine != "" {
			lines = append(lines, currentLine)
			currentLine = word
		} else if currentLine == "" {
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}

	// Add the last line
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n")
}

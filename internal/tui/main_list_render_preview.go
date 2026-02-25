package tui

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"scripto/entities"
	. "scripto/internal/utils"
)

func (m *MainListScreen) renderPreview(maxWidth, maxHeight int) string {
	totalVerticalBorder := 2
	totalHorizontalBorder := 2

	previewStyle := PreviewStyle
	if m.focusedPane == "preview" {
		previewStyle = PreviewFocusedStyle
	}

	log.Printf("renderPreview - maxWidth: %v, maxHeight: %v", maxWidth, maxHeight)

	previewStyle = previewStyle.
		Width(maxWidth - totalHorizontalBorder).
		MaxWidth(maxWidth).
		Height(maxHeight - totalVerticalBorder).
		MaxHeight(maxHeight)

	rendered := previewStyle.Render("Preview")
	log.Printf("renderPreview - rendered - rendered.Width: %v, rendered.Height: %v", lipgloss.Width(rendered), lipgloss.Height(rendered))

	return rendered
}

func (m *MainListScreen) formatPreviewContent(script *entities.Script) string {
	var sections []string

	title := m.formatPreviewTitle(script)
	sections = append(sections, title)

	metadata := m.formatPreviewMetadata(script)
	sections = append(sections, metadata)

	if script.Description != "" {
		description := m.formatPreviewDescription(script.Description, m.maxWidth)
		sections = append(sections, description)
	}

	if script.FilePath != "" {
		fileContent := m.formatPreviewFileContent(script.FilePath, m.maxWidth)
		sections = append(sections, fileContent)
	}

	return strings.Join(sections, "\n\n")
}

func (m *MainListScreen) formatPreviewTitle(selected *entities.Script) string {
	scopeIndicator := FormatScopeIndicator(selected.Scope)

	var title string
	if selected.Name != "" {
		title = selected.Name
	} else {
		title = "Unnamed Script"
	}

	return PreviewTitleStyle.Render(fmt.Sprintf("%s %s", scopeIndicator, title))
}

func (m *MainListScreen) formatPreviewMetadata(selected *entities.Script) string {
	var metadata []string

	if selected.Scope == "global" {
		metadata = append(metadata, "Scope: global")
	} else {
		scopeLabel := m.getScopeDisplayName(selected.Scope)
		metadata = append(metadata, fmt.Sprintf("Scope: %s", scopeLabel))

		dir := selected.Scope
		if len(dir) > 50 {
			dir = "..." + dir[len(dir)-47:]
		}
		metadata = append(metadata, fmt.Sprintf("Directory: %s", dir))
	}

	if selected.FilePath != "" {
		filename := filepath.Base(selected.FilePath)
		metadata = append(metadata, fmt.Sprintf("File: %s", filename))
	}

	return PreviewContentStyle.Render(strings.Join(metadata, "\n"))
}

func (m *MainListScreen) formatPreviewDescription(description string, maxWidth int) string {
	title := PreviewTitleStyle.Render("Description:")
	wrappedDesc := WrapText(description, maxWidth)
	content := PreviewContentStyle.Render(wrappedDesc)
	return title + "\n" + content
}

func (m *MainListScreen) formatPreviewFileContent(filePath string, maxWidth int) string {
	content, err := readScriptFile(filePath)
	if err != nil {
		return PreviewContentStyle.Render(fmt.Sprintf("Error reading file: %v", err))
	}

	title := PreviewTitleStyle.Render("File Content:")

	lines := strings.Split(content, "\n")
	if len(lines) > 10 {
		lines = lines[:10]
		lines = append(lines, "...")
	}

	var wrappedLines []string
	for _, line := range lines {
		if len(line) > maxWidth {
			wrapped := strings.Split(WrapText(line, maxWidth), "\n")
			wrappedLines = append(wrappedLines, wrapped...)
		} else {
			wrappedLines = append(wrappedLines, line)
		}
	}

	fileContent := strings.Join(wrappedLines, "\n")
	styledContent := PreviewCommandStyle.Render(fileContent)

	return title + "\n" + styledContent
}

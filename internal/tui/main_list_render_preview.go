package tui

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"scripto/entities"
	. "scripto/internal/utils"
	"scripto/internal/tui/colors"
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

	rendered := previewStyle.Render(m.formatPreviewContent(m.selectedScript))
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
		fileContent := m.formatPreviewFileContent()
		sections = append(sections, fileContent)
	}

	return strings.Join(sections, "\n")
}

func (m *MainListScreen) formatPreviewTitle(selected *entities.Script) string {
	scopeIndicator := FormatScopeIndicator(selected.Scope)

	var title string
	if selected.Name != "" {
		title = selected.Name
	} else {
		title = "Unnamed Script"
	}

	style := PreviewTitleStyle
	if m.previewNavMode && m.previewFocusedElement == previewFocusName {
		style = PreviewTitleStyle.Background(colors.SelectedBackground).Foreground(colors.SelectedText)
	}

	return style.Render(fmt.Sprintf("%s %s", scopeIndicator, title))
}

func (m *MainListScreen) formatPreviewMetadata(selected *entities.Script) string {
	var metadata []string
	var dirLine string

	if selected.Scope == "global" {
		metadata = append(metadata, "Scope: global")
	} else {
		scopeLabel := m.getScopeDisplayName(selected.Scope)
		metadata = append(metadata, fmt.Sprintf("Scope: %s", scopeLabel))

		dir := selected.Scope
		if len(dir) > 50 {
			dir = "..." + dir[len(dir)-47:]
		}
		dirLine = fmt.Sprintf("Directory: %s", dir)
		if m.previewNavMode && m.previewFocusedElement == previewFocusDirectory {
			dirStyle := PreviewContentStyle.Background(colors.SelectedBackground).Foreground(colors.SelectedText)
			metadata = append(metadata, dirStyle.Render(dirLine))
		} else {
			metadata = append(metadata, dirLine)
		}
	}

	if selected.FilePath != "" {
		filename := filepath.Base(selected.FilePath)
		metadata = append(metadata, fmt.Sprintf("File: %s", filename))
	}

	if selected.ID != "" && m.scriptStats != nil {
		if stats, ok := m.scriptStats[selected.ID]; ok && stats.ExecutionCount > 0 {
			lastRun := stats.LastExecutionTime.Format(time.RFC822)
			metadata = append(metadata, fmt.Sprintf("Last run: %s", lastRun))
			metadata = append(metadata, fmt.Sprintf("Runs: %d", stats.ExecutionCount))
		}
	}

	return PreviewContentStyle.Render(strings.Join(metadata, "\n"))
}

func (m *MainListScreen) formatPreviewDescription(description string, maxWidth int) string {
	title := PreviewTitleStyle.Render("Description:")
	wrappedDesc := WrapText(description, maxWidth)
	content := PreviewContentStyle.Render(wrappedDesc)
	return title + "\n" + content
}

func (m *MainListScreen) formatPreviewFileContent() string {
	labelStyle := PreviewTitleStyle
	if m.previewNavMode && m.previewFocusedElement == previewFocusViewport {
		labelStyle = PreviewTitleStyle.Background(colors.SelectedBackground).Foreground(colors.SelectedText)
	}
	title := labelStyle.Render("File Content:")
	return title + "\n" + m.previewViewport.View()
}

func (m *MainListScreen) getScopeDisplayName(scope string) string {
	if m.container != nil {
		return m.container.ScriptService.GetScopeDisplayName(scope)
	}
	return scope
}



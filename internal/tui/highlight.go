package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderHighlighted(text string, re *regexp.Regexp, base, highlight lipgloss.Style) string {
	if re == nil || text == "" {
		return base.Render(text)
	}

	matches := re.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return base.Render(text)
	}

	var sb strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if end <= start {
			continue
		}
		if start > last {
			sb.WriteString(base.Render(text[last:start]))
		}
		sb.WriteString(highlight.Render(text[start:end]))
		last = end
	}
	if last == 0 {
		return base.Render(text)
	}
	if last < len(text) {
		sb.WriteString(base.Render(text[last:]))
	}
	return sb.String()
}

func highlightStyles(selected bool) (base, highlight lipgloss.Style) {
	base = lipgloss.NewStyle()
	highlight = lipgloss.NewStyle().Foreground(warningColor).Bold(true)
	if selected {
		base = base.Foreground(selectedTextColor).Background(selectedBgColor).Bold(true)
		highlight = highlight.Background(selectedBgColor)
	}
	return base, highlight
}

package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Base colors
	primaryColor   = lipgloss.Color("#6366f1")
	secondaryColor = lipgloss.Color("#64748b")
	accentColor    = lipgloss.Color("#06b6d4")
	errorColor     = lipgloss.Color("#ef4444")
	successColor   = lipgloss.Color("#10b981")
	warningColor   = lipgloss.Color("#f59e0b")

	// Background colors
	bgColor         = lipgloss.Color("#1e293b")
	selectedBgColor = lipgloss.Color("#334155")
	borderColor     = lipgloss.Color("#475569")

	// Text colors
	textColor         = lipgloss.Color("#f8fafc")
	mutedTextColor    = lipgloss.Color("#94a3b8")
	selectedTextColor = lipgloss.Color("#ffffff")

	// Main container style
	ContainerStyle = lipgloss.NewStyle().
			Padding(1).
			Margin(0).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)

	// List styles
	ListStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

	SelectedItemStyle = lipgloss.NewStyle().
				Background(selectedBgColor).
				Foreground(selectedTextColor).
				Bold(true).
				Padding(0, 1)

	ItemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	// Preview pane styles
	PreviewStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1)

	PreviewTitleStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Margin(0, 0, 1, 0)

	PreviewContentStyle = lipgloss.NewStyle().
				Foreground(textColor)

	PreviewCommandStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Background(lipgloss.Color("#0f172a")).
				Padding(0, 1).
				Margin(1, 0)

	// Scope indicator styles
	ScopeLocalStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	ScopeParentStyle = lipgloss.NewStyle().
				Foreground(warningColor).
				Bold(true)

	ScopeGlobalStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	// Help text styles
	HelpStyle = lipgloss.NewStyle().
			Foreground(mutedTextColor).
			Italic(true).
			Margin(1, 0, 0, 0)

	// Status bar styles
	StatusStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(lipgloss.Color("#ffffff")).
			Padding(0, 1)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Popup styles
	PopupStyle = lipgloss.NewStyle().
			Background(bgColor).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1).
			Margin(2)

	PopupTitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Align(lipgloss.Center).
			Margin(0, 0, 1, 0)

	// Form field styles
	FieldLabelStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Bold(true).
			Margin(0, 0, 0, 0)

	FieldInputStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#374151")).
			Padding(0, 1).
			Margin(0, 0, 1, 0)

	FieldInputFocusedStyle = lipgloss.NewStyle().
				Foreground(selectedTextColor).
				Background(selectedBgColor).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1).
				Margin(0, 0, 1, 0)

	TextAreaStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#374151")).
			Padding(1).
			Margin(0, 0, 1, 0)

	TextAreaFocusedStyle = lipgloss.NewStyle().
				Foreground(selectedTextColor).
				Background(selectedBgColor).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(primaryColor).
				Padding(1).
				Margin(0, 0, 1, 0)

	CheckboxStyle = lipgloss.NewStyle().
			Foreground(textColor)

	CheckboxCheckedStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)
)

// GetScopeStyle returns the appropriate style for a script scope
func GetScopeStyle(scope string) lipgloss.Style {
	switch scope {
	case "local":
		return ScopeLocalStyle
	case "parent":
		return ScopeParentStyle
	case "global":
		return ScopeGlobalStyle
	default:
		return ItemStyle
	}
}

// FormatScopeIndicator returns a styled scope indicator
func FormatScopeIndicator(scope string) string {
	style := GetScopeStyle(scope)
	switch scope {
	case "local":
		return style.Render("●")
	case "parent":
		return style.Render("◐")
	case "global":
		return style.Render("○")
	default:
		return "●"
	}
}

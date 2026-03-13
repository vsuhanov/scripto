package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"scripto/internal/tui/colors"
)

var (
	// Base colors
	primaryColor   = colors.Primary
	secondaryColor = colors.Secondary
	accentColor    = colors.Accent
	errorColor     = colors.Error
	successColor   = colors.Success
	warningColor   = colors.Warning

	// Background colors
	bgColor         = colors.Background
	selectedBgColor = colors.SelectedBackground
	borderColor     = colors.Border

	// Text colors
	textColor         = colors.Text
	mutedTextColor    = colors.MutedText
	selectedTextColor = colors.SelectedText

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
			Foreground(selectedBgColor).
			Padding(0, 1)

	// Preview pane styles
	PreviewStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

	PreviewTitleStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Margin(0, 0, 1, 0)

	PreviewContentStyle = lipgloss.NewStyle().
				Foreground(textColor)

	PreviewCommandStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Background(colors.CommandBackground).
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
			Foreground(colors.White).
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
			Background(colors.InputBackground).
			Padding(0, 1).
			Margin(0, 0, 1, 0).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colors.InputBorder)

	FieldInputFocusedStyle = lipgloss.NewStyle().
				Foreground(selectedTextColor).
				Background(primaryColor).
				Padding(0, 1).
				Margin(0, 0, 1, 0).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colors.InputBorderFocused)

	TextAreaStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(colors.InputBackground).
			Padding(1).
			Margin(0, 0, 1, 0).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colors.InputBorder)

	TextAreaFocusedStyle = lipgloss.NewStyle().
				Foreground(selectedTextColor).
				Background(primaryColor).
				Padding(1).
				Margin(0, 0, 1, 0).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colors.InputBorderFocused)

	CheckboxStyle = lipgloss.NewStyle().
			Foreground(textColor)

	CheckboxCheckedStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	// Button styles
	PrimaryButtonStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Margin(0, 1).
				Background(colors.PrimaryButtonBackground).
				Foreground(colors.PrimaryButtonForeground)

	PrimaryButtonFocusedStyle = lipgloss.NewStyle().
					Padding(0, 2).
					Margin(0, 1).
					Background(colors.DangerButtonBackground).
					Foreground(colors.PrimaryButtonForeground)

	DangerButtonStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Margin(0, 1).
				Background(colors.DangerButtonBackground).
				Foreground(colors.DangerButtonForeground)

	DangerButtonFocusedStyle = lipgloss.NewStyle().
					Padding(0, 2).
					Margin(0, 1).
					Background(colors.DangerButtonBackground).
					Foreground(colors.DangerButtonForeground)

	// Form title style
	FormTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colors.Error).
			MarginBottom(1)

	// Description text style
	DescriptionStyle = lipgloss.NewStyle().
				Foreground(colors.MutedText).
				Italic(true)

	// Input styles for placeholders
	PlaceholderInputStyle = lipgloss.NewStyle().
				MarginBottom(1)
				// BorderStyle(lipgloss.RoundedBorder()).
				// BorderForeground(colors.InputBorder)

	PlaceholderInputFocusedStyle = lipgloss.NewStyle().
					MarginBottom(1)
					// BorderStyle(lipgloss.RoundedBorder()).
					// BorderForeground(colors.InputBorderFocused)

	// Instruction style
	InstructionStyle = lipgloss.NewStyle().
				Foreground(colors.MutedText).
				MarginTop(1)

	// History list item style
	HistoryItemStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	HistoryItemSelectedStyle = lipgloss.NewStyle().
					PaddingLeft(2).
					Background(selectedBgColor).
					Foreground(selectedTextColor).
					Bold(true)

	// Button container centering style
	ButtonContainerStyle = lipgloss.NewStyle().
				Align(lipgloss.Center)

	// Additional styles for main list screen
	TitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	LoadingStyle = lipgloss.NewStyle().
			Foreground(mutedTextColor).
			Align(lipgloss.Center)

	HeaderStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(colors.White).
			Padding(0, 1)

	ListFocusedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1)

	PreviewFocusedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1)

	FooterStyle = lipgloss.NewStyle().
			Background(borderColor).
			Foreground(textColor).
			Padding(0, 1)

	HelpScreenStyle = lipgloss.NewStyle().
			Padding(2).
			Background(bgColor).
			Foreground(textColor)

	ListItemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	ListItemSelectedStyle = lipgloss.NewStyle().
				Background(selectedBgColor).
				Foreground(selectedTextColor).
				Bold(true).
				Padding(0, 1)

	NoScriptsStyle = lipgloss.NewStyle().
			Foreground(mutedTextColor).
			Italic(true).
			Align(lipgloss.Center)
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

func FormatScopeIndicator(scope string) string {
	scopeType := getScopeType(scope)
	style := GetScopeStyle(scopeType)
	switch scopeType {
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

func getScopeType(scope string) string {
	if scope == "global" {
		return "global"
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "other"
	}

	if scope == cwd {
		return "local"
	}

	if strings.HasPrefix(cwd, scope+string(filepath.Separator)) {
		return "parent"
	}

	return "other"
}

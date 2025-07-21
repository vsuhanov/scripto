package tui

import "github.com/charmbracelet/lipgloss"

var Colors = struct {
	Primary   lipgloss.CompleteAdaptiveColor
	Secondary lipgloss.CompleteAdaptiveColor
	Accent    lipgloss.CompleteAdaptiveColor
	Error     lipgloss.CompleteAdaptiveColor
	Success   lipgloss.CompleteAdaptiveColor
	Warning   lipgloss.CompleteAdaptiveColor

	Background         lipgloss.CompleteAdaptiveColor
	SelectedBackground lipgloss.CompleteAdaptiveColor
	Border             lipgloss.CompleteAdaptiveColor
	InputBackground    lipgloss.CompleteAdaptiveColor
	CommandBackground  lipgloss.CompleteAdaptiveColor

	Text         lipgloss.CompleteAdaptiveColor
	MutedText    lipgloss.CompleteAdaptiveColor
	SelectedText lipgloss.CompleteAdaptiveColor
	White        lipgloss.CompleteAdaptiveColor

	InputBorder        lipgloss.CompleteAdaptiveColor
	InputBorderFocused lipgloss.CompleteAdaptiveColor

	ButtonBackground       lipgloss.CompleteAdaptiveColor
	ButtonForeground       lipgloss.CompleteAdaptiveColor
	PrimaryButtonBackground lipgloss.CompleteAdaptiveColor
	PrimaryButtonForeground lipgloss.CompleteAdaptiveColor
	PrimaryButtonBorder     lipgloss.CompleteAdaptiveColor
	DangerButtonBackground  lipgloss.CompleteAdaptiveColor
	DangerButtonForeground  lipgloss.CompleteAdaptiveColor
}{
	Primary: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#6366f1", ANSI256: "99", ANSI: "5"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#6366f1", ANSI256: "99", ANSI: "5"},
	},
	Secondary: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#64748b", ANSI256: "102", ANSI: "8"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#64748b", ANSI256: "102", ANSI: "8"},
	},
	Accent: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#06b6d4", ANSI256: "37", ANSI: "6"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#06b6d4", ANSI256: "37", ANSI: "6"},
	},
	Error: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#ef4444", ANSI256: "9", ANSI: "1"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#ef4444", ANSI256: "9", ANSI: "1"},
	},
	Success: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#10b981", ANSI256: "2", ANSI: "2"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#10b981", ANSI256: "2", ANSI: "2"},
	},
	Warning: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#f59e0b", ANSI256: "3", ANSI: "3"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#f59e0b", ANSI256: "3", ANSI: "3"},
	},

	Background: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#f8fafc", ANSI256: "15", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#1e293b", ANSI256: "0", ANSI: "0"},
	},
	SelectedBackground: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#e2e8f0", ANSI256: "7", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#334155", ANSI256: "8", ANSI: "8"},
	},
	Border: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#cbd5e1", ANSI256: "7", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#475569", ANSI256: "8", ANSI: "8"},
	},
	InputBackground: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#f1f5f9", ANSI256: "7", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#374151", ANSI256: "8", ANSI: "8"},
	},
	CommandBackground: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#e2e8f0", ANSI256: "7", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#0f172a", ANSI256: "0", ANSI: "0"},
	},

	Text: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#1e293b", ANSI256: "0", ANSI: "0"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#f8fafc", ANSI256: "15", ANSI: "7"},
	},
	MutedText: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#64748b", ANSI256: "8", ANSI: "8"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#94a3b8", ANSI256: "7", ANSI: "7"},
	},
	SelectedText: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#000000", ANSI256: "0", ANSI: "0"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#ffffff", ANSI256: "15", ANSI: "7"},
	},
	White: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#000000", ANSI256: "0", ANSI: "0"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#ffffff", ANSI256: "15", ANSI: "7"},
	},

	InputBorder: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#d1d5db", ANSI256: "7", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#dedede", ANSI256: "7", ANSI: "7"},
	},
	InputBorderFocused: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#3b82f6", ANSI256: "62", ANSI: "4"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#3b82f6", ANSI256: "62", ANSI: "4"},
	},

	ButtonBackground: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#f3f4f6", ANSI256: "7", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#374151", ANSI256: "8", ANSI: "8"},
	},
	ButtonForeground: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#1f2937", ANSI256: "0", ANSI: "0"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#f9fafb", ANSI256: "15", ANSI: "7"},
	},
	PrimaryButtonBackground: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#059669", ANSI256: "34", ANSI: "2"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#059669", ANSI256: "34", ANSI: "2"},
	},
	PrimaryButtonForeground: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#ffffff", ANSI256: "15", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#ffffff", ANSI256: "15", ANSI: "7"},
	},
	PrimaryButtonBorder: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#3b82f6", ANSI256: "62", ANSI: "4"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#3b82f6", ANSI256: "62", ANSI: "4"},
	},
	DangerButtonBackground: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#dc2626", ANSI256: "196", ANSI: "1"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#dc2626", ANSI256: "196", ANSI: "1"},
	},
	DangerButtonForeground: lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#ffffff", ANSI256: "15", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#ffffff", ANSI256: "15", ANSI: "7"},
	},
}
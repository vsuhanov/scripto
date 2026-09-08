package tui

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/vsuhanov/scripto/internal/services"
)

// Tests run without a TTY, where lipgloss degrades to the Ascii profile and
// emits no escape sequences at all - which would make these assertions pass
// trivially. Force a color profile so the rendering is actually exercised.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func exitCode(code int) *int { return &code }

func newRenderScreen() *ShellHistoryScreen {
	return &ShellHistoryScreen{width: 120, height: 40}
}

// Every rendered row must occupy the same number of terminal columns no matter
// how many highlight spans it contains, otherwise the columns tear.
func TestRenderRowWidthIsStable(t *testing.T) {
	s := newRenderScreen()
	records := []services.ShellHistoryRecord{
		{Command: "go build ./...", WorkingDirectory: "/tmp/project", StartedAt: 1, ExitCode: exitCode(0)},
		{Command: "日本語のコマンド 実行 テスト", WorkingDirectory: "/tmp/プロジェクト", StartedAt: 2, ExitCode: exitCode(1)},
		{Command: "echo 🎉🎉🎉 done", WorkingDirectory: "/tmp", StartedAt: 3},
		{Command: strings.Repeat("very-long-command ", 40), WorkingDirectory: "/tmp", StartedAt: 4},
		{Command: "", WorkingDirectory: "", StartedAt: 0},
	}

	for _, re := range []*regexp.Regexp{nil, regexp.MustCompile("(?i)o"), regexp.MustCompile("(?i).*")} {
		s.filterRe = re
		var want int
		for i, r := range records {
			for _, selected := range []bool{false, true} {
				got := lipgloss.Width(s.renderRow(r, selected))
				if i == 0 && !selected && re == nil {
					want = got
				}
				if want != 0 && got != want {
					t.Errorf("row %d (selected=%v, re=%v) width %d, want %d", i, selected, re, got, want)
				}
			}
		}
	}
}

func TestRenderRowWidthMatchesHeader(t *testing.T) {
	s := newRenderScreen()
	s.filterRe = regexp.MustCompile("(?i)build")

	header := strings.Split(s.renderHeaderRow(), "\n")[0]
	row := s.renderRow(services.ShellHistoryRecord{
		Command: "go build ./...", WorkingDirectory: "/tmp", StartedAt: 1, ExitCode: exitCode(0),
	}, false)

	if lipgloss.Width(header) != lipgloss.Width(row) {
		t.Fatalf("header width %d != row width %d", lipgloss.Width(header), lipgloss.Width(row))
	}
	if lipgloss.Width(row) > s.contentWidth() {
		t.Fatalf("row width %d exceeds content width %d", lipgloss.Width(row), s.contentWidth())
	}
}

// The highlighted text must survive styling intact - this is the failure mode
// that made bubbles/table unusable here.
func TestRenderRowKeepsCommandTextIntact(t *testing.T) {
	s := newRenderScreen()
	s.filterRe = regexp.MustCompile("(?i)build")

	row := s.renderRow(services.ShellHistoryRecord{
		Command: "go build ./...", WorkingDirectory: "/tmp", StartedAt: 1, ExitCode: exitCode(0),
	}, true)

	plain := stripANSI(row)
	if !strings.Contains(plain, "go build ./...") {
		t.Fatalf("command text mangled: %q", plain)
	}
	if !strings.Contains(row, "\x1b[") {
		t.Fatal("expected highlight styling in the rendered row")
	}
}

func TestRenderHighlighted(t *testing.T) {
	base := lipgloss.NewStyle()
	hl := lipgloss.NewStyle().Bold(true)

	cases := []struct {
		name  string
		text  string
		re    *regexp.Regexp
		plain string
	}{
		{"nil regexp", "hello", nil, "hello"},
		{"no match", "hello", regexp.MustCompile("zzz"), "hello"},
		{"leading match", "hello", regexp.MustCompile("he"), "hello"},
		{"trailing match", "hello", regexp.MustCompile("lo"), "hello"},
		{"multiple matches", "abcabc", regexp.MustCompile("b"), "abcabc"},
		{"zero width", "hello", regexp.MustCompile("x*"), "hello"},
		{"empty text", "", regexp.MustCompile("a"), ""},
		{"full match", "hello", regexp.MustCompile(".*"), "hello"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(renderHighlighted(tc.text, tc.re, base, hl))
			if got != tc.plain {
				t.Fatalf("got %q, want %q", got, tc.plain)
			}
		})
	}
}

func TestTruncateCellPadsToDisplayWidth(t *testing.T) {
	cases := []struct {
		value string
		width int
	}{
		{"short", 10},
		{"exactly-10", 10},
		{"way too long to fit here", 10},
		{"日本語", 10},
		{"日本語のとても長い文字列", 10},
		{"🎉🎉🎉", 10},
		{"", 10},
	}

	for _, tc := range cases {
		got := truncateCell(tc.value, tc.width)
		if w := lipgloss.Width(got); w != tc.width {
			t.Errorf("truncateCell(%q, %d) width %d, want %d", tc.value, tc.width, w, tc.width)
		}
	}

	if got := truncateCell("anything", 0); got != "" {
		t.Errorf("zero width should render empty, got %q", got)
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

package services

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/vsuhanov/scripto/internal/storage"
)

func newTestShellHistoryService(t *testing.T) *ShellHistoryService {
	t.Helper()
	t.Setenv("SCRIPTO_SQLITE_DB_PATH", filepath.Join(t.TempDir(), "scripto.sqlite"))

	db, err := storage.OpenSQLite()
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return NewShellHistoryServiceWithDB(db)
}

func seed(t *testing.T, s *ShellHistoryService, id, command, dir string, startedAt int64) {
	t.Helper()
	if err := s.RecordStart(id, command, dir, "session", ShellHistorySourceShell, startedAt); err != nil {
		t.Fatalf("RecordStart(%q): %v", command, err)
	}
}

func commandsOf(records []ShellHistoryRecord) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.Command
	}
	return out
}

// SQLite rewrites `X REGEXP Y` into regexp(Y, X), so the registered function
// receives the pattern first and the subject second. Getting that backwards
// silently matches nothing, so pin it down with an asymmetric pattern.
func TestGetHistoryRegexpArgumentOrder(t *testing.T) {
	s := newTestShellHistoryService(t)
	seed(t, s, "1", "go build ./...", "/tmp", 100)
	seed(t, s, "2", "npm run build", "/tmp", 200)

	records, err := s.GetHistory(ShellHistoryQuery{Filter: "^go "})
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(records) != 1 || records[0].Command != "go build ./..." {
		t.Fatalf("anchored pattern matched %v, want just the go command", commandsOf(records))
	}
}

func TestGetHistoryRegexpFeatures(t *testing.T) {
	s := newTestShellHistoryService(t)
	seed(t, s, "1", "git commit -m wip", "/tmp", 100)
	seed(t, s, "2", "git push origin main", "/tmp", 200)
	seed(t, s, "3", "docker compose up", "/tmp", 300)

	cases := []struct {
		filter string
		want   int
	}{
		{`git (commit|push)`, 2},
		{`^docker`, 1},
		{`(?i)GIT`, 2},
		{`origin\s+main`, 1},
		{`nomatch`, 0},
	}

	for _, tc := range cases {
		records, err := s.GetHistory(ShellHistoryQuery{Filter: tc.filter})
		if err != nil {
			t.Fatalf("GetHistory(%q): %v", tc.filter, err)
		}
		if len(records) != tc.want {
			t.Errorf("filter %q matched %v, want %d results", tc.filter, commandsOf(records), tc.want)
		}
	}
}

// A malformed pattern must degrade to "no matches" rather than failing the
// query, so a half-typed regexp cannot error out the history screen.
func TestGetHistoryInvalidRegexpDoesNotError(t *testing.T) {
	s := newTestShellHistoryService(t)
	seed(t, s, "1", "git status", "/tmp", 100)

	records, err := s.GetHistory(ShellHistoryQuery{Filter: "foo("})
	if err != nil {
		t.Fatalf("GetHistory with invalid pattern returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("invalid pattern matched %v, want no results", commandsOf(records))
	}
}

// The regexp must be applied before LIMIT, otherwise searching would only ever
// see the most recent page of commands.
func TestGetHistoryRegexpAppliedBeforeLimit(t *testing.T) {
	s := newTestShellHistoryService(t)

	seed(t, s, "old", "needle in a haystack", "/tmp", 1)
	for i := 0; i < 300; i++ {
		seed(t, s, fmt.Sprintf("noise-%d", i), fmt.Sprintf("echo %d", i), "/tmp", int64(1000+i))
	}

	records, err := s.GetHistory(ShellHistoryQuery{Filter: "needle", Limit: 200})
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(records) != 1 || records[0].Command != "needle in a haystack" {
		t.Fatalf("got %v, want the single old matching row", commandsOf(records))
	}
}

func TestGetHistoryFiltersCompose(t *testing.T) {
	s := newTestShellHistoryService(t)
	seed(t, s, "1", "go test ./...", "/project", 100)
	seed(t, s, "2", "go test ./...", "/other", 200)
	if err := s.RecordFinish("1", 1, 150); err != nil {
		t.Fatalf("RecordFinish: %v", err)
	}
	if err := s.RecordFinish("2", 0, 250); err != nil {
		t.Fatalf("RecordFinish: %v", err)
	}

	records, err := s.GetHistory(ShellHistoryQuery{
		Filter:           "^go test",
		WorkingDirectory: "/project",
		FailuresOnly:     true,
	})
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(records) != 1 || records[0].ID != "1" {
		t.Fatalf("got %v, want only the failing /project run", commandsOf(records))
	}
}

func TestGetHistoryExcludeSources(t *testing.T) {
	s := newTestShellHistoryService(t)
	if err := s.RecordStart("1", "scripto deploy", "/tmp", "sess", ShellHistorySourceShell, 100); err != nil {
		t.Fatalf("RecordStart: %v", err)
	}
	if err := s.RecordStart("2", "kubectl apply -f api.yaml", "/tmp", "sess", ShellHistorySourceScripto, 101); err != nil {
		t.Fatalf("RecordStart: %v", err)
	}

	records, err := s.GetHistory(ShellHistoryQuery{ExcludeSources: []string{ShellHistorySourceScripto}})
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(records) != 1 || records[0].Command != "scripto deploy" {
		t.Fatalf("got %v, want only the launcher line", commandsOf(records))
	}

	all, err := s.GetHistory(ShellHistoryQuery{})
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("without ExcludeSources got %v, want both rows", commandsOf(all))
	}
}

func TestGetHistoryExcludeSourcesComposesWithFilter(t *testing.T) {
	s := newTestShellHistoryService(t)
	if err := s.RecordStart("1", "scripto deploy", "/tmp", "sess", ShellHistorySourceShell, 100); err != nil {
		t.Fatalf("RecordStart: %v", err)
	}
	if err := s.RecordStart("2", "deploy the thing", "/tmp", "sess", ShellHistorySourceScripto, 101); err != nil {
		t.Fatalf("RecordStart: %v", err)
	}

	records, err := s.GetHistory(ShellHistoryQuery{
		Filter:         "(?i)deploy",
		ExcludeSources: []string{ShellHistorySourceScripto},
	})
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(records) != 1 || records[0].ID != "1" {
		t.Fatalf("got %v, want only the shell-sourced match", commandsOf(records))
	}
}

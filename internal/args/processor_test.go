package args

import (
	"os"
	"path/filepath"
	"testing"

	"scripto/entities"
)

func scriptWithContent(t *testing.T, content string) *entities.Script {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "script-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return &entities.Script{FilePath: f.Name()}
}

func scriptWithDemarcators(t *testing.T, content, start, end string) *entities.Script {
	t.Helper()
	s := scriptWithContent(t, content)
	s.PlaceholderStartDemarcator = start
	s.PlaceholderEndDemarcator = end
	return s
}

func TestExtractPlaceholderInfo_DefaultDemarcators(t *testing.T) {
	s := scriptWithContent(t, `echo "%name:your name%"`)
	p := NewArgumentProcessor(s)

	placeholders, err := p.extractPlaceholderInfo()
	if err != nil {
		t.Fatal(err)
	}
	if len(placeholders) != 1 {
		t.Fatalf("expected 1 placeholder, got %d", len(placeholders))
	}
	ph, ok := placeholders["name"]
	if !ok {
		t.Fatal("expected placeholder 'name'")
	}
	if ph.Description != "your name" {
		t.Errorf("expected description 'your name', got %q", ph.Description)
	}
}

func TestExtractPlaceholderInfo_EmojiDemarcators_SinglePlaceholder(t *testing.T) {
	s := scriptWithDemarcators(t, `echo "🌟foobar:this is description🍓"`, "🌟", "🍓")
	p := NewArgumentProcessor(s)

	placeholders, err := p.extractPlaceholderInfo()
	if err != nil {
		t.Fatal(err)
	}
	if len(placeholders) != 1 {
		t.Fatalf("expected 1 placeholder, got %d", len(placeholders))
	}
	ph, ok := placeholders["foobar"]
	if !ok {
		t.Fatal("expected placeholder 'foobar'")
	}
	if ph.Description != "this is description" {
		t.Errorf("expected description 'this is description', got %q", ph.Description)
	}
}

func TestExtractPlaceholderInfo_EmojiDemarcators_MultipleOccurrencesSameName(t *testing.T) {
	s := scriptWithDemarcators(t, `echo "🌟foobar:this is descripton🍓 🌟foobar:this is second usage🍓"`, "🌟", "🍓")
	p := NewArgumentProcessor(s)

	placeholders, err := p.extractPlaceholderInfo()
	if err != nil {
		t.Fatal(err)
	}
	if len(placeholders) != 1 {
		t.Fatalf("expected 1 placeholder (same name), got %d: %v", len(placeholders), placeholders)
	}
	if _, ok := placeholders["foobar"]; !ok {
		t.Fatal("expected placeholder 'foobar'")
	}
}

func TestGetPlaceholderOrder_EmojiDemarcators_MultipleOccurrencesSameName(t *testing.T) {
	s := scriptWithDemarcators(t, `echo "🌟foobar:this is descripton🍓 🌟foobar:this is second usage🍓"`, "🌟", "🍓")
	p := NewArgumentProcessor(s)

	order := p.GetPlaceholderOrder()
	if len(order) != 1 {
		t.Fatalf("expected 1 unique placeholder in order, got %d: %v", len(order), order)
	}
	if order[0] != "foobar" {
		t.Errorf("expected 'foobar', got %q", order[0])
	}
}

func TestSubstitutePlaceholders_EmojiDemarcators_MultipleOccurrences(t *testing.T) {
	s := scriptWithDemarcators(t, `echo "🌟foobar:this is descripton🍓 🌟foobar:this is second usage🍓"`, "🌟", "🍓")
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{"vitalij"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `echo "vitalij vitalij"`
	if result.FinalCommand != expected {
		t.Errorf("expected %q, got %q", expected, result.FinalCommand)
	}
}

func TestSubstitutePlaceholders_EmojiDemarcators_MultipleDifferentNames(t *testing.T) {
	s := scriptWithDemarcators(t, `echo "🌟first:first arg🍓 🌟second:second arg🍓"`, "🌟", "🍓")
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{"hello", "world"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `echo "hello world"`
	if result.FinalCommand != expected {
		t.Errorf("expected %q, got %q", expected, result.FinalCommand)
	}
}

func TestSubstitutePlaceholders_DefaultDemarcators_MultipleOccurrencesSameName(t *testing.T) {
	s := scriptWithContent(t, `echo "%name% %name%"`)
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{"vitalij"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `echo "vitalij vitalij"`
	if result.FinalCommand != expected {
		t.Errorf("expected %q, got %q", expected, result.FinalCommand)
	}
}

func TestProcessArguments_NeedsForm_WhenNoArgsProvided(t *testing.T) {
	s := scriptWithDemarcators(t, `echo "🌟name:your name🍓"`, "🌟", "🍓")
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalCommand != "" {
		t.Errorf("expected empty FinalCommand when placeholder is missing, got %q", result.FinalCommand)
	}
	if len(result.MissingArgs) != 1 {
		t.Errorf("expected 1 missing arg, got %d", len(result.MissingArgs))
	}
}

func TestSubstitutePlaceholders_QuotedPlaceholder_ValueWithSpaces(t *testing.T) {
	s := scriptWithContent(t, `echo "%name:your name:hello world%"`)
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{"--name=hello world"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `echo "hello world"`
	if result.FinalCommand != expected {
		t.Errorf("expected %q, got %q", expected, result.FinalCommand)
	}
}

func TestSubstitutePlaceholders_QuotedPlaceholder_DefaultValueWithSpaces(t *testing.T) {
	s := scriptWithContent(t, `echo "%name:your name:hello world%"`)
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{})
	if err != nil {
		t.Fatal(err)
	}
	expected := `echo "hello world"`
	if result.FinalCommand != expected {
		t.Errorf("expected %q, got %q", expected, result.FinalCommand)
	}
}

func TestSubstitutePlaceholders_UnquotedPlaceholder_ValueWithSpaces(t *testing.T) {
	s := scriptWithContent(t, `echo %name%`)
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{"--name=hello world"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `echo "hello world"`
	if result.FinalCommand != expected {
		t.Errorf("expected %q, got %q", expected, result.FinalCommand)
	}
}

func TestSubstitutePlaceholders_TwoQuotedPlaceholders_SimpleValues(t *testing.T) {
	s := scriptWithContent(t, `echo "%foobar:description:there is default value%" "%second:this is longer description%"`)
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{"--foobar=val1", "--second=val2"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `echo "val1" "val2"`
	if result.FinalCommand != expected {
		t.Errorf("expected %q, got %q", expected, result.FinalCommand)
	}
}

func TestSubstitutePlaceholders_TwoQuotedPlaceholders_DefaultValueWithSpaces(t *testing.T) {
	s := scriptWithContent(t, `echo "%foobar:description:there is default value%" "%second:this is longer description%"`)
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{"--foobar=there is default value", "--second=val2"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `echo "there is default value" "val2"`
	if result.FinalCommand != expected {
		t.Errorf("expected %q, got %q", expected, result.FinalCommand)
	}
}

func TestBuildPreviewCommand_QuotedPlaceholder_ValueWithSpaces(t *testing.T) {
	s := scriptWithContent(t, `echo "%name:your name%"`)
	p := NewArgumentProcessor(s)

	result := p.BuildPreviewCommand(map[string]string{"name": "hello world"})
	expected := `echo "hello world"`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildPreviewCommand_UnquotedPlaceholder_ValueWithSpaces(t *testing.T) {
	s := scriptWithContent(t, `echo %name%`)
	p := NewArgumentProcessor(s)

	result := p.BuildPreviewCommand(map[string]string{"name": "hello world"})
	expected := `echo "hello world"`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestScriptWithFilePath_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myscript.txt")
	if err := os.WriteFile(path, []byte(`echo "🌟val:value🍓"`), 0644); err != nil {
		t.Fatal(err)
	}
	s := &entities.Script{
		FilePath:                   path,
		PlaceholderStartDemarcator: "🌟",
		PlaceholderEndDemarcator:   "🍓",
	}
	p := NewArgumentProcessor(s)

	result, err := p.ProcessArguments([]string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `echo "test"`
	if result.FinalCommand != expected {
		t.Errorf("expected %q, got %q", expected, result.FinalCommand)
	}
}

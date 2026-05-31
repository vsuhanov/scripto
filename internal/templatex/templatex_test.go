package templatex

import (
	"testing"
)

func TestExtractVariables_Empty(t *testing.T) {
	metas, err := ExtractVariables("")
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 0 {
		t.Fatalf("expected 0 variables, got %d", len(metas))
	}
}

func TestExtractVariables_PlainVariable(t *testing.T) {
	metas, err := ExtractVariables(`{{ .Name }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(metas))
	}
	if metas[0].Name != "Name" {
		t.Errorf("expected Name=Name, got %q", metas[0].Name)
	}
	if metas[0].Label != "Name" {
		t.Errorf("expected Label=Name, got %q", metas[0].Label)
	}
}

func TestExtractVariables_LabelAnnotation(t *testing.T) {
	metas, err := ExtractVariables(`{{ .Name | label "Full Name" }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(metas))
	}
	if metas[0].Label != "Full Name" {
		t.Errorf("expected Label=Full Name, got %q", metas[0].Label)
	}
}

func TestExtractVariables_DefaultValueAnnotation(t *testing.T) {
	metas, err := ExtractVariables(`{{ .X | defaultValue "hello" }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(metas))
	}
	if metas[0].DefaultValue != "hello" {
		t.Errorf("expected DefaultValue=hello, got %q", metas[0].DefaultValue)
	}
}

func TestExtractVariables_AllowedValuesAnnotation(t *testing.T) {
	metas, err := ExtractVariables(`{{ .Env | allowedValues "dev" "prod" }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(metas))
	}
	if len(metas[0].AllowedValues) != 2 {
		t.Fatalf("expected 2 allowed values, got %d", len(metas[0].AllowedValues))
	}
	if metas[0].AllowedValues[0] != "dev" || metas[0].AllowedValues[1] != "prod" {
		t.Errorf("unexpected AllowedValues: %v", metas[0].AllowedValues)
	}
}

func TestExtractVariables_FullAnnotation(t *testing.T) {
	metas, err := ExtractVariables(`{{ .Svc | label "Service" | defaultValue "api" | allowedValues "api" "worker" }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(metas))
	}
	m := metas[0]
	if m.Name != "Svc" {
		t.Errorf("Name: got %q", m.Name)
	}
	if m.Label != "Service" {
		t.Errorf("Label: got %q", m.Label)
	}
	if m.DefaultValue != "api" {
		t.Errorf("DefaultValue: got %q", m.DefaultValue)
	}
	if len(m.AllowedValues) != 2 {
		t.Errorf("AllowedValues count: got %d", len(m.AllowedValues))
	}
}

func TestExtractVariables_Deduplication_LastLabelWins(t *testing.T) {
	metas, err := ExtractVariables(`{{ .X }} ... {{ .X | label "renamed" }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 variable (deduplicated), got %d", len(metas))
	}
	if metas[0].Label != "renamed" {
		t.Errorf("expected last label to win, got %q", metas[0].Label)
	}
}

func TestExtractVariables_IfCondition(t *testing.T) {
	metas, err := ExtractVariables(`{{ if eq .A .B }}equal{{ else }}not equal{{ end }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(metas))
	}
	if metas[0].Name != "A" || metas[1].Name != "B" {
		t.Errorf("expected A and B, got %q and %q", metas[0].Name, metas[1].Name)
	}
}

func TestExtractVariables_AnnotatedConditionWithParam(t *testing.T) {
	metas, err := ExtractVariables(`{{ if eq .Count .Env | label "Count" | param .Env | allowedValues "foo" "bar" }}yes{{ else }}no{{ end }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(metas))
	}
	if metas[0].Label != "Count" {
		t.Errorf("Count label: got %q", metas[0].Label)
	}
	if len(metas[1].AllowedValues) != 2 {
		t.Errorf("Env allowedValues count: got %d", len(metas[1].AllowedValues))
	}
}

func TestExtractVariables_Range(t *testing.T) {
	metas, err := ExtractVariables(`{{ range .Items }}{{ .Name }}{{ end }}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(metas))
	}
	if metas[0].Name != "Items" || metas[1].Name != "Name" {
		t.Errorf("expected Items and Name, got %q and %q", metas[0].Name, metas[1].Name)
	}
}

func TestExtractVariables_InvalidSyntax(t *testing.T) {
	_, err := ExtractVariables(`{{ .Name`)
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
}

func TestExecute_SimpleSubstitution(t *testing.T) {
	result, err := Execute(`echo {{ .Name }}`, map[string]string{"Name": "world"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "echo world" {
		t.Errorf("expected %q, got %q", "echo world", result)
	}
}

func TestExecute_MultipleVariables(t *testing.T) {
	result, err := Execute(`{{ .Cmd }} -n {{ .NS }}`, map[string]string{"Cmd": "kubectl", "NS": "default"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "kubectl -n default" {
		t.Errorf("expected %q, got %q", "kubectl -n default", result)
	}
}

func TestExecute_MissingVariableRendersEmpty(t *testing.T) {
	result, err := Execute(`echo {{ .Missing }}`, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if result != "echo" {
		t.Errorf("expected %q, got %q", "echo", result)
	}
}

func TestExecute_WithAnnotations(t *testing.T) {
	tmpl := `kubectl rollout restart deploy/{{ .Service | label "Service name" | defaultValue "api" }} -n {{ .Namespace | allowedValues "default" "staging" }}`
	result, err := Execute(tmpl, map[string]string{"Service": "worker", "Namespace": "staging"})
	if err != nil {
		t.Fatal(err)
	}
	expected := "kubectl rollout restart deploy/worker -n staging"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExecute_InvalidTemplate(t *testing.T) {
	_, err := Execute(`{{ .Name`, map[string]string{})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}

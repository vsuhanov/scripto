package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
	"text/template/parse"
)

type VariableMeta struct {
	Name          string
	Label         string
	DefaultValue  string
	AllowedValues []string
}

func (v VariableMeta) String() string {
	av := "none"
	if len(v.AllowedValues) > 0 {
		av = "[" + strings.Join(v.AllowedValues, ", ") + "]"
	}
	def := v.DefaultValue
	if def == "" {
		def = "(none)"
	}
	return fmt.Sprintf("Name=%-14s Label=%-14s Default=%-10s AllowedValues=%s",
		fmt.Sprintf("%q", v.Name),
		fmt.Sprintf("%q", v.Label),
		fmt.Sprintf("%q", def),
		av,
	)
}

type extractor struct {
	vars  map[string]*VariableMeta
	order []string
}

func newExtractor() *extractor {
	return &extractor{vars: make(map[string]*VariableMeta)}
}

func (e *extractor) getOrCreate(name string) *VariableMeta {
	if _, ok := e.vars[name]; !ok {
		e.vars[name] = &VariableMeta{Name: name, Label: name}
		e.order = append(e.order, name)
	}
	return e.vars[name]
}

func (e *extractor) results() []VariableMeta {
	result := make([]VariableMeta, len(e.order))
	for i, name := range e.order {
		result[i] = *e.vars[name]
	}
	return result
}

func (e *extractor) walk(list *parse.ListNode) {
	if list == nil {
		return
	}
	for _, node := range list.Nodes {
		switch n := node.(type) {
		case *parse.ActionNode:
			e.extractFromPipe(n.Pipe, false)
		case *parse.IfNode:
			e.extractFromPipe(n.Pipe, true)
			e.walk(n.List)
			e.walk(n.ElseList)
		case *parse.RangeNode:
			e.extractFromPipe(n.Pipe, false)
			e.walk(n.List)
			e.walk(n.ElseList)
		case *parse.WithNode:
			e.extractFromPipe(n.Pipe, true)
			e.walk(n.List)
			e.walk(n.ElseList)
		}
	}
}

// extractFromPipe handles both simple variable actions and control-flow conditions.
// isCondition=true means the first command may be a function call (eq, ne, …) whose
// FieldNode arguments are the parameters, with optional pipe annotations after it.
func (e *extractor) extractFromPipe(pipe *parse.PipeNode, isCondition bool) {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return
	}
	cmd0 := pipe.Cmds[0]
	if len(cmd0.Args) == 0 {
		return
	}
	if field, ok := cmd0.Args[0].(*parse.FieldNode); ok {
		varName := strings.Join(field.Ident, ".")
		meta := e.getOrCreate(varName)
		e.applyAnnotations(meta, pipe.Cmds[1:])
	} else if _, ok := cmd0.Args[0].(*parse.IdentifierNode); ok && isCondition {
		e.extractAnnotatedCondition(pipe.Cmds)
	}
}

// extractAnnotatedCondition handles:
//
//	{{ if eq .A .B }}                           — plain, no annotations
//	{{ if eq .A .B | label "A label" | param .B | allowedValues "x" "y" }}
//
// Annotations before the first `param` apply to the first variable in the condition.
// Each `param .VarName` shifts the annotation target to .VarName.
func (e *extractor) extractAnnotatedCondition(cmds []*parse.CommandNode) {
	var condVars []string
	for _, arg := range cmds[0].Args[1:] {
		if field, ok := arg.(*parse.FieldNode); ok {
			name := strings.Join(field.Ident, ".")
			e.getOrCreate(name)
			condVars = append(condVars, name)
		}
	}
	if len(cmds) == 1 || len(condVars) == 0 {
		return
	}

	currentVar := condVars[0]
	var currentCmds []*parse.CommandNode

	flush := func() {
		if currentVar != "" && len(currentCmds) > 0 {
			e.applyAnnotations(e.getOrCreate(currentVar), currentCmds)
		}
		currentCmds = nil
	}

	for _, cmd := range cmds[1:] {
		if len(cmd.Args) == 0 {
			continue
		}
		ident, ok := cmd.Args[0].(*parse.IdentifierNode)
		if !ok {
			continue
		}
		if ident.Ident == "param" {
			flush()
			if len(cmd.Args) > 1 {
				if field, ok := cmd.Args[1].(*parse.FieldNode); ok {
					currentVar = strings.Join(field.Ident, ".")
				}
			}
		} else {
			currentCmds = append(currentCmds, cmd)
		}
	}
	flush()
}

func (e *extractor) applyAnnotations(meta *VariableMeta, cmds []*parse.CommandNode) {
	for _, cmd := range cmds {
		if len(cmd.Args) == 0 {
			continue
		}
		ident, ok := cmd.Args[0].(*parse.IdentifierNode)
		if !ok {
			continue
		}
		switch ident.Ident {
		case "label":
			if len(cmd.Args) > 1 {
				if s, ok := cmd.Args[1].(*parse.StringNode); ok {
					meta.Label = s.Text
				}
			}
		case "defaultValue":
			if len(cmd.Args) > 1 {
				if s, ok := cmd.Args[1].(*parse.StringNode); ok {
					meta.DefaultValue = s.Text
				}
			}
		case "allowedValues":
			meta.AllowedValues = nil
			for _, arg := range cmd.Args[1:] {
				if s, ok := arg.(*parse.StringNode); ok {
					meta.AllowedValues = append(meta.AllowedValues, s.Text)
				}
			}
		}
	}
}

// parseFuncMap is used by parse.Parse so it recognises our custom identifiers.
// Signatures must be valid Go functions; return types only matter for execution.
var parseFuncMap = map[string]interface{}{
	"label":         func(string, interface{}) interface{} { return nil },
	"defaultValue":  func(string, interface{}) interface{} { return nil },
	"allowedValues": func(...interface{}) interface{} { return nil },
	// param receives (fieldValue, pipedValue) and passes the piped bool through
	"param": func(interface{}, interface{}) interface{} { return nil },
	// comparison builtins are not known to parse.Parse — register them here
	"eq": func(interface{}, interface{}) bool { return false },
	"ne": func(interface{}, interface{}) bool { return false },
	"lt": func(interface{}, interface{}) bool { return false },
	"le": func(interface{}, interface{}) bool { return false },
	"gt": func(interface{}, interface{}) bool { return false },
	"ge": func(interface{}, interface{}) bool { return false },
}

// execFuncMap registers noop functions for template execution.
// In Go templates the piped value is appended as the last argument.
//
//	{{ .X | label "Foo" }}  →  label("Foo", X)
//	{{ .X | allowedValues "A" "B" }}  →  allowedValues("A", "B", X)
//	{{ cond | param .Y }}  →  param(Y, cond)
var execFuncMap = template.FuncMap{
	"label":        func(lbl string, v interface{}) interface{} { return v },
	"defaultValue": func(def string, v interface{}) interface{} { return v },
	"allowedValues": func(args ...interface{}) interface{} {
		if len(args) > 0 {
			return args[len(args)-1]
		}
		return nil
	},
	"param": func(varVal, piped interface{}) interface{} { return piped },
}

func extractMeta(templateStr string) ([]VariableMeta, error) {
	trees, err := parse.Parse("tmpl", templateStr, "", "", parseFuncMap)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	tree, ok := trees["tmpl"]
	if !ok || tree == nil || tree.Root == nil {
		return nil, nil
	}
	e := newExtractor()
	e.walk(tree.Root)
	return e.results(), nil
}

func executeTemplate(templateStr string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("tmpl").Funcs(execFuncMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute error: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

func runCase(name, templateStr string, data map[string]interface{}) {
	fmt.Printf("--- %s ---\n", name)
	fmt.Printf("Template: %s\n", templateStr)

	metas, err := extractMeta(templateStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
		fmt.Println()
		return
	}

	fmt.Printf("Variables (%d):\n", len(metas))
	for _, m := range metas {
		fmt.Printf("  %s\n", m)
	}

	if data != nil {
		result, err := executeTemplate(templateStr, data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  EXEC ERROR: %v\n", err)
		} else {
			fmt.Printf("Execution: %q\n", result)
		}
	}
	fmt.Println()
}

func main() {
	runCase(
		"1. Full annotation",
		`{{ .MyValue | label "Foobar" | defaultValue "hello" | allowedValues "A" "B" "C" }}`,
		map[string]interface{}{"MyValue": "chosen"},
	)

	runCase(
		"2. Plain variable — no metadata, label defaults to name",
		`Hello, {{ .PlainVar }}!`,
		map[string]interface{}{"PlainVar": "World"},
	)

	runCase(
		"3. Deduplication — .X appears twice",
		`{{ .X }} and {{ .X }}`,
		map[string]interface{}{"X": "42"},
	)

	runCase(
		"4. if condition — extract both vars, default labels",
		`{{ if eq .Count .Foobar }}equal{{ else }}not equal{{ end }}`,
		map[string]interface{}{"Count": 1, "Foobar": 1},
	)

	runCase(
		"5. if condition — annotated with param delimiter",
		`{{ if eq .Count .Foobar | label "Count" | param .Foobar | allowedValues "foo" "bar" }}yes{{ else }}no{{ end }}`,
		map[string]interface{}{"Count": "foo", "Foobar": "foo"},
	)

	runCase(
		"6. range — recurse into body",
		`{{ range .Items }}{{ .Name }} {{ end }}`,
		map[string]interface{}{
			"Items": []map[string]interface{}{
				{"Name": "alpha"},
				{"Name": "beta"},
			},
		},
	)
}

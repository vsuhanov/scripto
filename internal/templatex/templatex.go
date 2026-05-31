package templatex

import (
	"bytes"
	"fmt"
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

var parseFuncMap = map[string]interface{}{
	"label":         func(string, interface{}) interface{} { return nil },
	"defaultValue":  func(string, interface{}) interface{} { return nil },
	"allowedValues": func(...interface{}) interface{} { return nil },
	"param":         func(interface{}, interface{}) interface{} { return nil },
	"eq":            func(interface{}, interface{}) bool { return false },
	"ne":            func(interface{}, interface{}) bool { return false },
	"lt":            func(interface{}, interface{}) bool { return false },
	"le":            func(interface{}, interface{}) bool { return false },
	"gt":            func(interface{}, interface{}) bool { return false },
	"ge":            func(interface{}, interface{}) bool { return false },
}

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

func ExtractVariables(templateStr string) ([]VariableMeta, error) {
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

func Execute(templateStr string, values map[string]string) (string, error) {
	tmpl, err := template.New("tmpl").Option("missingkey=zero").Funcs(execFuncMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("execute error: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

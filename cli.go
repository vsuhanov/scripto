package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/vsuhanov/scripto/entities"
	"github.com/vsuhanov/scripto/internal/services"
	"github.com/vsuhanov/scripto/internal/templatex"
)

type cliPlaceholder struct {
	Name          string   `json:"name"`
	Label         string   `json:"label"`
	DefaultValue  string   `json:"default_value,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
}

type cliScript struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	Scope        string           `json:"scope"`
	FilePath     string           `json:"file_path"`
	Archived     bool             `json:"archived"`
	Command      string           `json:"command"`
	Placeholders []cliPlaceholder `json:"placeholders"`
}

type cliJSONInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Scope       *string `json:"scope"`
	Command     *string `json:"command"`
}

const cliUsage = `Usage: scripto cli <verb> [flags]

Non-interactive script management with JSON output.

Verbs:
  list       List scripts (--all, --archived)
  get        Show a single script (--id | --name)
  add        Create a script (--name, --description, --scope, --command | --command-file | --stdin, --json)
  edit       Update a script (--id | --name, --new-name, --description, --scope, --command | --command-file | --stdin, --json)
  delete     Delete a script (--id | --name)
  archive    Archive a script (--id | --name)
  unarchive  Unarchive a script (--id | --name)

Run 'scripto cli <verb> --help' for verb-specific flags.
All verbs print JSON to stdout; errors print {"error": "..."} with exit code 1.`

func handleCli(container *services.Container, args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Println(cliUsage)
		if len(args) == 0 {
			return 1
		}
		return 0
	}
	switch args[0] {
	case "list":
		return cliList(container, args[1:])
	case "get":
		return cliGet(container, args[1:])
	case "add":
		return cliAdd(container, args[1:])
	case "edit":
		return cliEdit(container, args[1:])
	case "delete":
		return cliDelete(container, args[1:])
	case "archive":
		return cliArchiveToggle(container, args[1:], true)
	case "unarchive":
		return cliArchiveToggle(container, args[1:], false)
	default:
		return cliError(fmt.Sprintf("unknown verb '%s': expected one of list, get, add, edit, delete, archive, unarchive", args[0]))
	}
}

func printJSON(v any) int {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return cliError(err.Error())
	}
	fmt.Println(string(data))
	return 0
}

func cliError(msg string) int {
	data, _ := json.Marshal(map[string]string{"error": msg})
	fmt.Println(string(data))
	return 1
}

func newCliFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func cliParse(fs *flag.FlagSet, args []string) (bool, int) {
	err := fs.Parse(args)
	if err == nil {
		return true, 0
	}
	if errors.Is(err, flag.ErrHelp) {
		fmt.Printf("Usage: scripto cli %s [flags]\n\nFlags:\n", fs.Name())
		fs.SetOutput(os.Stdout)
		fs.PrintDefaults()
		return false, 0
	}
	return false, cliError(err.Error())
}

func toCliScript(s *entities.Script) cliScript {
	command := ""
	if s.FilePath != "" {
		if data, err := os.ReadFile(s.FilePath); err == nil {
			command = string(data)
		}
	}

	placeholders := []cliPlaceholder{}
	if vars, err := templatex.ExtractVariables(command); err == nil {
		for _, v := range vars {
			placeholders = append(placeholders, cliPlaceholder{
				Name:          v.Name,
				Label:         v.Label,
				DefaultValue:  v.DefaultValue,
				AllowedValues: v.AllowedValues,
			})
		}
	}

	scope := s.Scope
	if s.OriginalScope != "" {
		scope = s.OriginalScope
	}

	return cliScript{
		ID:           s.ID,
		Name:         s.Name,
		Description:  s.Description,
		Scope:        scope,
		FilePath:     s.FilePath,
		Archived:     s.Archived,
		Command:      command,
		Placeholders: placeholders,
	}
}

func resolveScript(container *services.Container, id, name string) (*entities.Script, error) {
	if (id == "") == (name == "") {
		return nil, fmt.Errorf("exactly one of --id or --name is required")
	}

	all, err := container.ScriptService.FindAllScopesScriptsWithArchived()
	if err != nil {
		return nil, err
	}

	if id != "" {
		for _, s := range all {
			if s.ID == id {
				return s, nil
			}
		}
		return nil, fmt.Errorf("no script found with id '%s'", id)
	}

	match, err := container.ScriptService.Match(name)
	if err != nil {
		return nil, err
	}
	if match != nil {
		return match, nil
	}

	matches, err := container.ScriptService.MatchAllScopes(name)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		for _, s := range all {
			if s.Name != "" && s.Name == name {
				matches = append(matches, s)
			}
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no script found with name '%s'", name)
	}
	if len(matches) > 1 {
		var scopes []string
		for _, m := range matches {
			scopes = append(scopes, m.Scope)
		}
		return nil, fmt.Errorf("ambiguous name '%s' found in scopes: %s; use --id instead", name, strings.Join(scopes, ", "))
	}
	return matches[0], nil
}

func readCommandInput(cmdFlag, cmdFile string, useStdin bool) (string, bool, error) {
	if useStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", false, fmt.Errorf("failed to read stdin: %w", err)
		}
		return string(data), true, nil
	}
	if cmdFile != "" {
		data, err := os.ReadFile(cmdFile)
		if err != nil {
			return "", false, fmt.Errorf("failed to read command file: %w", err)
		}
		return string(data), true, nil
	}
	if cmdFlag != "" {
		return cmdFlag, true, nil
	}
	return "", false, nil
}

func readJSONInput() (*cliJSONInput, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdin: %w", err)
	}
	var payload cliJSONInput
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON input: %w", err)
	}
	return &payload, nil
}

func cliList(container *services.Container, args []string) int {
	fs := newCliFlagSet("list")
	all := fs.Bool("all", false, "list scripts from every scope, not just those visible from the current directory")
	archived := fs.Bool("archived", false, "list every script including archived ones")
	if ok, code := cliParse(fs, args); !ok {
		return code
	}

	var scripts []*entities.Script
	var err error
	if *archived {
		scripts, err = container.ScriptService.FindAllScopesScriptsWithArchived()
	} else if *all {
		scripts, err = container.ScriptService.FindAllScopesScripts()
	} else {
		scripts, err = container.ScriptService.FindAllScripts()
	}
	if err != nil {
		return cliError(err.Error())
	}

	out := []cliScript{}
	for _, s := range scripts {
		out = append(out, toCliScript(s))
	}
	return printJSON(out)
}

func cliGet(container *services.Container, args []string) int {
	fs := newCliFlagSet("get")
	id := fs.String("id", "", "select script by id")
	name := fs.String("name", "", "select script by name")
	if ok, code := cliParse(fs, args); !ok {
		return code
	}

	script, err := resolveScript(container, *id, *name)
	if err != nil {
		return cliError(err.Error())
	}
	return printJSON(toCliScript(script))
}

func cliAdd(container *services.Container, args []string) int {
	fs := newCliFlagSet("add")
	name := fs.String("name", "", "script name")
	description := fs.String("description", "", "script description")
	scope := fs.String("scope", "", "scope: 'global', an absolute directory path, or a glob pattern (default: current directory)")
	command := fs.String("command", "", "command body as a string")
	commandFile := fs.String("command-file", "", "read command body from a file")
	useStdin := fs.Bool("stdin", false, "read command body from stdin")
	useJSON := fs.Bool("json", false, "read {name,description,scope,command} JSON object from stdin; explicit flags override")
	if ok, code := cliParse(fs, args); !ok {
		return code
	}
	if *useJSON && *useStdin {
		return cliError("--json and --stdin cannot be combined")
	}

	script := container.ScriptService.CreateEmptyScript()
	var commandBody string
	haveCommand := false

	if *useJSON {
		payload, err := readJSONInput()
		if err != nil {
			return cliError(err.Error())
		}
		if payload.Name != nil {
			script.Name = *payload.Name
		}
		if payload.Description != nil {
			script.Description = *payload.Description
		}
		if payload.Scope != nil {
			script.Scope = *payload.Scope
		}
		if payload.Command != nil {
			commandBody = *payload.Command
			haveCommand = true
		}
	}

	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "name":
			script.Name = *name
		case "description":
			script.Description = *description
		case "scope":
			script.Scope = *scope
		}
	})

	if body, ok, err := readCommandInput(*command, *commandFile, *useStdin); err != nil {
		return cliError(err.Error())
	} else if ok {
		commandBody = body
		haveCommand = true
	}

	if !haveCommand || strings.TrimSpace(commandBody) == "" {
		return cliError("command is required: provide --command, --command-file, --stdin, or a 'command' key with --json")
	}

	if err := container.ScriptService.ValidateScript(script); err != nil {
		return cliError(err.Error())
	}
	if err := container.ScriptService.SaveScript(script, commandBody, nil); err != nil {
		return cliError(err.Error())
	}
	return printJSON(toCliScript(script))
}

func cliEdit(container *services.Container, args []string) int {
	fs := newCliFlagSet("edit")
	id := fs.String("id", "", "select script by id")
	name := fs.String("name", "", "select script by name")
	newName := fs.String("new-name", "", "rename the script")
	description := fs.String("description", "", "new description (omit to preserve, pass \"\" to clear)")
	scope := fs.String("scope", "", "new scope: 'global', an absolute directory path, or a glob pattern")
	command := fs.String("command", "", "new command body as a string")
	commandFile := fs.String("command-file", "", "read new command body from a file")
	useStdin := fs.Bool("stdin", false, "read new command body from stdin")
	useJSON := fs.Bool("json", false, "read {name,description,scope,command} JSON object from stdin; only present keys are applied")
	if ok, code := cliParse(fs, args); !ok {
		return code
	}
	if *useJSON && *useStdin {
		return cliError("--json and --stdin cannot be combined")
	}

	original, err := resolveScript(container, *id, *name)
	if err != nil {
		return cliError(err.Error())
	}

	updated := *original
	updated.OriginalScope = ""
	var commandBody string
	haveCommand := false

	if *useJSON {
		payload, err := readJSONInput()
		if err != nil {
			return cliError(err.Error())
		}
		if payload.Name != nil {
			updated.Name = *payload.Name
		}
		if payload.Description != nil {
			updated.Description = *payload.Description
		}
		if payload.Scope != nil {
			updated.Scope = *payload.Scope
		}
		if payload.Command != nil {
			commandBody = *payload.Command
			haveCommand = true
		}
	}

	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "new-name":
			updated.Name = *newName
		case "description":
			updated.Description = *description
		case "scope":
			updated.Scope = *scope
		}
	})

	if body, ok, err := readCommandInput(*command, *commandFile, *useStdin); err != nil {
		return cliError(err.Error())
	} else if ok {
		commandBody = body
		haveCommand = true
	}

	if !haveCommand {
		if original.FilePath == "" {
			return cliError("script has no file and no command source was provided")
		}
		data, err := os.ReadFile(original.FilePath)
		if err != nil {
			return cliError(fmt.Sprintf("failed to read current command: %v", err))
		}
		commandBody = string(data)
	}

	if err := container.ScriptService.ValidateScript(&updated); err != nil {
		return cliError(err.Error())
	}
	if err := container.ScriptService.SaveScript(&updated, commandBody, original); err != nil {
		return cliError(err.Error())
	}
	return printJSON(toCliScript(&updated))
}

func cliDelete(container *services.Container, args []string) int {
	fs := newCliFlagSet("delete")
	id := fs.String("id", "", "select script by id")
	name := fs.String("name", "", "select script by name")
	if ok, code := cliParse(fs, args); !ok {
		return code
	}

	script, err := resolveScript(container, *id, *name)
	if err != nil {
		return cliError(err.Error())
	}
	if err := container.ScriptService.DeleteScript(script); err != nil {
		return cliError(err.Error())
	}
	return printJSON(map[string]any{"deleted": true, "id": script.ID})
}

func cliArchiveToggle(container *services.Container, args []string, archive bool) int {
	verb := "archive"
	if !archive {
		verb = "unarchive"
	}
	fs := newCliFlagSet(verb)
	id := fs.String("id", "", "select script by id")
	name := fs.String("name", "", "select script by name")
	if ok, code := cliParse(fs, args); !ok {
		return code
	}

	script, err := resolveScript(container, *id, *name)
	if err != nil {
		return cliError(err.Error())
	}
	if archive {
		err = container.ScriptService.ArchiveScript(script)
	} else {
		err = container.ScriptService.UnarchiveScript(script)
	}
	if err != nil {
		return cliError(err.Error())
	}
	return printJSON(map[string]any{"archived": archive, "id": script.ID})
}

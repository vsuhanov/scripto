---
name: scripto
description: Manage the user's scripto shell scripts (list, inspect, create, edit, delete, archive) via the non-interactive `scripto cli` commands with JSON output.
---

# Scripto CLI

Scripto stores reusable shell commands ("scripts") with names, descriptions, and scopes. The `scripto cli` command group is fully non-interactive and always prints JSON to stdout, making it safe for programmatic use.

## Script model

Each script has:

- `id` — stable unique identifier (preferred selector for edits)
- `name` — short identifier used to run the script (`scripto <name>`); may be empty
- `description` — human-readable description
- `scope` — where the script is visible:
  - `global` — visible everywhere
  - an absolute directory path (e.g. `/Users/x/projects/app`) — visible in that directory and its subdirectories
  - a glob pattern (e.g. `/Users/x/projects/**`) — visible in any matching directory
- `file_path` — path to the file holding the command body (managed by scripto)
- `archived` — hidden from normal listings when true
- `command` — the command body (a Go text/template, see placeholder syntax below)
- `placeholders` — variables extracted from the command: `{name, label, default_value, allowed_values}`

Caveat: a script literally named `cli` cannot be run via bare `scripto cli` (that invokes this command group). It remains fully manageable through `scripto cli get/edit/...`.

## CLI reference

All verbs print JSON to stdout. Exit code 0 on success, 1 on failure. Failures print `{"error": "message"}`.

### list

```
scripto cli list              # scripts visible from the current directory (cwd + matching patterns + global)
scripto cli list --all        # every script in every scope
scripto cli list --archived   # every script including archived ones
```

Output: JSON array of script objects.

```json
[
  {
    "id": "a1b2c3",
    "name": "deploy",
    "description": "Deploy to server",
    "scope": "global",
    "file_path": "/Users/x/.scripto/scripts/a1b2c3_deploy.zsh",
    "archived": false,
    "command": "scp {{ .File }} user@{{ .Server }}:~/apps/",
    "placeholders": [
      {"name": "File", "label": "File"},
      {"name": "Server", "label": "Server"}
    ]
  }
]
```

### get

```
scripto cli get --id <id>
scripto cli get --name <name>
```

Exactly one of `--id` or `--name` is required (applies to all selector-based verbs). If a name exists in multiple scopes, the error lists the candidate scopes; retry with `--id`.

Output: a single script object.

### add

```
scripto cli add --name build --description "Build the app" --command 'go build -o bin/app .'
scripto cli add --name deploy --scope global --command-file ./cmd.txt
cat cmd.txt | scripto cli add --name deploy --stdin
echo '{"name":"t2","command":"ls -la","scope":"global"}' | scripto cli add --json
```

Flags:

- `--name`, `--description` — optional metadata
- `--scope` — defaults to the current working directory; use `global`, an absolute path, or a glob pattern
- Command body (required, exactly one source): `--command <string>`, `--command-file <path>`, or `--stdin`
- `--json` — read a full object from stdin (see JSON input schema); explicit flags override JSON keys

Output: the created script object (with its assigned `id`). Duplicate name in the same scope is an error.

### edit

```
scripto cli edit --name build --description "new description"
scripto cli edit --id a1b2c3 --new-name build2 --command 'go build -v -o bin/app .'
echo '{"description":"updated"}' | scripto cli edit --name build --json
```

Selector: `--id` or `--name`. Updates:

- `--new-name` — rename the script
- `--description`, `--scope` — only applied when the flag is explicitly present (`--description ""` clears it; omitting it preserves the current value)
- `--command`, `--command-file`, `--stdin` — replace the command body; when omitted, the body is unchanged
- `--json` — object on stdin; only present keys are applied (`name` here means the new name)

Output: the updated script object. `file_path` is preserved across edits.

### delete

```
scripto cli delete --id <id>
scripto cli delete --name <name>
```

Permanently removes the script and its command file. Output: `{"deleted": true, "id": "..."}`.

### archive / unarchive

```
scripto cli archive --name old-task
scripto cli unarchive --name old-task
```

Archiving hides a script from normal listings without deleting it. Output: `{"archived": true|false, "id": "..."}`. Archived scripts are visible via `list --archived` and can be selected by `--id` or `--name`.

## JSON input schema (add/edit `--json`)

```json
{
  "name": "string",
  "description": "string",
  "scope": "global | /abs/path | /glob/**",
  "command": "string"
}
```

All keys optional; only present keys are applied. Explicit CLI flags take precedence over JSON keys.

## Placeholder syntax

Command bodies are Go text/templates. Every `{{ .VarName }}` becomes a placeholder that the user fills in a form when executing the script. Repeated uses of the same variable are deduplicated and prompted once.

### Basic variable

```
kubectl get pods -n {{ .Namespace }}
```

### Pipe annotations

Annotations after a variable control how the input form renders that field:

| Pipe | Effect |
|---|---|
| `\| label "Display Label"` | Sets the field label shown in the form |
| `\| defaultValue "value"` | Pre-fills the input with a default value |
| `\| allowedValues "a" "b" "c"` | Renders a picker restricted to the listed options |

Annotations combine in any order; if the same annotation appears twice, the later one wins. Unknown pipe functions are ignored.

```
echo {{ .Msg | defaultValue "hi" }}
scp {{ .File | label "File to deploy" }} user@{{ .Server | label "Target server" | defaultValue "prod-1" }}:~/apps/
kubectl rollout restart deploy/{{ .Service | label "Service" | defaultValue "api" }} -n {{ .Env | allowedValues "default" "staging" "prod" }}
```

### Conditionals

Variables inside `{{ if }}` conditions are extracted too:

```
{{ if eq .Env "prod" }}kubectl --context prod apply -f .{{ else }}kubectl apply -f .{{ end }}
```

To annotate variables used inside a condition, append pipes after the condition. Annotations apply to the first variable of the condition until a `| param .OtherVar` delimiter routes subsequent annotations to that variable:

```
{{ if eq .Env .Target | label "Current env" | param .Target | allowedValues "staging" "prod" }}...{{ end }}
```

Here `label "Current env"` applies to `.Env` and `allowedValues "staging" "prod"` applies to `.Target`.

### Semantics

- Variables with no value provided render as empty strings (`missingkey=zero`)
- The final rendered command is trimmed of leading/trailing whitespace
- There is no `$ENV` or positional-argument substitution — only `{{ .Var }}` template variables

## Safety

- For experiments or tests, set `SCRIPTO_CONFIG=/tmp/some-config.json` to avoid touching the user's real scripts
- Never hand-edit `~/.scripto/scripts.json` or the files under `~/.scripto/scripts/` — always use `scripto cli edit` so metadata, script files, and shell shortcuts stay in sync

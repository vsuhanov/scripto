# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

- **Build**: `go build -o bin/scripto ./cmd/scripto` - Builds the scripto binary to bin/scripto
- **Run**: `go run ./cmd/scripto` - Runs the application directly
- **Test**: `go test ./...` - Runs all tests (currently no test files exist)
- **Install**: `go install ./cmd/scripto` - Installs scripto to $GOPATH/bin

## Architecture Overview

Scripto is a Go CLI application for managing and executing custom scripts with placeholder support. The application uses:

- **CLI Framework**: Cobra for command-line structure and argument parsing
- **Storage**: JSON file at `~/.scripto/scripts.json` for script persistence
- **Future TUI**: Planned bubbletea integration for interactive interface

### Key Components

1. **Main Entry Point**: `cmd/scripto/main.go` - Simple main function that calls Execute()

2. **Root Command**: `cmd/scripto/root.go` - Defines the root cobra command with placeholder TUI message

3. **Add Command**: `cmd/scripto/add.go` - Implements `scripto add [flags] <command>` functionality:
   - Stores scripts globally with `--global` flag or locally to current directory
   - Parses commands and supports placeholder format `$variable:description$`

4. **Storage Layer**: `internal/storage/storage.go` - Handles configuration persistence:
   - Config stored as JSON at `~/.scripto/scripts.json`
   - Maps directory paths to arrays of Script objects
   - Special "global" key for directory-agnostic scripts

### Storage Structure

Scripts are stored in a JSON file with this structure:
```json
{
  "global": [Script, ...],
  "/absolute/path/to/dir": [Script, ...],
  ...
}
```

Each Script object contains:
- `name`: Script identifier
- `command`: The command to execute
- `placeholders`: Array of placeholder variables
- `description`: Human-readable description

## Current Implementation Status

- ✅ Basic CLI structure with Cobra
- ✅ `add` command with global/local scoping
- ✅ JSON-based storage system
- ❌ TUI interface (planned with bubbletea)
- ❌ Script execution with placeholder substitution
- ❌ Shell autocompletion for zsh
- ❌ Project-specific `.scripto/scripts.json` support

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/pflag` - Flag parsing (cobra dependency)
- `github.com/inconshreveable/mousetrap` - Windows console handling (cobra dependency)

## Testing Guidelines

- For all testing via terminal commands YOU MUST provide SCRIPTO_CONFIG environment variable with path to a local scripto.json file YOU MUST NEVER touch the production ~/.scripto/* directory. 
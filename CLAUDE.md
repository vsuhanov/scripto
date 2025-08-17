# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

- **Build**: `go build -o bin/scripto .` - Builds the scripto binary to bin/scripto (main.go is in root)
- **Run**: `go run .` - Runs the application directly
- **Test**: `go test ./...` - Runs all tests (currently no test files exist)
- **Install**: `go install .` - Installs scripto to $GOPATH/bin
- **Version**: Built binaries support `--version` flag showing build version

## Architecture Overview

Scripto is a Go CLI application for managing and executing custom scripts with placeholder support and an interactive TUI. The application uses:

- **CLI Framework**: Cobra for command-line structure and argument parsing
- **TUI Framework**: Bubbletea and Lipgloss for interactive terminal interface
- **Storage**: JSON file at `~/.scripto/scripts.json` for script persistence
- **Shell Integration**: Zsh function wrapper for proper script execution context

### Key Components

1. **Main Entry Point**: `main.go` - Handles version flag and delegates to commands.Execute()

2. **Commands Package**: `commands/` - All CLI command implementations
   - `root.go` - Root command with TUI integration and script execution
   - `add.go` - Script addition functionality with global/local scoping
   - `completion.go` - Shell completion system
   - `install.go` - Shell integration installer

3. **TUI Package**: `internal/tui/` - Complete terminal user interface
   - `model.go` - Main TUI state management and event handling
   - `list.go` - Script list view with scope indicators and navigation
   - `preview.go` - Script preview pane with detailed information
   - `edit_popup.go` - Inline editing form with fields for all script properties
   - `styles.go` - Consistent visual styling and theming
   - `tui.go` - TUI initialization and result handling

4. **Storage Layer**: `internal/storage/storage.go` - Configuration and file management:
   - JSON config at `~/.scripto/scripts.json` (configurable via SCRIPTO_CONFIG)
   - Individual script files in `~/.scripto/scripts/` directory
   - Directory-based script organization with scope hierarchy

5. **Script Management**: `internal/script/matcher.go` - Script discovery and matching:
   - Hierarchical scope search (local → parent → global)
   - Name-based and partial command matching
   - Confidence scoring for best match selection

6. **Shell Integration**: `commands/scripts/scripto.zsh` - Zsh function wrapper:
   - Proper script execution in current shell context
   - External editor support with SCRIPTO_EDITOR
   - Command output handling via file descriptors

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
- `name`: Script identifier (can be empty for unnamed scripts)
- `command`: The command to execute
- `placeholders`: Array of placeholder variables for dynamic substitution
- `description`: Human-readable description
- `file_path`: Path to individual script file (optional)

## Current Implementation Status

### ✅ Completed Features

- **CLI Framework**: Complete Cobra-based command structure
- **Interactive TUI**: Full bubbletea interface with two-pane layout
- **Script Management**: Add, edit, delete operations with confirmation
- **Inline Editing**: Popup form editor with all script properties
- **External Editor**: Integration with configurable editors (vim, nvim, code, etc.)
- **Scope System**: Local, parent directory, and global script organization
- **Smart Execution**: Name matching and partial command matching
- **Shell Integration**: Zsh function wrapper with proper context execution
- **Completion System**: Shell autocompletion for script names and commands
- **File Management**: Individual script files with automatic cleanup
- **Safety Features**: Duplicate prevention, quit protection, confirmation prompts
- **Version Support**: --version flag with build-time version injection
- **Release Automation**: GitHub Actions for ARM64 macOS binary releases

### 🔄 Partially Implemented

- **Testing**: Basic smoke tests, needs comprehensive test suite
- **Cross-platform**: Currently ARM64 macOS only, other platforms planned

### ❌ Planned Features

- **Name Editing**: In-TUI script name editing (placeholder exists)
- **Scope Toggling**: In-TUI scope changes (placeholder exists)
- **Script Templates**: Predefined script templates and snippets
- **Execution History**: Track and replay previous executions
- **Import/Export**: Backup and sharing functionality
- **Multi-platform**: Linux, Windows, Intel Mac binaries

## Dependencies

### Main Dependencies
- `github.com/charmbracelet/bubbletea` - TUI framework for interactive interface
- `github.com/charmbracelet/lipgloss` - Terminal styling and layout
- `github.com/spf13/cobra` - CLI framework and command structure

### Transitive Dependencies  
- `github.com/spf13/pflag` - Flag parsing (cobra dependency)
- `github.com/inconshreveable/mousetrap` - Windows console handling (cobra dependency)
- Various charmbracelet ecosystem packages for terminal handling

## Logging

- **Logging Strategy**: Use log package for log output
- **Log File Location**: `/tmp/scripto.log` - Standard log file for application logs

## Testing Guidelines

- **CRITICAL**: For all testing via terminal commands YOU MUST provide SCRIPTO_CONFIG environment variable with path to a local scripto.json file YOU MUST NEVER touch the production ~/.scripto/* directory
- **Example**: `SCRIPTO_CONFIG="./test-config.json" ./bin/scripto`
- **TUI Testing**: Use custom config to test TUI without affecting real scripts
- **Shell Integration**: Test shell function separately from production wrapper

## TUI Usage

### Key Bindings
- **Navigation**: `j`/`k` or arrow keys to move, `tab` to switch panes
- **Actions**: `enter` to execute, `e` for inline edit, `E` for external editor
- **Management**: `d` for delete (with confirmation), `D` for immediate delete
- **Help**: `?` to toggle help screen, `q` to quit

### Edit Popup (Press `e`)
- **Fields**: Name, Description, Command (multi-line), Global checkbox
- **Navigation**: `tab`/`shift+tab` between fields, `enter` to save/toggle
- **Exit**: `esc` to cancel, Save button to confirm changes

### External Editor (Press `E`)
- **Configuration**: Set `SCRIPTO_EDITOR` or uses `$EDITOR` (defaults to `vi`)
- **Workflow**: Opens script file → edit → save → prompt to execute
- **Shell Integration**: Returns to TUI after editor exits

## Release Process

- **Automation**: GitHub Actions build ARM64 macOS binaries on version tags
- **Versioning**: Use semantic versioning tags like `v1.0.0`
- **Process**: Update CHANGELOG.md → commit → tag → push tag
- **Output**: GitHub release with binary, archive, checksums, and documentation

## Development Best Practices

- When needing to use a color ONLY use colors defined in the Colors in the internal/tui/colors.go files. 

# Important Instructions for Claude Code

## Core Guidelines
- Do what has been asked; nothing more, nothing less
- NEVER create files unless they're absolutely necessary for achieving your goal
- ALWAYS prefer editing an existing file to creating a new one
- NEVER proactively create documentation files (*.md) or README files unless explicitly requested

## Testing Safety
- **ALWAYS** use SCRIPTO_CONFIG environment variable for testing
- **NEVER** modify or touch ~/.scripto/* directory during development
- Use test fixtures in test/ directory for any test data needed

## Environment Variables

### SCRIPTO_SHELL_HISTORY_FILE_PATH
- **Purpose**: Source of truth for command history in TUI screens
- **Usage**: Shell wrapper sets this to provide command history to history selection screens
- **Format**: File contains fc output format: "  123  command here" with line numbers
- **Important**: ALL history screens MUST read from this variable, not directly from shell history files
- **Implementation**: Used by `internal/tui/history_screen.go` for consistent history access 
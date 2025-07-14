# Scripto

A powerful CLI tool for managing and executing custom scripts with placeholder support and an interactive TUI interface.

Scripto allows you to store command snippets with placeholders, organize them by scope (local, parent directories, or global), and execute them quickly through either command-line arguments or an intuitive terminal user interface.

## Features

- 🚀 **Interactive TUI** - Browse, edit, and execute scripts with a beautiful terminal interface
- 📁 **Scope Management** - Organize scripts as local (current directory), parent directory, or global
- 🔧 **Inline Editing** - Edit scripts directly in the TUI with form-based popup editor
- 📝 **External Editor Support** - Open scripts in your preferred editor (vim, nvim, code, etc.)
- 🔍 **Smart Matching** - Find scripts by name or partial command matching
- 🏷️ **Placeholder Support** - Use placeholders in scripts for dynamic values
- 🗃️ **Auto-completion** - Shell completion for script names and commands
- 💾 **File-based Storage** - Scripts stored as individual files with JSON metadata
- 🔄 **Shell Integration** - Seamlessly execute scripts in your current shell context

## Installation

### Prerequisites

- A Unix-like shell (bash, zsh, fish)

### Download Pre-built Binary (Recommended)

1. **Download the latest release from GitHub:**
   ```bash
   # Download the latest release (replace with actual download URL)
   curl -L https://github.com/your-username/scripto/releases/latest/download/scripto-darwin-arm64.tar.gz -o scripto.tar.gz
   tar -xzf scripto.tar.gz
   ```

2. **Add to your PATH:**
   ```bash
   # Move to a directory in your PATH
   sudo mv scripto /usr/local/bin/
   
   # Or add to your local bin directory
   mkdir -p ~/bin
   mv scripto ~/bin/
   echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
   ```

3. **Install shell integration:**
   ```bash
   scripto install
   ```

4. **Restart your terminal session** or run:
   ```bash
   source ~/.zshrc
   ```

### Build from Source

For development or if pre-built binaries aren't available:

**Prerequisites:**
- Go 1.23.4 or later

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd scripto
   ```

2. **Build the binary:**
   ```bash
   go build -o bin/scripto ./cmd/scripto
   ```

3. **Install to your PATH:**
   ```bash
   go install ./cmd/scripto
   ```
   
   Or copy the binary manually:
   ```bash
   cp bin/scripto /usr/local/bin/
   ```

### Shell Integration

For full functionality, you need to install the shell wrapper function:

#### Zsh Users
```bash
# Install the completion and wrapper
./bin/scripto install

# Add to your ~/.zshrc
source ~/.scripto/completion.zsh
```

#### Installing with Custom Names

Scripto supports installing under different command names for faster access:

**Quick access with `sc` command:**
```bash
# Install scripto + sc alias
./bin/scripto install --turbo

# Now you can use either:
scripto        # Full command
sc             # Short alias
```

**Custom alias name:**
```bash
# Install with your preferred alias
./bin/scripto install --alias myalias

# Now you can use either:
scripto        # Full command  
myalias        # Your custom alias
```

Both the main `scripto` command and your alias will have full completion support. The alias simply points to the main `scripto` command, so all functionality is identical.

#### Manual Installation
If you prefer manual setup:

1. **Copy the shell function:**
   ```bash
   cp commands/scripts/scripto.zsh ~/.scripto/
   ```

2. **Add to your shell configuration:**
   ```bash
   # For zsh (~/.zshrc)
   source ~/.scripto/scripto.zsh
   
   # For bash (~/.bashrc)
   source ~/.scripto/scripto.sh
   ```

## Usage

### Basic Usage (Quick Start)

The fastest way to get started is to save commands you've already run:

**Save your last command:**
```bash
# After running a command, save it with a name
scripto add --name "build" --global -- "!!"

# Or save it locally (current directory only)
scripto add --name "test" -- "!!"
```

**Execute saved commands:**
```bash
# Run by name
scripto build
scripto test

# Or run the last command again
scripto "!!"
```

**Quick workflow:**
```bash
# 1. Run a command
go build -o bin/myapp ./cmd/myapp

# 2. Save it for later
scripto add --name "build" --global -- "!!"

# 3. Use it anytime
scripto build
```

The `!!` bash expansion refers to your last command, making it easy to save commands you've just tested.

### Interactive TUI (Recommended)

Launch the interactive interface by running scripto without arguments:

```bash
scripto
```

#### TUI Key Bindings

**Navigation:**
- `j`, `↓` - Move down in script list
- `k`, `↑` - Move up in script list  
- `tab` - Switch between panes
- `?` - Toggle help screen

**Actions:**
- `enter` - Execute selected script
- `e` - Edit script inline (popup editor)
- `E` - Edit script in external editor
- `d` - Delete script (with confirmation)
- `D` - Delete script (no confirmation)
- `n` - Add/edit script name *(coming soon)*
- `s` - Toggle script scope *(coming soon)*

**Other:**
- `q`, `ctrl+c` - Quit

#### Inline Editor

Press `e` to open the inline editor popup with these fields:
- **Name** - Script identifier for easy access
- **Description** - Human-readable description
- **Command** - The actual command/script content (multi-line)
- **Global** - Checkbox to make script available globally

**Editor Navigation:**
- `tab`/`shift+tab` - Navigate between fields
- `enter` - Save changes (when on Save button) or toggle checkbox
- `space` - Toggle checkbox
- `esc` - Cancel and close editor

### Command Line Interface

#### Adding Scripts

**Add a global script:**
```bash
scripto add --global --name "backup" --description "Backup home directory" "tar -czf backup-$(date +%Y%m%d).tar.gz ~/"
```

**Add a local script:**
```bash
scripto add --name "build" "go build -o bin/myapp ./cmd/myapp"
```

**Add a script with placeholders:**
```bash
scripto add --name "deploy" --description "Deploy to server" "scp %file:File to deploy% user@%server:Target server%:~/apps/"
```

#### Executing Scripts

**By name:**
```bash
scripto build
scripto deploy myapp.zip production-server

# Or use your alias (if installed with --turbo or --alias)
sc build
myalias deploy myapp.zip production-server
```

**By partial command:**
```bash
scripto "go build"  # Matches scripts starting with "go build"
```

**Interactive execution:**
```bash
scripto  # Opens TUI for selection
```

#### Managing Scripts

**List all scripts:**
```bash
scripto completion  # Shows available scripts
```

**View help:**
```bash
scripto --help
scripto add --help
```

### Script Scopes

Scripto organizes scripts in three scopes:

1. **Local** - Available only in the current directory
2. **Parent** - Available in current directory and parent directories
3. **Global** - Available everywhere

Scripts are searched in this priority order: Local → Parent → Global

### Placeholder Support

Use placeholders in your scripts for dynamic values:

```bash
# Placeholder syntax: %variable:description%
scripto add --name "ssh-connect" "ssh %user:Username%@%host:Hostname%"

# When executed, you'll be prompted:
# Username: myuser
# Hostname: myserver.com
# Final command: ssh myuser@myserver.com
```

### Environment Variables

- `SCRIPTO_CONFIG` - Custom path for scripts configuration file
- `SCRIPTO_EDITOR` - Preferred editor for external editing (defaults to `$EDITOR`, then `vi`)
- `SCRIPTO_CMD_FD` - Internal use for shell integration

## Configuration

Scripts are stored in `~/.scripto/scripts.json` by default. You can customize this location with the `SCRIPTO_CONFIG` environment variable.

The configuration file structure:
```json
{
  "global": [
    {
      "name": "backup",
      "command": "tar -czf backup.tar.gz ~/",
      "description": "Backup home directory",
      "placeholders": [],
      "file_path": "/Users/you/.scripto/scripts/abc123_backup.zsh"
    }
  ],
  "/path/to/project": [
    {
      "name": "build",
      "command": "go build -o bin/app ./cmd/app",
      "description": "Build the application",
      "placeholders": [],
      "file_path": "/Users/you/.scripto/scripts/def456_build.zsh"
    }
  ]
}
```

## Examples

### Development Workflow

```bash
# Add project-specific scripts
cd ~/my-project
scripto add --name "test" "go test ./..."
scripto add --name "build" "go build -o bin/app ./cmd/app"
scripto add --name "run" "./bin/app"

# Add deployment script with placeholders
scripto add --name "deploy" --description "Deploy to environment" \
  "docker build -t myapp:%version:Version tag% . && docker push myapp:%version% && kubectl set image deployment/myapp myapp=myapp:%version% -n %env:Environment%"

# Use the TUI to manage and execute
scripto
```

### System Administration

```bash
# Add global utility scripts
scripto add --global --name "ports" --description "Show listening ports" "netstat -tlnp | grep LISTEN"
scripto add --global --name "disk-usage" --description "Show disk usage" "df -h"
scripto add --global --name "top-memory" --description "Top memory consumers" "ps aux --sort=-%mem | head"

# Quick access from anywhere
scripto ports
scripto disk-usage
```

### Docker Operations

```bash
# Container management scripts
scripto add --name "docker-clean" --description "Clean up Docker" \
  "docker system prune -f && docker volume prune -f"

scripto add --name "docker-logs" --description "Follow container logs" \
  "docker logs -f %container:Container name%"

# Execute with TUI
scripto  # Select docker-logs, enter container name when prompted
```

## Troubleshooting

### Common Issues

**Scripts not executing in current shell context:**
- Ensure you've installed the shell wrapper function
- Source the completion file in your shell configuration

**Completion not working:**
- Run `scripto install` to set up completion
- Restart your shell or source your configuration file

**Scripts not found:**
- Check if you're in the correct directory for local scripts
- Use `scripto` (TUI) to see all available scripts and their scopes

**External editor not opening:**
- Set `SCRIPTO_EDITOR` environment variable: `export SCRIPTO_EDITOR=nvim`
- Ensure your editor is in your PATH

### Debug Mode

For debugging, you can check the configuration:
```bash
# View current config location
echo $SCRIPTO_CONFIG

# Use custom config for testing
SCRIPTO_CONFIG="./test-config.json" scripto
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Changelog

### Current Version
- ✅ Interactive TUI with two-pane layout
- ✅ Inline script editing with popup forms
- ✅ External editor integration
- ✅ Script deletion with confirmation
- ✅ Smart script matching and execution
- ✅ Shell integration and completion
- ✅ Scope-based script organization

### Planned Features
- 🔄 Script name editing in TUI
- 🔄 Scope toggling in TUI  
- 🔄 Script templates and snippets
- 🔄 Script execution history
- 🔄 Import/export functionality
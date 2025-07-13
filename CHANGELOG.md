# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.0.1] - 2025-07-13

### Added
- **Interactive TUI** with two-pane layout showing script list and preview
- **Inline script editing** with popup forms for name, description, command, and scope
- **External editor integration** (configurable via SCRIPTO_EDITOR environment variable)
- **Script management** with delete operations (d for confirmation, D for immediate)
- **Smart script execution** with exact name and partial command matching
- **Scope-based organization** supporting local, parent directory, and global scripts
- **Shell integration** with zsh completion and function wrapper
- **Placeholder support** for dynamic script values with user prompts
- **CLI commands** for adding scripts with various options
- **Version information** accessible via --version flag
- **Comprehensive documentation** with README and usage examples

### Features
- **Navigation**: j/k or arrow keys for movement, tab to switch panes
- **Script Operations**: Enter to execute, e for inline edit, E for external editor
- **Form Editing**: Tab navigation, checkbox toggles, multi-line text areas
- **Safety Features**: Confirmation prompts, duplicate prevention, quit protection
- **File Management**: Automatic script file creation and cleanup
- **Cross-scope Support**: Move scripts between local, parent, and global scopes

### Technical Details
- Built with Go 1.23.4 and bubbletea TUI framework
- JSON-based configuration storage at ~/.scripto/scripts.json
- Shell function wrapper for proper script execution context
- ARM64 macOS binary support with GitHub Actions automation
- Comprehensive error handling and user feedback

This initial release provides a complete script management solution with both command-line and interactive interfaces.

## [v1.0.0] - TBD

### Added
- Initial release with core functionality
- CLI commands for adding and executing scripts
- JSON-based configuration storage
- Basic script file management

---

## Release Notes Template

When creating a new release, copy this template and fill in the details:

```markdown
## [vX.Y.Z] - YYYY-MM-DD

### Added
- New features and capabilities

### Changed  
- Changes to existing functionality

### Deprecated
- Features that will be removed in future versions

### Removed
- Features that have been removed

### Fixed
- Bug fixes and corrections

### Security
- Security-related improvements
```
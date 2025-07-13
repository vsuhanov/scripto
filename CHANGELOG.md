# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Interactive TUI with two-pane layout (script list and preview)
- Inline script editing with popup forms
- External editor integration (configurable via SCRIPTO_EDITOR)
- Script deletion with confirmation options (d/D keys)
- Smart script matching and execution
- Scope-based script organization (local, parent, global)
- Shell integration with zsh completion
- Placeholder support for dynamic script values

### Changed
- Moved external editor functionality from 'e' to 'E' key
- Enhanced script matching with better duplicate prevention
- Improved quit handling to prevent accidental script execution

### Fixed
- Duplicate script creation bug in edit popup
- Script execution when quitting with 'q' key
- Multiple save attempts in edit form

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
This document outlines the design for the `scripto` application.

### 1. Core Technology

The application will be implemented in **Go**.
- **CLI Framework:** `cobra` will be used for command-line argument parsing and command structure.
- **TUI Framework:** `bubbletea` will be used for the interactive user interface.

### 2. Storage Strategy

To facilitate easy sharing and backup via Git, all scripts will be stored in a **single JSON file** located at `~/.scripto/scripts.json`.

- **Structure:** The JSON file will contain a top-level map.
    - The keys of the map will be the absolute paths of directories where scripts were added.
    - A special key named `global` will be used to store scripts accessible from any directory.
    - The value for each key will be an array of script objects.
- **Future Expansion:** In the future, the application may be enhanced to also read a `.scripto/scripts.json` file from the current working directory (or parent directories) to allow for project-specific, shareable script configurations.

### 3. `add` Command

- **Usage:** `scripto add [flags] <command>`
- **Functionality:**
    - The command to be stored is passed as the final argument.
    - Placeholders in the format `{variable:description}` will be parsed from the command and stored.
    - The `--global` flag will assign the script to the `global` scope instead of the current directory's scope.

### 4. Interactive UI

- **Invocation:** Running `scripto` with no arguments will launch the TUI.
- **Layout:** The UI will feature panels displaying:
    - A list of scripts available in the current context (local, parent directories, global).
    - Details for the currently selected script, including its description and required placeholders.

### 5. Shell Autocompletion

- **Functionality:** The application will provide dynamic shell completion for `zsh`.
- **Behavior:** When a user types `scripto ` and presses the `Tab` key, a list of available script names will be suggested, ordered by scope: current directory, parent directories, and finally global.
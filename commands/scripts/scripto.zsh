# Load shortcuts function - sources all function files from bin directory
scripto_load_shortcuts() {
    local bin_dir="$HOME/.scripto/bin"
    [ -n "$SCRIPTO_CONFIG" ] && bin_dir="${SCRIPTO_CONFIG:h}/bin"
    if [ -d "$bin_dir" ]; then
        setopt local_options null_glob
        for func_file in "$bin_dir"/*.zsh; do
            [ -f "$func_file" ] && source "$func_file"
        done
    fi
}

scripto_load_shortcuts

# Shell history tracking - armed by SCRIPTO_TRACK_HISTORY
autoload -Uz add-zsh-hook
zmodload -F zsh/datetime p:EPOCHSECONDS p:EPOCHREALTIME 2>/dev/null

typeset -g _scripto_hist_id=""
typeset -g _scripto_hist_cmd=""
typeset -g _scripto_hist_cwd=""
typeset -g _scripto_hist_started=""

_scripto_should_record() {
    local cmd="$1"
    [[ "$cmd" == ' '* ]] && return 1
    [[ -z "${cmd//[[:space:]]/}" ]] && return 1

    local -a words
    words=(${(z)cmd})
    local first="${words[1]}"

    if [[ -n "${aliases[$first]}" ]]; then
        local -a expanded
        expanded=(${(z)aliases[$first]})
        first="${expanded[1]}"
    fi

    if [[ "$first" == "scripto" ]]; then
        case "${words[2]}" in
            ""|add|install|cli|completion|edit|help|--help|-h|--version|-v|--migrate|__*) return 1 ;;
        esac
        return 0
    fi

    local -a ignore
    ignore=(${=SCRIPTO_HISTORY_IGNORE:-ls cd pwd clear exit history fg bg jobs})
    (( ${ignore[(Ie)$first]} )) && return 1
    return 0
}

_scripto_preexec() {
    [[ -n "$SCRIPTO_TRACK_HISTORY" ]] || return
    _scripto_should_record "$1" || return
    _scripto_hist_id="$$-${EPOCHREALTIME}-${RANDOM}"
    _scripto_hist_cmd="$1"
    _scripto_hist_cwd="$PWD"
    _scripto_hist_started="$EPOCHSECONDS"
}

_scripto_precmd() {
    local code=$?
    [[ -n "$_scripto_hist_id" ]] || return
    command scripto __record-history --id "$_scripto_hist_id" --cwd "$_scripto_hist_cwd" \
        --session "$$" --started "$_scripto_hist_started" --finished "$EPOCHSECONDS" \
        --exit "$code" -- "$_scripto_hist_cmd" >/dev/null 2>&1 &!
    _scripto_hist_id=""
    _scripto_hist_cmd=""
}

add-zsh-hook preexec _scripto_preexec
# prepended rather than registered via add-zsh-hook so $? is read before any
# other precmd hook (starship, p10k, vcs_info) overwrites it
precmd_functions=(_scripto_precmd ${precmd_functions:#_scripto_precmd})

scripto() {
    # Create a temporary file for command communication
    local cmd_file=$(mktemp)
    local exec_id_file=$(mktemp)
    
    # Check if this is a "scripto add" command with no additional arguments
    # and generate shell history file if needed
    local history_file=""
    if [ "$1" = "add" ] && [ $# -eq 1 ]; then
        # Create temporary file for shell history
        history_file=$(mktemp)
        
        # Generate command history using fc and save to file
        fc -l -100 2>/dev/null > "$history_file" || true
        
        # Set environment variable for scripto to find the history file
        export SCRIPTO_SHELL_HISTORY_FILE_PATH="$history_file"
    fi
    
    # Run scripto with custom descriptor, allow normal interaction
    SCRIPTO_CMD_FD="$cmd_file" SCRIPTO_EXEC_ID_FILE="$exec_id_file" command scripto "$@"
    local exit_code=$?
    local exec_id=""
    [ -s "$exec_id_file" ] && exec_id=$(<"$exec_id_file")
    rm -f "$exec_id_file"
    
    # Clean up history file if it was created
    if [ -n "$history_file" ]; then
        rm -f "$history_file"
        unset SCRIPTO_SHELL_HISTORY_FILE_PATH
    fi
    
    # Check if a command was written to the file
    if [ $exit_code -eq 0 ] && [ -s "$cmd_file" ]; then
        # Source the command file directly - it contains the actual command to execute
        # echo "going to source $cmd_file"
        # cat $cmd_file
        source "$cmd_file"
        local source_exit=$?
        rm -f "$cmd_file"
        if [ -n "$exec_id" ]; then
            command scripto __record-execution --id "$exec_id" --exit "$source_exit" \
                --finished "$EPOCHSECONDS" >/dev/null 2>&1 &!
        fi
        # Load shortcuts after script execution
        scripto_load_shortcuts
        return $source_exit
    elif [ $exit_code -eq 0 ]; then
        rm -f "$cmd_file"
        return 0
    elif [ $exit_code -eq 3 ]; then
        # Built-in command completed - cleanup and return success
        rm -f "$cmd_file"
        # Load shortcuts after built-in commands (like add)
        scripto_load_shortcuts
        return 0
    elif [ $exit_code -eq 4 ] && [ -s "$cmd_file" ]; then
        # Edit mode - read script path and open in editor
        local script_path=$(cat "$cmd_file")
        rm -f "$cmd_file"
        
        # Determine which editor to use
        local editor="${SCRIPTO_EDITOR:-${EDITOR:-vi}}"
        
        # Open the script file in the editor
        "$editor" "$script_path"
        local editor_exit=$?
        
        # After editing, prompt user if they want to execute the script
        echo -n "Execute the edited script? (y/N): "
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            # Execute the script
            if [[ "$script_path" == *".zsh" ]] || [[ "$script_path" == *".sh" ]]; then
                source "$script_path"
                local exec_exit=$?
                # Load shortcuts after script execution
                scripto_load_shortcuts
                return $exec_exit
            else
                # Fallback to reading file content and eval
                local script_content=$(cat "$script_path" 2>/dev/null)
                if [ -n "$script_content" ]; then
                    eval "$script_content"
                    local eval_exit=$?
                    # Load shortcuts after script execution
                    scripto_load_shortcuts
                    return $eval_exit
                fi
            fi
        fi
        
        # Load shortcuts even if script wasn't executed (in case changes were made)
        scripto_load_shortcuts
        return $editor_exit
    else
        # Error occurred - cleanup and return error
        # rm -f "$cmd_file"
        return $exit_code
    fi
}

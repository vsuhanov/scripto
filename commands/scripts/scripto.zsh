scripto() {
    # Create a temporary file for command communication
    local cmd_file=$(mktemp)
    
    # Run scripto with custom descriptor, allow normal interaction
    SCRIPTO_CMD_FD="$cmd_file" command scripto "$@"
    local exit_code=$?
    
    # Check if a command was written to the file
    if [ $exit_code -eq 0 ] && [ -s "$cmd_file" ]; then
        # Script execution - read the script path and source it
        local script_path=$(cat "$cmd_file")
        
        # Check if it's a file path (starts with / or contains .zsh/.sh extension)
        if [[ "$script_path" == /* ]] || [[ "$script_path" == *".zsh" ]] || [[ "$script_path" == *".sh" ]]; then
            # Source the script file in current shell context
            source "$script_path"
            local source_exit=$?
            rm -f "$cmd_file"
            return $source_exit
        else
            # Fallback: treat as direct command for backward compatibility
            eval "$script_path"
            local eval_exit=$?
            rm -f "$cmd_file"
            return $eval_exit
        fi
    elif [ $exit_code -eq 3 ]; then
        # Built-in command completed - cleanup and return success
        rm -f "$cmd_file"
        return 0
    else
        # Error occurred - cleanup and return error
        rm -f "$cmd_file"
        return $exit_code
    fi
}
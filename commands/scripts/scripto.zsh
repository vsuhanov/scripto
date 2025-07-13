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
                return $?
            else
                # Fallback to reading file content and eval
                local script_content=$(cat "$script_path" 2>/dev/null)
                if [ -n "$script_content" ]; then
                    eval "$script_content"
                    return $?
                fi
            fi
        fi
        
        return $editor_exit
    else
        # Error occurred - cleanup and return error
        rm -f "$cmd_file"
        return $exit_code
    fi
}
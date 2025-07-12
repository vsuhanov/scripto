#compdef scripto
compdef _scripto scripto
# zsh completion for scripto                              -*- shell-script -*-

__scripto_debug()
{
    local file="$BASH_COMP_DEBUG_FILE"
    if [[ -n ${file} ]]; then
        echo "$*" >> "${file}"
    fi
}

_scripto()
{
#    local context state line
#
#    if [[ $#words -eq 2 ]]; then
#      local -a database_commands file_commands network_commands
#
#      database_commands=(
#        'connect:Connect to database'
#        'query:Execute SQL query'
#        'backup:Create database backup'
#      )
#
#      file_commands=(
#        'copy:Copy files'
#        'move:Move files'
#        'delete:Remove files'
#      )
#
#      network_commands=(
#        'ping:Test network connectivity'
#        'download:Download files'
#        'upload:Upload files'
#      )
#
#      # Show groups with proper separation
#      _describe -t database -V 'Database Commands' database_commands
#      _describe -t files -V 'File Operations' file_commands
#      _describe -t network -V 'Network Tools' network_commands
#    fi
#    return 0
#    local context state line
#
#    # If no arguments yet, show grouped commands
#    if [[ $#words -eq 2 ]]; then
#      local -a all_commands
#
#      # Add each group separately with headers
#      compadd -X "Database Commands:" connect query backup
#      compadd -X "File Operations:" copy move delete
#      compadd -X "Network Tools:" ping download upload
#
#      # Add descriptions for each command
#      compadd -Q -d '(
#        "Connect to database"
#        "Execute SQL query"
#        "Create database backup"
#        "Copy files"
#        "Move files"
#        "Remove files"
#        "Test network connectivity"
#        "Download files"
#        "Upload files"
#      )' connect query backup copy move delete ping download upload
#    fi
#    return 0
#    local context state line
#
#    # Define command groups
#    local -a database_commands database_descriptions
#    local -a file_commands file_descriptions
#    local -a network_commands network_descriptions
#
#    database_commands=(connect query backup)
#    database_descriptions=(
#      'Connect to database'
#      'Execute SQL query'
#      'Create database backup'
#    )
#
#    file_commands=(copy move delete)
#    file_descriptions=(
#      'Copy files'
#      'Move files'
#      'Remove files'
#    )
#
#    network_commands=(ping download upload)
#    network_descriptions=(
#      'Test network connectivity'
#      'Download files'
#      'Upload files'
#    )
#
#    # Add completions with group headers using compadd directly
#    compadd -X "Database Commands:" -d database_descriptions -a database_commands
#    compadd -X "File Operations:" -d file_descriptions -a file_commands
#    compadd -X "Network Tools:" -d network_descriptions -a network_commands
#    return 0
#
#  local context state line
#
#  # Define command groups - use separate arrays for commands and descriptions
#  local -a database_commands database_descriptions
#  local -a file_commands file_descriptions
#  local -a network_commands network_descriptions
#
#  database_commands=(connect query backup)
#  database_descriptions=(
#    'Connect to database'
#    'Execute SQL query'
#    'Create database backup'
#  )
#
#  file_commands=(copy move delete)
#  file_descriptions=(
#    'Copy files'
#    'Move files'
#    'Remove files'
#  )
#
#  network_commands=(ping download upload)
#  network_descriptions=(
#    'Test network connectivity'
#    'Download files'
#    'Upload files'
#  )
#
#  # Use _alternative to group and display commands
#  _alternative \
#    'database:Database Commands:compadd -d database_descriptions -a database_commands' \
#    'files:File Operations:compadd -d file_descriptions -a file_commands' \
#    'network:Network Tools:compadd -d network_descriptions -a network_commands'
#
#return  0
#    local context state line
#
#    # Define command groups
#    local -a database_commands
#    local -a file_commands
#    local -a network_commands
#
#    database_commands=(
#      'connect:Connect to database'
#      'query:Execute SQL query'
#      'backup:Create database backup'
#    )
#
#    file_commands=(
#      'copy:Copy files'
#      'move:Move files'
#      'delete:Remove files'
#    )
#
#    network_commands=(
#      'ping:Test network connectivity'
#      'download:Download files'
#      'upload:Upload files'
#    )
#
#    # Use _alternative to group and display commands
#    _alternative \
#      'database:Database Commands:_describe "database commands" database_commands' \
#      'files:File Operations:_describe "file operations" file_commands' \
#      'network:Network Tools:_describe "network tools" network_commands'
#
#      return 0
#  local context state line
#
#  # Define command groups with descriptions
#  local -a database_commands database_descriptions
#  local -a file_commands file_descriptions
#  local -a network_commands network_descriptions
#
#  database_commands=(connect query backup)
#  database_descriptions=(
#    'Connect to database'
#    'Execute SQL query'
#    'Create database backup'
#  )
#
#  file_commands=(copy move delete)
#  file_descriptions=(
#    'Copy files'
#    'Move files'
#    'Remove files'
#  )
#
#  network_commands=(ping download upload)
#  network_descriptions=(
#    'Test network connectivity'
#    'Download files'
#    'Upload files'
#  )
#
#  # Add completions with group separators
#  compadd -X "Database Commands:" -d database_descriptions -a database_commands
#  compadd -X "File Operations:" -d file_descriptions -a file_commands
#  compadd -X "Network Tools:" -d network_descriptions -a network_commands
#  return 0
#   local toComplete="${words[@]:1}"
#   __scripto_debug $toComplete
#
#   local -a completions=(
#     "echo foobar:Run echo foobar"
#     "echo barbaz:description barbaz"
#     "ecto barbaz:description barbaz"
#     "ls -la:List files"
#   )
#
#   for comp in "${completions[@]}"; do
#     local full="${comp%%:*}"
#     local descr=${comp#*:}
#
#     # Check if user's input matches somewhere inside the full command (not necessarily prefix)
#     if [[ "$full" == *"$toComplete"* ]]; then
#       # Calculate what's missing
#       local insertion="${full#$toComplete}"
#
#       # If insertion is empty (exact match), insert full thing
#       [[ -z "$insertion" ]] && insertion="$full"
#
#       __scripto_debug "insertion: $insertion"
#       local -a displayArray=("$full")
#       compadd -U -Q -d displayArray -V "$insertion" -P "$words[CURRENT]" -- "$insertion"
#     fi
#   done
#   return 0
# -------------------------------------------------------------
    #   local toComplete="${words[@]:1}"
#   __scripto_debug $toComplete
#
#   local -a completions=(
#     "echo foobar"
#     "echo barbazbaz"
#     "ecto barbazbaz"
#     "ls -la:List"
#   )
#
#     local -a displays=(
#     "bar1"
#     "bar2"
#     "bar3"
#     "bar4"
#     )
#
#       compadd -d displays  -a completions
#   for comp in "${completions[@]}"; do
#     local full="${comp%%:*}"
#     local descr=${comp#*:}
#
#     # Check if user's input matches somewhere inside the full command (not necessarily prefix)
#     if [[ "$full" == *"$toComplete"* ]]; then
#       # Calculate what's missing
#       local insertion="${full#$toComplete}"
#
#       # If insertion is empty (exact match), insert full thing
#       [[ -z "$insertion" ]] && insertion="$full"
#
#       __scripto_debug "insertion: $insertion"
#       local -a displayArray=("$full")
#       compadd -U -Q -d displayArray -V "$insertion" -P "$words[CURRENT]" -- "$insertion"
#     fi
#   done
#    return 0
# -------------------------------------------------------------

    local shellCompDirectiveError=1
    local shellCompDirectiveNoSpace=2
    local shellCompDirectiveNoFileComp=4
    local shellCompDirectiveFilterFileExt=8
    local shellCompDirectiveFilterDirs=16
    local shellCompDirectiveKeepOrder=32

    local lastParam lastChar flagPrefix requestComp out directive comp lastComp noSpace keepOrder
    
    # Force menu selection for completions
    setopt local_options BASH_REMATCH
    zstyle ':completion:*' menu select
    zstyle ':completion:*' list-colors ''

    __scripto_debug "\n========= starting completion logic =========="
    __scripto_debug "CURRENT: ${CURRENT}, words[*]: ${words[*]}"

    # The user could have moved the cursor backwards on the command-line.
    # We need to trigger completion from the $CURRENT location, so we need
    # to truncate the command-line ($words) up to the $CURRENT location.
    # (We cannot use $CURSOR as its value does not work when a command is an alias.)
    words=("${=words[1,CURRENT]}")
    __scripto_debug "Truncated words[*]: ${words[*]},"

    lastParam=${words[-1]}
    lastChar=${lastParam[-1]}
    __scripto_debug "lastParam: ${lastParam}, lastChar: ${lastChar}"
    
    # Set toComplete for prefix stripping logic
    local toComplete="${words[@]:1}"
#
#    local toComplete=""
#    if [ "${lastChar}" != "" ]; then
#        toComplete="$lastParam"
#    fi
    __scripto_debug "toComplete: ${toComplete}"

    # For zsh, when completing a flag with an = (e.g., scripto -n=<TAB>)
    # completions must be prefixed with the flag
    setopt local_options BASH_REMATCH
    if [[ "${lastParam}" =~ '-.*=' ]]; then
        # We are dealing with a flag with an =
        flagPrefix="-P ${BASH_REMATCH}"
    fi

    # Prepare the command to obtain completions
    requestComp="command ${words[1]} __complete ${words[2,-1]}"
    if [ "${lastChar}" = "" ]; then
        # If the last parameter is complete (there is a space following it)
        # We add an extra empty parameter so we can indicate this to the go completion code.
        __scripto_debug "Adding extra empty parameter"
        requestComp="${requestComp} \"\""
    fi

    __scripto_debug "About to call: eval ${requestComp}"

    # Use eval to handle any environment variables and such
    out=$(eval ${requestComp} 2>/dev/null)
    __scripto_debug "completion output: ${out}"

    # Extract the directive integer following a : from the last line
    local lastLine
    while IFS=$'\n' read -r line; do
        lastLine=${line}
    done < <(printf "%s\n" "${out[@]}")
    __scripto_debug "last line: ${lastLine}"

    if [ "${lastLine[1]}" = : ]; then
        directive=${lastLine[2,-1]}
        # Remove the directive including the : and the newline
        local suffix
        (( suffix=${#lastLine}+2))
        out=${out[1,-$suffix]}
    else
        # There is no directive specified.  Leave $out as is.
        __scripto_debug "No directive found.  Setting do default"
        directive=0
    fi

    __scripto_debug "directive: ${directive}"
    __scripto_debug "completions: ${out}"
    __scripto_debug "flagPrefix: ${flagPrefix}"

    if [ $((directive & shellCompDirectiveError)) -ne 0 ]; then
        __scripto_debug "Completion received error. Ignoring completions."
        return
    fi

    local activeHelpMarker="_activeHelp_ "
    local endIndex=${#activeHelpMarker}
    local startIndex=$((${#activeHelpMarker}+1))
    local hasActiveHelp=0
    
    # Use associative arrays to group completions by group name
    local -A groupedCompletions
    local -a groupNames
    local tab="$(printf '\t')"

    local separator=$'\x1F'  # ASCII Unit Separator (rare character)
    __scripto_debug "===================== out ===================="
    __scripto_debug $out
    __scripto_debug "===================== out ===================="
    while IFS=$'\n' read -r comp; do
        __scripto_debug "===================== out ===================="
        __scripto_debug $comp
        __scripto_debug "===================== out ===================="
        # Check if this is an activeHelp statement (i.e., prefixed with $activeHelpMarker)
#        if [ "${comp[1,$endIndex]}" = "$activeHelpMarker" ];then
#            __scripto_debug "ActiveHelp found: $comp"
#            comp="${comp[$startIndex,-1]}"
#            if [ -n "$comp" ]; then
#                compadd -x "${comp}"
#                __scripto_debug "ActiveHelp will need delimiter"
#                hasActiveHelp=1
#            fi
#
#            continue
#        fi

        if [ -n "$comp" ]; then
            local groupName="" completion="" description=""

            # Split on tabs
            local -a parts
            IFS=$separator read -rA parts <<< "$comp"

            if [ ${#parts[@]} -ge 3 ]; then
                # Format: groupname\tcompletion\tdescription
                # Note: zsh arrays are 1-indexed
                groupName="${parts[1]}"
                completion="${parts[2]}"
                description="${parts[3]}"

                # Escape colons in completion and description for zsh
                completion=${completion//:/\\:}
                description=${description//:/\\:}

                if [[ "$completion" == *"$toComplete"* ]]; then
                  # Calculate what's missing
                  local insertion="${completion#$toComplete}"

                  # If insertion is empty (exact match), insert full thing
                  [[ -z "$insertion" ]] && insertion="$completion"

                  __scripto_debug "insertion: $insertion"
#                  local -a displayArray=("$full")
                  local -a display=("$completion -- $description")
                  local -a insertionA=("$insertion")
#                  compadd -x "--- $groupName ---"
                  compadd -U -Q -d display -a insertionA -P "$words[CURRENT]"
#                  compadd -U -Q -d display -a insertionA -V "$groupName" -x "--- $groupName ---" -P "$words[CURRENT]"
                fi
#                compadd -U -Q  -V "$insertion" -P "$words[CURRENT]" -- "$insertion"
#                # Combine completion and description with : separator for zsh
#                local completionWithDesc="${completion}:${description}"
#
#                __scripto_debug "Parsed group: ${groupName}, completion: ${completion}, description: ${description}"
#
#                # Add to grouped completions using a unique separator
#                if [[ -z "${groupedCompletions[$groupName]}" ]]; then
#                    groupedCompletions[$groupName]="$completionWithDesc"
#                    groupNames+=("$groupName")
#                else
#                    groupedCompletions[$groupName]="${groupedCompletions[$groupName]}${separator}$completionWithDesc"
#                fi
            fi

        fi
    done < <(printf "%s\n" "${out[@]}")

    return 0
    # Add a delimiter after the activeHelp statements, but only if:
    # - there are completions following the activeHelp statements, or
    # - file completion will be performed (so there will be choices after the activeHelp)
#    if [ $hasActiveHelp -eq 1 ]; then
#        if [ ${#groupNames[@]} -ne 0 ] || [ $((directive & shellCompDirectiveNoFileComp)) -eq 0 ]; then
#            __scripto_debug "Adding activeHelp delimiter"
#            compadd -x "--"
#            hasActiveHelp=0
#        fi
#    fi
#
#    if [ $((directive & shellCompDirectiveNoSpace)) -ne 0 ]; then
#        __scripto_debug "Activating nospace."
#        noSpace="-S ''"
#    fi

    if [ $((directive & shellCompDirectiveKeepOrder)) -ne 0 ]; then
        __scripto_debug "Activating keep order."
        keepOrder="-V"
    fi

    if [ $((directive & shellCompDirectiveFilterFileExt)) -ne 0 ]; then
        # File extension filtering
        local filteringCmd
        filteringCmd='_files'
        for filter in ${completions[@]}; do
            if [ ${filter[1]} != '*' ]; then
                # zsh requires a glob pattern to do file filtering
                filter="\*.$filter"
            fi
            filteringCmd+=" -g $filter"
        done
        filteringCmd+=" ${flagPrefix}"

        __scripto_debug "File filtering command: $filteringCmd"
        _arguments '*:filename:'"$filteringCmd"
    elif [ $((directive & shellCompDirectiveFilterDirs)) -ne 0 ]; then
        # File completion for directories only
        local subdir
        subdir="${completions[1]}"
        if [ -n "$subdir" ]; then
            __scripto_debug "Listing directories in $subdir"
            pushd "${subdir}" >/dev/null 2>&1
        else
            __scripto_debug "Listing directories in ."
        fi

        local result
        _arguments '*:dirname:_files -/'" ${flagPrefix}"
        result=$?
        if [ -n "$subdir" ]; then
            popd >/dev/null 2>&1
        fi
        return $result
    else
        __scripto_debug "Calling _describe with grouped completions"
        local foundCompletions=0

        # Set up prefix for partial completions
        local prefixFlag=""
        if [ -n "$toComplete" ]; then
            __scripto_debug "toComplete $toComplete"
            prefixFlag="-P $toComplete"
        fi

        #  local -a groupCompletions=(
        #    "echo foobar:Run echo foobar"
        #    "ls -la:List files"
        #  )
        #  local -a groupActual=(
        #    "o foobar"
        #    "ls -la"
        #  )
        #
        #  _describe 'Commands' groupCompletions groupActual -J 'mygroup'
        #  return 0
        # Call _describe for each group
        for groupName in "${groupNames[@]}"; do
            __scripto_debug "================================ $groupName ========================"
            local -a groupCompletions
            local separator=$'\x1F'  # ASCII Unit Separator (rare character)

            # Split the separator-separated completions for this group
            local groupData="${groupedCompletions[$groupName]}"
            IFS=$separator read -rA groupCompletions <<< "$groupData"

            __scripto_debug "Group: $groupName, completions: ${groupCompletions[*]}"

            if [ ${#groupCompletions[@]} -gt 0 ]; then
                __scripto_debug "Array size - groupCompletions: ${#groupCompletions[@]}"
                __scripto_debug "words[current]: $words[CURRENT]"
                # Call _describe for this group with prefix flag and force menu
                if eval _describe $keepOrder -V "$groupName" groupCompletions $flagPrefix -Q ; then
                    __scripto_debug "_describe found completions for group: $groupName"
                    foundCompletions=1
                else
                    __scripto_debug "_describe failed for group: $groupName (exit code: $?)"
                fi
            fi
        done

#        if [ $foundCompletions -eq 1 ]; then
#            # Return the success of having called _describe
#            return 0
#        else
#            __scripto_debug "_describe did not find completions."
#            __scripto_debug "Checking if we should do file completion."
#            if [ $((directive & shellCompDirectiveNoFileComp)) -ne 0 ]; then
#                __scripto_debug "deactivating file completion"
#
#                # We must return an error code here to let zsh know that there were no
#                # completions found by _describe; this is what will trigger other
#                # matching algorithms to attempt to find completions.
#                # For example zsh can match letters in the middle of words.
#                return 1
#            else
#                # Perform file completion
#                __scripto_debug "Activating file completion"
#
#                # We must return the result of this command, so it must be the
#                # last command, or else we must store its result to return it.
#                _arguments '*:filename:_files'" ${flagPrefix}"
#            fi
#        fi
    fi
}

# don't run the completion function when being source-ed or eval-ed
if [ "$funcstack[1]" = "_scripto" ]; then
    _scripto
fi

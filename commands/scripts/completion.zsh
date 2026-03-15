#compdef scripto

__scripto_debug()
{
    local file="$BASH_COMP_DEBUG_FILE"
    if [[ -n ${file} ]]; then
        echo "$*" >> "${file}"
    fi
}

_scripto() {
  local separator=$'\x1F'
  local out
  out=$(command scripto __complete --more)

  local -a comps display
  local prevGroup prevColor

  while IFS=$separator read -r group comp desc color; do
    [[ -z "$comp" ]] && continue

    if [[ -n "$prevGroup" && "$group" != "$prevGroup" ]]; then
      local label="%B${prevGroup}%b"
      [[ -n "$prevColor" ]] && label="%F{${prevColor}}${label}%f"
      # _describe -t "$prevGroup" "$prevGroup" comps -l -X "$label"
      _describe -t "$prevGroup" "$prevGroup" display comps -l -X "$label" -o nosort
      comps=()
      display=()
    fi

    prevGroup="$group"
    prevColor="$color"
    local displayName="$comp"
    [[ -n "$desc" ]] && displayName="$comp -- $desc"
    comps+=("$comp")
    display+=("$displayName")

  done <<< "$out"

  if [[ ${#comps} -gt 0 ]]; then
    local label="%B${prevGroup}%b"
    [[ -n "$prevColor" ]] && label="%F{${prevColor}}${label}%f"
    _describe -t "$prevGroup" "$prevGroup" display comps -l -X "$label" -o nosort
  fi
}
compdef _scripto scripto
# don't run the completion function when being source-ed or eval-ed
if [ "$funcstack[1]" = "_scripto" ]; then
    _scripto
fi

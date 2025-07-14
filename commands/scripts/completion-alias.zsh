#compdef {{.Alias}}
compdef _{{.Alias}} {{.Alias}}

_{{.Alias}}() {
    _scripto "$@"
}
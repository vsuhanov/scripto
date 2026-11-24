package utils

import (
	"regexp"
	"strings"
)

var shellSafeRe = regexp.MustCompile(`^[a-zA-Z0-9_.,/:@%+=-]+$`)

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func ShellQuoteIfNeeded(s string) string {
	if shellSafeRe.MatchString(s) {
		return s
	}
	return ShellQuote(s)
}

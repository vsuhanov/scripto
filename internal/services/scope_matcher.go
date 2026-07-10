package services

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func IsPatternScope(scope string) bool {
	return scope != "global" && strings.ContainsAny(scope, "*?[")
}

func ScopeMatchesDir(scope, dir string) bool {
	if IsPatternScope(scope) {
		matched, err := doublestar.Match(scope, dir)
		if err != nil {
			return false
		}
		return matched
	}
	return scope == dir
}

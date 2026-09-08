package storage

import (
	"database/sql/driver"
	"fmt"
	"regexp"
	"sync"

	"modernc.org/sqlite"
)

var regexpCache struct {
	sync.Mutex
	pattern  string
	compiled *regexp.Regexp
	err      error
}

func compileCached(pattern string) (*regexp.Regexp, error) {
	regexpCache.Lock()
	defer regexpCache.Unlock()

	if regexpCache.compiled != nil || regexpCache.err != nil {
		if regexpCache.pattern == pattern {
			return regexpCache.compiled, regexpCache.err
		}
	}

	re, err := regexp.Compile(pattern)
	regexpCache.pattern = pattern
	regexpCache.compiled = re
	regexpCache.err = err
	return re, err
}

func valueToString(v driver.Value) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case []byte:
		return string(t), true
	case nil:
		return "", false
	default:
		return fmt.Sprint(t), true
	}
}

// sqliteRegexp backs the SQLite REGEXP operator. SQLite rewrites `X REGEXP Y`
// into the call `regexp(Y, X)`, so args[0] is the pattern and args[1] is the
// subject. An uncompilable pattern yields 0 rather than an error so a
// half-typed pattern degrades to "no matches" instead of failing the query.
func sqliteRegexp(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 2 {
		return int64(0), nil
	}

	pattern, ok := valueToString(args[0])
	if !ok {
		return int64(0), nil
	}
	subject, ok := valueToString(args[1])
	if !ok {
		return int64(0), nil
	}

	re, err := compileCached(pattern)
	if err != nil {
		return int64(0), nil
	}
	if re.MatchString(subject) {
		return int64(1), nil
	}
	return int64(0), nil
}

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("regexp", 2, sqliteRegexp)
}

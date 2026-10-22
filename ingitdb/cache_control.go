package ingitdb

import (
	"fmt"
	"strconv"
	"strings"
)

// CacheControl is a parsed HTTP Cache-Control value (RFC 9111 section 5.2)
// as declared by the optional `cache_control` setting at database level
// (.ingitdb/settings.yaml) or collection level (.collection/definition.yaml).
//
// Directive names are lower-cased. A directive without an argument maps to the
// empty string. Unknown directives are kept in Directives and otherwise
// ignored, as RFC 9111 requires of every cache.
type CacheControl struct {
	// Raw is the declared value, trimmed of surrounding whitespace.
	Raw        string
	Directives map[string]string
}

// deltaSecondsDirectives are the directives whose argument is a non-negative
// integer number of seconds (RFC 9111 section 1.2.2).
var deltaSecondsDirectives = []string{"max-age", "s-maxage", "stale-while-revalidate", "stale-if-error"}

// flagDirectives take no argument.
var flagDirectives = []string{"no-store", "must-revalidate", "proxy-revalidate", "immutable", "public", "no-transform", "must-understand"}

func isTokenChar(r rune) bool {
	switch {
	case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", r)
}

func isToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isTokenChar(r) {
			return false
		}
	}
	return true
}

// splitDirectives splits a Cache-Control value on commas that are outside
// quoted strings.
func splitDirectives(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote, escaped := false, false
	for _, r := range s {
		switch {
		case escaped:
			escaped = false
		case inQuote && r == '\\':
			escaped = true
		case r == '"':
			inQuote = !inQuote
		case r == ',' && !inQuote:
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	return append(parts, cur.String())
}

func isQuotedString(s string) bool {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return false
	}
	body := s[1 : len(s)-1]
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '\\':
			i++
			if i >= len(body) {
				return false
			}
		case '"':
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ParseCacheControl parses and validates a Cache-Control directive string.
// It rejects syntactically invalid input: an empty value, a directive name
// that is not an RFC 9110 token, an unterminated quoted string, a
// delta-seconds directive whose argument is not a non-negative integer, a
// delta-seconds directive without an argument, or a flag directive (for
// example `immutable`) given an argument. Unknown directives are accepted and
// ignored. Duplicate directives are rejected as ambiguous.
func ParseCacheControl(value string) (CacheControl, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return CacheControl{}, fmt.Errorf("cache_control must not be empty")
	}
	cc := CacheControl{Raw: raw, Directives: map[string]string{}}
	for _, part := range splitDirectives(raw) {
		part = strings.TrimSpace(part)
		if part == "" {
			return CacheControl{}, fmt.Errorf("cache_control %q has an empty directive", raw)
		}
		name, arg, hasArg := strings.Cut(part, "=")
		name = strings.ToLower(strings.TrimSpace(name))
		arg = strings.TrimSpace(arg)
		if !isToken(name) {
			return CacheControl{}, fmt.Errorf("cache_control %q has an invalid directive name %q", raw, name)
		}
		if _, dup := cc.Directives[name]; dup {
			return CacheControl{}, fmt.Errorf("cache_control %q repeats directive %q", raw, name)
		}
		if hasArg && !isToken(arg) && !isQuotedString(arg) {
			return CacheControl{}, fmt.Errorf("cache_control %q has an invalid argument for directive %q", raw, name)
		}
		if contains(deltaSecondsDirectives, name) {
			if !hasArg {
				return CacheControl{}, fmt.Errorf("cache_control directive %q requires a number of seconds", name)
			}
			if n, err := strconv.ParseUint(arg, 10, 63); err != nil || n > 1<<62 {
				return CacheControl{}, fmt.Errorf("cache_control directive %q needs a non-negative integer number of seconds, got %q", name, arg)
			}
		}
		if contains(flagDirectives, name) && hasArg {
			return CacheControl{}, fmt.Errorf("cache_control directive %q takes no argument", name)
		}
		cc.Directives[name] = arg
	}
	return cc, nil
}

// ValidateCacheControl reports an error when a non-empty declared value is not
// a valid Cache-Control directive string. An empty value means "not declared"
// and is valid.
func ValidateCacheControl(value string) error {
	if value == "" {
		return nil
	}
	_, err := ParseCacheControl(value)
	return err
}

// Has reports whether the directive is present.
func (c CacheControl) Has(name string) bool {
	_, ok := c.Directives[name]
	return ok
}

// MaxAgeSeconds returns the max-age directive, if declared.
func (c CacheControl) MaxAgeSeconds() (int64, bool) {
	arg, ok := c.Directives["max-age"]
	if !ok {
		return 0, false
	}
	n, _ := strconv.ParseInt(arg, 10, 64) // validated by ParseCacheControl
	return n, true
}

// EffectiveCacheControl resolves the value that applies to a collection: the
// collection-level value when declared, otherwise the database-level value.
// The collection value replaces the database value as a whole; directives are
// never merged. An empty result means nothing is declared.
func EffectiveCacheControl(database, collection string) string {
	if strings.TrimSpace(collection) != "" {
		return strings.TrimSpace(collection)
	}
	return strings.TrimSpace(database)
}

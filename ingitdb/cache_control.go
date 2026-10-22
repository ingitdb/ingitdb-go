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

// MaxDeltaSeconds is the largest delta-seconds value: RFC 9111 section 1.2.2
// tells recipients to treat anything larger as 2^31. Validation rejects a
// larger value; readers clamp it.
const MaxDeltaSeconds int64 = 1 << 31

// ImmutableTTLSeconds is the freshness a SHA-pinned read gets from `immutable`
// when no max-age is declared: one year.
const ImmutableTTLSeconds int64 = 31536000

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

// unquote strips the quotes of a quoted-string argument and resolves its
// backslash escapes; any other argument is returned unchanged.
func unquote(s string) string {
	if !isQuotedString(s) {
		return s
	}
	var b strings.Builder
	body := s[1 : len(s)-1]
	for i := 0; i < len(body); i++ {
		if body[i] == '\\' {
			i++
		}
		b.WriteByte(body[i])
	}
	return b.String()
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// deltaSeconds parses a delta-seconds argument (quoted form accepted), clamped
// to MaxDeltaSeconds. over reports that the clamp applied.
func deltaSeconds(arg string) (n int64, over, ok bool) {
	arg = unquote(arg)
	if !isDigits(arg) {
		return 0, false, false
	}
	n, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || n > MaxDeltaSeconds {
		return MaxDeltaSeconds, true, true
	}
	return n, false, true
}

// ParseCacheControl parses and strictly validates a Cache-Control directive
// string, for authoring time (validate). It rejects: an empty value, an empty
// element, a directive name that is not an RFC 9110 token, a malformed
// argument, a repeated directive, a delta-seconds directive (max-age, s-maxage,
// stale-while-revalidate, stale-if-error) whose argument is missing, not a
// non-negative integer (quoted form accepted) or above MaxDeltaSeconds, and a
// flag directive (for example `immutable`) given an argument. Unknown
// directives are accepted and ignored. Readers use ReadCacheControl instead,
// which never fails.
func ParseCacheControl(value string) (CacheControl, error) {
	return parseCacheControl(value, true)
}

// ReadCacheControl parses a Cache-Control value the way a reader must: leniently.
// A malformed directive is dropped, the first of a repeated directive wins, a
// flag directive given an argument still counts as present, and a delta-seconds
// value above MaxDeltaSeconds is clamped. It never fails; a value with nothing
// usable yields an empty CacheControl.
func ReadCacheControl(value string) CacheControl {
	cc, _ := parseCacheControl(value, false)
	return cc
}

func parseCacheControl(value string, strict bool) (CacheControl, error) {
	raw := strings.TrimSpace(value)
	cc := CacheControl{Raw: raw, Directives: map[string]string{}}
	if raw == "" {
		if strict {
			return CacheControl{}, fmt.Errorf("cache_control must not be empty")
		}
		return cc, nil
	}
	for _, part := range splitDirectives(raw) {
		part = strings.TrimSpace(part)
		if part == "" {
			if strict {
				return CacheControl{}, fmt.Errorf("cache_control %q has an empty directive", raw)
			}
			continue
		}
		name, arg, hasArg := strings.Cut(part, "=")
		name = strings.ToLower(strings.TrimSpace(name))
		arg = strings.TrimSpace(arg)
		if !isToken(name) {
			if strict {
				return CacheControl{}, fmt.Errorf("cache_control %q has an invalid directive name %q", raw, name)
			}
			continue
		}
		if _, dup := cc.Directives[name]; dup {
			if strict {
				return CacheControl{}, fmt.Errorf("cache_control %q repeats directive %q", raw, name)
			}
			continue
		}
		if strict && hasArg && !isToken(arg) && !isQuotedString(arg) {
			return CacheControl{}, fmt.Errorf("cache_control %q has an invalid argument for directive %q", raw, name)
		}
		if contains(deltaSecondsDirectives, name) {
			n, over, ok := deltaSeconds(arg)
			switch {
			case !hasArg && strict:
				return CacheControl{}, fmt.Errorf("cache_control directive %q requires a number of seconds", name)
			case !ok && strict:
				return CacheControl{}, fmt.Errorf("cache_control directive %q needs a non-negative integer number of seconds, got %q", name, arg)
			case over && strict:
				return CacheControl{}, fmt.Errorf("cache_control directive %q is above the maximum of %d seconds", name, MaxDeltaSeconds)
			case !ok:
				continue
			}
			cc.Directives[name] = strconv.FormatInt(n, 10)
			continue
		}
		if contains(flagDirectives, name) {
			if hasArg && strict {
				return CacheControl{}, fmt.Errorf("cache_control directive %q takes no argument", name)
			}
			cc.Directives[name] = ""
			continue
		}
		cc.Directives[name] = unquote(arg)
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

// NoStore reports whether the value forbids storing the file.
func (c CacheControl) NoStore() bool { return c.Has("no-store") }

// NoCache reports whether the value forbids reusing a stored file without revalidation.
func (c CacheControl) NoCache() bool { return c.Has("no-cache") }

// Immutable reports whether the immutable directive is present.
func (c CacheControl) Immutable() bool { return c.Has("immutable") }

// FreshnessSeconds decides how long a reader may reuse a fetched file, in
// seconds; 0 means do not store or reuse it. declared is the effective
// cache_control, origin is the origin's own Cache-Control header ("" when
// absent) and defaultSeconds is the freshness assumed when the origin says
// nothing. pinned must be true only when the caller knows the read is of an
// immutable commit SHA; anything uncertain is unpinned.
//
// An unpinned read may only be tightened by the declared value. A pinned read
// may also be relaxed by it. no-store and no-cache on either side always win.
// must-revalidate needs no handling: readers never serve a stale entry.
func FreshnessSeconds(declared string, pinned bool, origin string, defaultSeconds int64) int64 {
	d := ReadCacheControl(declared)
	o := ReadCacheControl(origin)
	if d.NoStore() || d.NoCache() || o.NoStore() || o.NoCache() {
		return 0
	}
	originMax := defaultSeconds
	if n, ok := o.MaxAgeSeconds(); ok {
		originMax = n
	}
	n, hasMax := d.MaxAgeSeconds()
	switch {
	case pinned && hasMax:
		return n
	case pinned && d.Immutable():
		return ImmutableTTLSeconds
	case hasMax && n < originMax:
		return n
	}
	return originMax
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

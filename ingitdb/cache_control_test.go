package ingitdb

import (
	"strings"
	"testing"
)

func TestParseCacheControl_Valid(t *testing.T) {
	t.Parallel()
	cc, err := ParseCacheControl(`  Public, max-age=31536000 , IMMUTABLE, x-custom="a,b\"c", private=token `)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cc.Has("public") || !cc.Has("immutable") || !cc.Has("x-custom") || cc.Has("no-store") {
		t.Fatalf("unexpected directives: %v", cc.Directives)
	}
	if n, ok := cc.MaxAgeSeconds(); !ok || n != 31536000 {
		t.Fatalf("max-age = %d, %v", n, ok)
	}
	if strings.HasPrefix(cc.Raw, " ") || strings.HasSuffix(cc.Raw, " ") {
		t.Fatalf("Raw not trimmed: %q", cc.Raw)
	}
}

func TestCacheControl_NoMaxAge(t *testing.T) {
	t.Parallel()
	cc, err := ParseCacheControl("no-store")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cc.MaxAgeSeconds(); ok {
		t.Fatal("max-age must be absent")
	}
}

func TestParseCacheControl_UnknownDirectivesIgnored(t *testing.T) {
	t.Parallel()
	if _, err := ParseCacheControl("max-age=60, future-directive=7"); err != nil {
		t.Fatalf("unknown directives must be accepted: %v", err)
	}
}

func TestParseCacheControl_Invalid(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                             "must not be empty",
		"   ":                          "must not be empty",
		"public,,max-age=1":            "empty directive",
		"max-age=1,":                   "empty directive",
		"bad name":                     "invalid directive name",
		"=5":                           "invalid directive name",
		"max-age=5, max-age=6":         "repeats directive",
		`x="unterminated`:              "invalid argument",
		`x="a\"`:                       "invalid argument",
		`x="a"b"`:                      "invalid argument",
		`x=a b`:                        "invalid argument",
		"max-age":                      "requires a number of seconds",
		"max-age=abc":                  "non-negative integer",
		"max-age=-1":                   "non-negative integer",
		"s-maxage=1.5":                 "non-negative integer",
		"max-age=99999999999999999999": "non-negative integer",
		"immutable=1":                  "takes no argument",
		"no-store=yes":                 "takes no argument",
	}
	for in, want := range cases {
		_, err := ParseCacheControl(in)
		if err == nil {
			t.Errorf("%q: expected error", in)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %q lacks %q", in, err, want)
		}
	}
}

func TestParseCacheControl_QuotedEscapes(t *testing.T) {
	t.Parallel()
	if _, err := ParseCacheControl(`x="a\\b"`); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCacheControl(t *testing.T) {
	t.Parallel()
	if err := ValidateCacheControl(""); err != nil {
		t.Fatalf("empty means undeclared: %v", err)
	}
	if err := ValidateCacheControl("max-age=1"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCacheControl("max-age=x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestEffectiveCacheControl(t *testing.T) {
	t.Parallel()
	if got := EffectiveCacheControl(" public, max-age=60 ", ""); got != "public, max-age=60" {
		t.Fatalf("db fallback: %q", got)
	}
	if got := EffectiveCacheControl("public, max-age=60", " no-store "); got != "no-store" {
		t.Fatalf("collection override: %q", got)
	}
	if got := EffectiveCacheControl("", "  "); got != "" {
		t.Fatalf("undeclared: %q", got)
	}
}

func TestCollectionDef_Validate_CacheControl(t *testing.T) {
	t.Parallel()
	def := recordCountBoundsDef(nil, nil)
	def.CacheControl = "public, max-age=60"
	if err := def.Validate(); err != nil {
		t.Fatalf("valid cache_control rejected: %v", err)
	}
	def.CacheControl = "max-age=soon"
	err := def.Validate()
	if err == nil || !strings.Contains(err.Error(), "max-age") {
		t.Fatalf("expected a clear cache_control error, got %v", err)
	}
}

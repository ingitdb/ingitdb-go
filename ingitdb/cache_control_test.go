package ingitdb

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type ccVectors struct {
	Parse []struct {
		Input     string `yaml:"input"`
		Valid     bool   `yaml:"valid"`
		MaxAge    *int64 `yaml:"maxAge"`
		NoStore   bool   `yaml:"noStore"`
		NoCache   bool   `yaml:"noCache"`
		Immutable bool   `yaml:"immutable"`
	} `yaml:"parse"`
	Freshness []struct {
		Name     string  `yaml:"name"`
		Declared *string `yaml:"declared"`
		Ref      string  `yaml:"ref"`
		Origin   *string `yaml:"origin"`
		Default  int64   `yaml:"default"`
		Expect   int64   `yaml:"expect"`
	} `yaml:"freshness"`
}

func loadCCVectors(t *testing.T) ccVectors {
	t.Helper()
	data, err := os.ReadFile("testdata/cache_control_vectors.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var v ccVectors
	if err := yaml.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Runs the shared conformance vectors (ingitdb/ingitdb conformance/cache-control).
func TestCacheControl_ConformanceParse(t *testing.T) {
	t.Parallel()
	v := loadCCVectors(t)
	if len(v.Parse) == 0 {
		t.Fatal("no vectors")
	}
	for _, row := range v.Parse {
		_, err := ParseCacheControl(row.Input)
		if (err == nil) != row.Valid {
			t.Errorf("%q: valid = %v, want %v (err %v)", row.Input, err == nil, row.Valid, err)
		}
		cc := ReadCacheControl(row.Input)
		n, ok := cc.MaxAgeSeconds()
		if ok != (row.MaxAge != nil) || (ok && n != *row.MaxAge) {
			t.Errorf("%q: maxAge = %d,%v, want %v", row.Input, n, ok, row.MaxAge)
		}
		if cc.NoStore() != row.NoStore || cc.NoCache() != row.NoCache || cc.Immutable() != row.Immutable {
			t.Errorf("%q: flags = %v %v %v, want %v %v %v", row.Input, cc.NoStore(), cc.NoCache(), cc.Immutable(), row.NoStore, row.NoCache, row.Immutable)
		}
	}
}

func TestCacheControl_ConformanceFreshness(t *testing.T) {
	t.Parallel()
	v := loadCCVectors(t)
	if len(v.Freshness) == 0 {
		t.Fatal("no vectors")
	}
	for _, row := range v.Freshness {
		got := FreshnessSeconds(deref(row.Declared), row.Ref == "pinned", deref(row.Origin), row.Default)
		if got != row.Expect {
			t.Errorf("%s: got %d, want %d", row.Name, got, row.Expect)
		}
	}
}

func TestParseCacheControl_ErrorMessages(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"   ":                          "must not be empty",
		"public,,max-age=1":            "empty directive",
		"bad name":                     "invalid directive name",
		"max-age=5, max-age=6":         "repeats directive",
		`x="a\"`:                       "invalid argument",
		`x="a"b"`:                      "invalid argument",
		`x="unterminated`:              "invalid argument",
		"max-age":                      "requires a number of seconds",
		"max-age=abc":                  "non-negative integer",
		"max-age=99999999999999999999": "above the maximum",
		"immutable=1":                  "takes no argument",
	}
	for in, want := range cases {
		_, err := ParseCacheControl(in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %v lacks %q", in, err, want)
		}
	}
}

func TestParseCacheControl_ValidKeepsRaw(t *testing.T) {
	t.Parallel()
	cc, err := ParseCacheControl(`  Public, x-custom="a\\b" `)
	if err != nil {
		t.Fatal(err)
	}
	if cc.Raw != `Public, x-custom="a\\b"` || cc.Directives["x-custom"] != `a\b` {
		t.Fatalf("unexpected: %+v", cc)
	}
}

func TestReadCacheControl_Empty(t *testing.T) {
	t.Parallel()
	if cc := ReadCacheControl(" "); len(cc.Directives) != 0 {
		t.Fatalf("unexpected: %+v", cc)
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

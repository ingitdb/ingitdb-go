package ingitdb

import (
	"strings"
	"testing"
)

// REQ:formula-load-time-resolution — an undeclared identifier must be rejected
// when the definition loads, not survive to evaluation. The resolver does this
// for us once compileFormula is given a real is-predeclared predicate.
func TestValidateComputedColumn_RejectsUndeclaredIdentifierAtLoad(t *testing.T) {
	columns := map[string]*ColumnDef{
		"total": {Type: ColumnTypeInt, Formula: "nosuchfield == 1"},
		"count": {Type: ColumnTypeInt},
	}
	err := validateComputedColumn("c", "total", columns["total"], columns)
	if err == nil {
		t.Fatal("expected load-time error for undeclared identifier, got nil")
	}
	if !strings.Contains(err.Error(), "nosuchfield") {
		t.Errorf("error must name the undeclared identifier, got: %v", err)
	}
}

// REQ:formula-load-time-resolution — the predicate must not be so strict that
// it rejects legitimate builtins. A hand-rolled walk over every *syntax.Ident
// would reject these; the resolver does not.
func TestValidateComputedColumn_AllowsBuiltinsAndStoredSiblings(t *testing.T) {
	cases := []struct {
		name    string
		formula string
	}{
		{"len over a stored list", `len(tags) > 0`},
		{"string method on a stored field", `name.startswith("x")`},
		{"comprehension binding a local", `[c for c in counties if c]`},
		{"curated numeric helper", `abs(count)`},
		{"universe constant", `True`},
		{"stored sibling arithmetic", `count * 2`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			columns := map[string]*ColumnDef{
				"derived":  {Type: ColumnTypeAny, Formula: tc.formula},
				"tags":     {Type: ColumnType("[]string")},
				"name":     {Type: ColumnTypeString},
				"counties": {Type: ColumnType("[]string")},
				"count":    {Type: ColumnTypeInt},
			}
			if err := validateComputedColumn("c", "derived", columns["derived"], columns); err != nil {
				t.Errorf("formula %q must load cleanly, got: %v", tc.formula, err)
			}
		})
	}
}

// REQ:formula-load-time-resolution — referencing a computed sibling stays an
// error. With the resolver approach this surfaces because computed columns are
// deliberately absent from the predeclared set.
func TestValidateComputedColumn_RejectsComputedSiblingReference(t *testing.T) {
	columns := map[string]*ColumnDef{
		"a": {Type: ColumnTypeInt, Formula: "b + 1"},
		"b": {Type: ColumnTypeInt, Formula: "1"},
	}
	err := validateComputedColumn("c", "a", columns["a"], columns)
	if err == nil {
		t.Fatal("expected error referencing computed column 'b', got nil")
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error must name the referenced computed column, got: %v", err)
	}
}

// REQ:computed-column-name-not-builtin — a computed column named after a
// universe member is rejected. Without this the rule above silently never
// fires: a name that lives in the universe is never "undefined".
func TestValidateComputedColumn_RejectsBuiltinName(t *testing.T) {
	for _, name := range []string{"len", "abs", "type", "True", "min"} {
		t.Run(name, func(t *testing.T) {
			columns := map[string]*ColumnDef{
				name: {Type: ColumnTypeInt, Formula: "1"},
			}
			err := validateComputedColumn("c", name, columns[name], columns)
			if err == nil {
				t.Fatalf("computed column named %q must be rejected", name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error must name the offending column, got: %v", err)
			}
		})
	}
}

// A stored (non-computed) column may still be named after a builtin: at
// evaluation the field is bound last and shadows the builtin. Only computed
// column names are constrained.
func TestValidateComputedColumn_AllowsStoredColumnNamedAfterBuiltin(t *testing.T) {
	columns := map[string]*ColumnDef{
		"derived": {Type: ColumnTypeInt, Formula: "type + 1"},
		"type":    {Type: ColumnTypeInt},
	}
	if err := validateComputedColumn("c", "derived", columns["derived"], columns); err != nil {
		t.Errorf("stored column named after a builtin must be usable, got: %v", err)
	}
}

// REQ:formula-cache-key-includes-predeclared-set — identical formula source
// compiled against different predeclared sets must not share a cache entry.
// Load order decides which collection compiles first, so a source-only key
// lets a collection that MUST fail take a hit from one that succeeded.
func TestCompileFormula_CacheKeyIncludesPredeclaredSet(t *testing.T) {
	const src = "population > 0"

	// Compiles: 'population' is a stored column here.
	okColumns := map[string]*ColumnDef{
		"derived":    {Type: ColumnTypeBool, Formula: src},
		"population": {Type: ColumnTypeInt},
	}
	if err := validateComputedColumn("has-population", "derived", okColumns["derived"], okColumns); err != nil {
		t.Fatalf("collection declaring 'population' must compile, got: %v", err)
	}

	// Same source, different collection, no such column: must still fail even
	// though the identical source already compiled successfully above.
	badColumns := map[string]*ColumnDef{
		"derived": {Type: ColumnTypeBool, Formula: src},
	}
	err := validateComputedColumn("no-population", "derived", badColumns["derived"], badColumns)
	if err == nil {
		t.Fatal("collection NOT declaring 'population' must fail despite a warm cache entry for the same source")
	}
	if !strings.Contains(err.Error(), "population") {
		t.Errorf("error must name the undeclared identifier, got: %v", err)
	}
}

// The cache must still do its job: same source AND same predeclared set
// returns the identical compiled program rather than recompiling.
func TestCompileFormula_CachesOnIdenticalPredeclaredSet(t *testing.T) {
	extra := []string{"count"}
	p1, err := compileFormulaStrict("count + 1", extra)
	if err != nil {
		t.Fatalf("first compile: %v", err)
	}
	p2, err := compileFormulaStrict("count + 1", extra)
	if err != nil {
		t.Fatalf("second compile: %v", err)
	}
	if p1 != p2 {
		t.Error("identical source and predeclared set must return the cached program")
	}
}

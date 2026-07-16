package ingitdb

import (
	"strings"
	"testing"
)

// REQ:reject-unknown-column-type — an unrecognised column type is rejected at
// definition-load time. ValidateColumnType used to end in a bare `return nil`,
// so everything that was not empty and not a malformed map[ key was accepted.
func TestValidateColumnType_RejectsUnknown(t *testing.T) {
	for _, ct := range []ColumnType{
		// The live case: a valid map *key* type that was never a column type.
		// e2e-test-ingitdb declared it and passed only because unknown types
		// matched every value.
		"number",
		"foo",
		"String",  // case matters
		"integer", // plausible synonym
		"[]",      // list of nothing
		"[]map[string]any",
		"[][]string",
		"[]number",
	} {
		t.Run(string(ct), func(t *testing.T) {
			err := ValidateColumnType(ct)
			if err == nil {
				t.Fatalf("column type %q must be rejected", ct)
			}
			if !strings.Contains(err.Error(), string(ct)) {
				t.Errorf("error must name the offending type, got: %v", err)
			}
		})
	}
}

// Every type the workspace actually uses must stay valid. This list is the
// audit output across all 337 definition files, so a regression here breaks a
// real database.
func TestValidateColumnType_AcceptsEveryTypeInUse(t *testing.T) {
	for _, ct := range []ColumnType{
		"string", "int", "float", "bool", "date", "time", "datetime", "any",
		"map[locale]string",
		"map[string]any",
		"[]string",
	} {
		t.Run(string(ct), func(t *testing.T) {
			if err := ValidateColumnType(ct); err != nil {
				t.Errorf("column type %q is in use and must stay valid, got: %v", ct, err)
			}
		})
	}
}

// An empty type stays distinguishable from an unknown one: the caller maps it
// to "missing 'type' in column definition".
func TestValidateColumnType_EmptyIsMissingNotUnknown(t *testing.T) {
	err := ValidateColumnType("")
	if err == nil {
		t.Fatal("empty column type must be an error")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("empty type must report as missing, got: %v", err)
	}
}

// The map[ key-type check keeps working, and reports the bad key.
func TestValidateColumnType_RejectsBadMapKeyType(t *testing.T) {
	for _, ct := range []ColumnType{"map[]string", "map[float]string", "map[any]string"} {
		t.Run(string(ct), func(t *testing.T) {
			if err := ValidateColumnType(ct); err == nil {
				t.Errorf("column type %q must be rejected", ct)
			}
		})
	}
}

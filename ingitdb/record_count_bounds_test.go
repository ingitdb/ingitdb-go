package ingitdb

import (
	"strings"
	"testing"
)

// recordCountBoundsDef builds a minimal valid CollectionDef with the given
// record-count bounds, so CollectionDef.Validate exercises only the bound
// checks and nothing else fails first.
func recordCountBoundsDef(minCount, maxCount *int) *CollectionDef {
	return &CollectionDef{
		ID:              "widgets",
		Columns:         map[string]*ColumnDef{"name": {Type: ColumnTypeString}},
		RecordFile:      &RecordFileDef{Name: "widgets.yaml", Format: RecordFormatYAML, RecordType: MapOfRecords},
		MinRecordsCount: minCount,
		MaxRecordsCount: maxCount,
	}
}

func rcInt(n int) *int { return &n }

// Verifies record-count-constraints#ac:negative-min-rejected-at-load.
func TestRecordCountBounds_NegativeMinRejected(t *testing.T) {
	t.Parallel()
	err := recordCountBoundsDef(rcInt(-1), nil).Validate()
	if err == nil {
		t.Fatal("a negative min_records_count must be rejected at load")
	}
	for _, want := range []string{"min_records_count", "negative"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must mention %q, got: %v", want, err)
		}
	}
}

// Verifies record-count-constraints#ac:negative-max-rejected-at-load.
func TestRecordCountBounds_NegativeMaxRejected(t *testing.T) {
	t.Parallel()
	err := recordCountBoundsDef(nil, rcInt(-1)).Validate()
	if err == nil {
		t.Fatal("a negative max_records_count must be rejected at load")
	}
	for _, want := range []string{"max_records_count", "negative"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must mention %q, got: %v", want, err)
		}
	}
}

// Verifies record-count-constraints#ac:min-exceeds-max-rejected-at-load.
func TestRecordCountBounds_MinExceedsMaxRejected(t *testing.T) {
	t.Parallel()
	err := recordCountBoundsDef(rcInt(10), rcInt(5)).Validate()
	if err == nil {
		t.Fatal("min_records_count greater than max_records_count must be rejected at load")
	}
	for _, want := range []string{"min_records_count", "max_records_count", "exceeds"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must mention %q, got: %v", want, err)
		}
	}
}

// Valid bounds — including a declared zero max and equal min/max — must pass
// definition-load validation.
func TestRecordCountBounds_ValidBoundsAccepted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		min, max *int
	}{
		{"min only", rcInt(1), nil},
		{"max only", nil, rcInt(100)},
		{"both", rcInt(1), rcInt(100)},
		{"zero max (must be empty)", nil, rcInt(0)},
		{"zero min", rcInt(0), nil},
		{"equal min and max", rcInt(3), rcInt(3)},
		{"neither", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := recordCountBoundsDef(tc.min, tc.max).Validate(); err != nil {
				t.Fatalf("valid bounds must pass load validation, got: %v", err)
			}
		})
	}
}

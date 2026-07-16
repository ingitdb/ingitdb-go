package validator

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// REQ:reject-unknown-column-keys — an unrecognised key in a column definition
// is rejected at load, naming the key. Silently discarding unknown keys is what
// lets a plausible-looking enum: or one_of: appear enforced while doing nothing.
func TestDecodeCollectionDef_RejectsUnknownColumnKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
		yaml string
	}{
		{
			// The real spelling is a collection-level primary_key list. Column-level
			// primaryKey: true is the natural guess and was silently dropped —
			// agiledger-demo and this package's own fixtures both had it.
			name: "primaryKey on a column",
			key:  "primaryKey",
			yaml: "columns:\n  id:\n    type: string\n    primaryKey: true\n",
		},
		{
			name: "unique on a column",
			key:  "unique",
			yaml: "columns:\n  id:\n    type: string\n    unique: true\n",
		},
		{
			// The motivating case from the Feature: a constraint that looks enforced.
			name: "one_of instead of enum",
			key:  "one_of",
			yaml: "columns:\n  state:\n    type: string\n    one_of: [a, b]\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var colDef ingitdb.CollectionDef
			err := decodeCollectionDef([]byte(tc.yaml), &colDef)
			if err == nil {
				t.Fatalf("unknown column key %q must be rejected", tc.key)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("error must name the offending key %q, got: %v", tc.key, err)
			}
		})
	}
}

// Strict decoding is document-wide, not column-only. geo-ingitdb declared all
// of these and none has ever been implemented: an inherits: hierarchy across
// four files and record_labels. They were read and dropped, so the config
// looked live and did nothing.
//
// min_records_count / max_records_count are NOT in this list: they were in the
// same geo-ingitdb sweep, but ingitdb-go#8 implemented them, so they are now
// modelled keys (see TestDecodeCollectionDef_AcceptsModelledKeys).
func TestDecodeCollectionDef_RejectsUnknownCollectionKey(t *testing.T) {
	for _, key := range []string{"inherits", "record_labels", "records_file"} {
		t.Run(key, func(t *testing.T) {
			y := key + ": x\ncolumns:\n  id:\n    type: string\n"
			var colDef ingitdb.CollectionDef
			err := decodeCollectionDef([]byte(y), &colDef)
			if err == nil {
				t.Fatalf("unknown collection key %q must be rejected", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error must name the offending key %q, got: %v", key, err)
			}
		})
	}
}

// The keys the schema does model still decode, including the ones this Feature
// added. Guards against a strictness change that rejects legitimate config.
func TestDecodeCollectionDef_AcceptsModelledKeys(t *testing.T) {
	y := `titles:
  en: Capabilities
primary_key: ["id"]
min_records_count: 1
max_records_count: 100
record_file:
  name: "{key}.json"
  type: "map[string]any"
  format: json
columns:
  id:
    type: string
    required: true
  state:
    type: string
    enum: [native, absent]
  name:
    type: string
    required_when: 'state != "absent"'
  docs:
    type: "[]string"
    min_length: 1
  population:
    type: int
    min_value: 0
    max_value: 99000000
columns_order: [id, state, name]
`
	var colDef ingitdb.CollectionDef
	if err := decodeCollectionDef([]byte(y), &colDef); err != nil {
		t.Fatalf("modelled keys must decode cleanly, got: %v", err)
	}
	if colDef.Columns["name"].RequiredWhen == "" {
		t.Error("required_when must survive decoding")
	}
	if colDef.Columns["state"].Enum == nil {
		t.Error("enum must survive decoding")
	}
	// Verifies record-count-constraints#ac:record-count-bounds-decode-under-strict-fields.
	if colDef.MinRecordsCount == nil || *colDef.MinRecordsCount != 1 {
		t.Error("min_records_count must survive decoding as a modelled key")
	}
	if colDef.MaxRecordsCount == nil || *colDef.MaxRecordsCount != 100 {
		t.Error("max_records_count must survive decoding as a modelled key")
	}
}

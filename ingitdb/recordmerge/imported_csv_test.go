package recordmerge

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestImportedCSVMergeUsesTransportID(t *testing.T) {
	col := &ingitdb.CollectionDef{
		RecordFile:   &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatCSV, CSVCellEncoding: "json-v1"},
		ColumnsOrder: []string{"$ID", "part_a", "part_b"},
		PrimaryKey:   []string{"part_a", "part_b"},
		SourceSchema: &ingitdb.SourceSchemaDef{KeyMode: "source-primary-key"},
	}
	keys := csvKeyColumns(col)
	if len(keys) != 1 || keys[0] != "$ID" {
		t.Fatalf("merge key=%v, want transport ID", keys)
	}
	// Use the core encoder so each CSV cell is a JSON literal before RFC4180 quoting.
	encoded, err := ingitdb.EncodeRecordContentForCollection([]map[string]any{{"$ID": "pk-transport", "part_a": "A", "part_b": int64(2)}}, col)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parseCSVRecords(encoded, col, keys)
	if err != nil || len(rows) != 1 || rows[0].Key != "pk-transport" {
		t.Fatalf("imported CSV merge rows=%#v err=%v", rows, err)
	}
	bad, err := ingitdb.EncodeRecordContentForCollection([]map[string]any{{"$ID": nil, "part_a": "A", "part_b": int64(2)}}, col)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseCSVRecords(bad, col, keys); err == nil || !strings.Contains(err.Error(), "$ID") {
		t.Fatalf("missing imported merge ID accepted: %v", err)
	}
}

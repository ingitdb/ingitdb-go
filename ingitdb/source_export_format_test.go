package ingitdb

import (
	"strings"
	"testing"
)

func TestImportedCSVJSONCellsKeepTransportIDAndTypes(t *testing.T) {
	col := &CollectionDef{
		ID: "imported",
		RecordFile: &RecordFileDef{Name: "records.csv", Format: RecordFormatCSV,
			RecordType: ListOfRecords, CSVCellEncoding: "json-v1"},
		Columns: map[string]*ColumnDef{
			"part_a": {Type: ColumnTypeString}, "part_b": {Type: ColumnTypeInt},
			"amount": {Type: ColumnTypeString}, "note": {Type: ColumnTypeString},
			"wide": {Type: ColumnTypeInt},
		},
		ColumnsOrder: []string{"$ID", "part_a", "part_b", "amount", "note", "wide"},
		PrimaryKey:   []string{"part_a", "part_b"},
		SourceSchema: &SourceSchemaDef{KeyMode: "source-primary-key"},
	}
	if err := col.Validate(); err != nil {
		t.Fatalf("imported CSV definition: %v", err)
	}
	rows := []map[string]any{
		{"$ID": "pk-composite", "part_a": "A", "part_b": int64(2), "amount": "1.20", "note": "", "wide": int64(9007199254740993)},
		{"$ID": "pk-null", "part_a": "B", "part_b": int64(3), "amount": nil, "note": "line 1,\nline 2", "wide": int64(1)},
	}
	encoded, err := EncodeRecordContentForCollection(rows, col)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRecordContentForCollection(encoded, col)
	if err != nil {
		t.Fatal(err)
	}
	got := parsed[recordsKey].([]map[string]any)
	if len(got) != 2 || got[0]["wide"] != int64(9007199254740993) || got[0]["note"] != "" || got[1]["amount"] != nil || got[1]["note"] != "line 1,\nline 2" {
		t.Fatalf("CSV typed cells changed: %#v", got)
	}
	if id, ok := ResolveListRecordKey(got[0], col); !ok || id != "pk-composite" {
		t.Fatalf("transport ID was replaced by source PK: %q %v", id, ok)
	}
}

func TestImportedJSONLAndINGRKeepWideInteger(t *testing.T) {
	rows := []map[string]any{{"$ID": "pk-composite", "id": int64(9007199254740993), "amount": "1.20"}}
	encoded, err := EncodeListOfRecordsContent(rows, RecordFormatJSONL, []string{"$ID", "id", "amount"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseListOfRecordsContent(encoded, RecordFormatJSONL)
	if err != nil || len(parsed) != 1 || parsed[0]["id"] != int64(9007199254740993) {
		t.Fatalf("JSONL wide integer: %#v, %v", parsed, err)
	}
	col := &CollectionDef{PrimaryKey: []string{"id"}, SourceSchema: &SourceSchemaDef{KeyMode: "source-primary-key"}}
	if id, ok := ResolveListRecordKey(parsed[0], col); !ok || id != "pk-composite" {
		t.Fatalf("JSONL transport ID: %q %v", id, ok)
	}
	mapRecords := map[string]map[string]any{"pk-composite": {"id": int64(9007199254740993), "amount": "1.20"}}
	ingrContent, err := EncodeMapOfRecordsContent(mapRecords, RecordFormatINGR, "imported", []string{"id", "amount"})
	if err != nil {
		t.Fatal(err)
	}
	ingrParsed, err := ParseMapOfRecordsContent(ingrContent, RecordFormatINGR)
	if err != nil || ingrParsed["pk-composite"]["id"] != int64(9007199254740993) || ingrParsed["pk-composite"]["amount"] != "1.20" {
		t.Fatalf("INGR wide integer/decimal: %#v, %v", ingrParsed, err)
	}
}

func TestImportedCSVEncodingIsOptInAndMalformedCellsFail(t *testing.T) {
	col := &CollectionDef{RecordFile: &RecordFileDef{Name: "rows.csv", Format: RecordFormatCSV, RecordType: ListOfRecords,
		CSVCellEncoding: "json-v1"}, ColumnsOrder: []string{"$ID", "value"}}
	for _, bad := range []string{"$ID,value\n\"pk\",not-json\n", "$ID,value\n\"pk\",1 2\n"} {
		if _, err := parseCSVForCollection([]byte(bad), col); err == nil {
			t.Fatalf("malformed json-v1 cell accepted: %q", bad)
		}
	}
	trailingCol := &CollectionDef{RecordFile: col.RecordFile, ColumnsOrder: []string{"value"}}
	if _, err := parseCSVForCollection([]byte("value\n1 2\n"), trailingCol); err == nil || !strings.Contains(err.Error(), "trailing json-v1") {
		t.Fatalf("trailing json-v1 value was accepted: %v", err)
	}
	if _, err := encodeCSVForCollection([]map[string]any{{"$ID": "pk", "value": make(chan int)}}, col); err == nil || !strings.Contains(err.Error(), "encode json-v1 cell") {
		t.Fatalf("unencodable json-v1 cell was accepted: %v", err)
	}
	legacy := &CollectionDef{RecordFile: &RecordFileDef{Name: "rows.csv", Format: RecordFormatCSV, RecordType: ListOfRecords},
		ColumnsOrder: []string{"value"}}
	got, err := parseCSVForCollection([]byte("value\n\"\"\n"), legacy)
	if err != nil || got[recordsKey].([]map[string]any)[0]["value"] != "" {
		t.Fatalf("legacy CSV string/empty behavior changed: %#v %v", got, err)
	}
	for _, recordFile := range []RecordFileDef{
		{Name: "rows.csv", Format: RecordFormatCSV, RecordType: ListOfRecords, CSVCellEncoding: "unknown"},
		{Name: "rows.json", Format: RecordFormatJSON, RecordType: ListOfRecords, CSVCellEncoding: "json-v1"},
	} {
		if err := recordFile.Validate(); err == nil || !strings.Contains(err.Error(), "csv_cell_encoding") {
			t.Fatalf("invalid CSV encoding was accepted: %+v, %v", recordFile, err)
		}
	}
}

func TestImportedJSONListsRejectTrailingValues(t *testing.T) {
	for _, test := range []struct {
		format RecordFormat
		data   string
	}{
		{RecordFormatJSON, "[] []"},
		{RecordFormatJSONL, "{} {}\n"},
	} {
		if _, err := ParseListOfRecordsContent([]byte(test.data), test.format); err == nil || !strings.Contains(err.Error(), "trailing content") {
			t.Fatalf("%s accepted trailing JSON value: %v", test.format, err)
		}
	}
}

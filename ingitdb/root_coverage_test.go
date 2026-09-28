package ingitdb

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/ingr-io/ingr-go/ingr"
)

// 1. Column formula eval
func TestColumnFormulaEval_PrintAndBadMap(t *testing.T) {
	// Call print() inside formula to hit Print callback
	v, err := EvaluateFormula("print('debug') or 42", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != int64(42) {
		t.Errorf("got %v, want 42", v)
	}

	// Bad value in nested map: func() cannot be converted to Starlark
	_, err = EvaluateFormula("42", map[string]any{"nested": map[string]any{"bad": func() {}}})
	if err == nil {
		t.Fatal("expected error for unsupported type in record map")
	}
}

// 2. Column type
func TestColumnType_EdgeCases(t *testing.T) {
	if err := ValidateColumnType("map[unclosed"); err == nil {
		t.Error("expected error for map[unclosed")
	}
	if _, ok := ListElementType("string"); ok {
		t.Error("expected false for non-list type")
	}
}

// 3. CSV tests
type mockFailCSVWriter struct {
	failHeader         bool
	failRowAfterHeader bool
	callCount          int
	failError          bool
}

func (m *mockFailCSVWriter) Write(record []string) error {
	m.callCount++
	if m.failHeader && m.callCount == 1 {
		return errors.New("fail header")
	}
	if m.failRowAfterHeader && m.callCount > 1 {
		return errors.New("fail row")
	}
	return nil
}

func (m *mockFailCSVWriter) Flush() {}

func (m *mockFailCSVWriter) Error() error {
	if m.failError {
		return errors.New("flush error")
	}
	return nil
}

func TestCSV_RemainingBranches(t *testing.T) {
	// parseCSVForCollection empty ColumnsOrder
	colNoCols := &CollectionDef{}
	if _, err := parseCSVForCollection([]byte("a,b\n1,2"), colNoCols); err == nil {
		t.Error("expected error on empty ColumnsOrder")
	}

	colDef := &CollectionDef{
		ColumnsOrder: []string{"id", "name"},
	}

	// Empty CSV (EOF)
	if _, err := parseCSVForCollection([]byte(""), colDef); err == nil {
		t.Error("expected error on empty CSV")
	}

	// CSV header syntax error
	if _, err := parseCSVForCollection([]byte("id,\"name\n"), colDef); err == nil {
		t.Error("expected error on invalid CSV header quote")
	}

	// Row syntax error (bare quote)
	if _, err := parseCSVForCollection([]byte("id,name\n1,\"bad quote\n2,3"), colDef); err == nil {
		t.Error("expected error on bad CSV row")
	}

	// Header has extra columns
	if err := validateCSVHeader([]string{"id", "name", "extra"}, []string{"id", "name"}); err == nil {
		t.Error("expected error on extra columns in header")
	}

	// encodeCSVForCollection: empty ColumnsOrder
	if _, err := encodeCSVForCollection([]map[string]any{{"a": "b"}}, colNoCols); err == nil {
		t.Error("expected error on empty ColumnsOrder for encode")
	}

	// encodeCSVForCollection: missing col, nil value, and primitive value
	rows := []map[string]any{
		{"id": "1", "name": nil}, // name is nil
		{"id": "2"},               // name missing
		{"id": 3, "name": true},   // primitive int and bool
	}
	out, err := encodeCSVForCollection(rows, colDef)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty CSV output")
	}

	// coerceToRowList with []any
	anyValid := []any{map[string]any{"id": "1"}}
	if _, err := coerceToRowList(anyValid); err != nil {
		t.Errorf("coerceToRowList([]any): unexpected error %v", err)
	}
	anyInvalid := []any{"not-a-map"}
	if _, err := coerceToRowList(anyInvalid); err == nil {
		t.Error("expected error on []any with non-map item")
	}
	if _, err := coerceToRowList(12345); err == nil {
		t.Error("expected error on int value")
	}

	// mock failing CSV writers
	origCSVWriter := newCSVWriter
	defer func() { newCSVWriter = origCSVWriter }()

	newCSVWriter = func(w io.Writer) csvWriter {
		return &mockFailCSVWriter{failHeader: true}
	}
	if _, err := encodeCSVForCollection(rows, colDef); err == nil {
		t.Error("expected error on failHeader")
	}

	newCSVWriter = func(w io.Writer) csvWriter {
		return &mockFailCSVWriter{failRowAfterHeader: true}
	}
	if _, err := encodeCSVForCollection(rows, colDef); err == nil {
		t.Error("expected error on failRow")
	}

	newCSVWriter = func(w io.Writer) csvWriter {
		return &mockFailCSVWriter{failError: true}
	}
	if _, err := encodeCSVForCollection(rows, colDef); err == nil {
		t.Error("expected error on failError")
	}
}

// 4. List records
func TestListRecords_RemainingBranches(t *testing.T) {
	if _, err := ParseListOfRecordsContent([]byte("[]"), "unknown_format"); err == nil {
		t.Error("expected error for unknown format")
	}
	if rows, err := parseYAMLList([]byte("   \n")); err != nil || rows != nil {
		t.Errorf("expected nil rows on empty YAML, got %v, %v", rows, err)
	}
	if _, err := parseYAMLList([]byte("invalid: [yaml")); err == nil {
		t.Error("expected error on invalid YAML list")
	}
	if rows, err := parseJSONList([]byte("   \n")); err != nil || rows != nil {
		t.Errorf("expected nil rows on empty JSON, got %v, %v", rows, err)
	}
	if _, err := parseJSONList([]byte("invalid json")); err == nil {
		t.Error("expected error on invalid JSON list")
	}
	if _, err := parseJSONLList([]byte("invalid json line\n")); err == nil {
		t.Error("expected error on invalid JSONL line")
	}
}

// 5. Parse and encode
type mockFailRecordsWriter struct {
	failHeader  bool
	failRecords bool
	failClose   bool
}

func (m *mockFailRecordsWriter) WriteHeader(name string, cols []ingr.ColDef) (int, error) {
	if m.failHeader {
		return 0, errors.New("fail header")
	}
	return 0, nil
}

func (m *mockFailRecordsWriter) WriteRecords(offset int, records ...ingr.Record) (int, error) {
	if m.failRecords {
		return 0, errors.New("fail records")
	}
	return 0, nil
}

func (m *mockFailRecordsWriter) Close() error {
	if m.failClose {
		return errors.New("fail close")
	}
	return nil
}

func makeINGR(recordsetName string, colNames []string, rows []map[string]any) []byte {
	var buf bytes.Buffer
	w := ingr.NewRecordsWriter(&buf)
	cols := make([]ingr.ColDef, len(colNames))
	for i, c := range colNames {
		cols[i] = ingr.ColDef{Name: c}
	}
	_, _ = w.WriteHeader(recordsetName, cols)
	records := make([]ingr.Record, len(rows))
	for i, r := range rows {
		records[i] = ingr.NewMapRecordEntry(r["$ID"], r)
	}
	_, _ = w.WriteRecords(0, records...)
	_ = w.Close()
	return buf.Bytes()
}

func TestParse_RemainingBranches(t *testing.T) {
	// ParseRecordContentForCollection colDef checks
	if _, err := ParseRecordContentForCollection([]byte(""), nil); err == nil {
		t.Error("expected error for nil colDef")
	}
	if _, err := ParseRecordContentForCollection([]byte(""), &CollectionDef{}); err == nil {
		t.Error("expected error for nil RecordFile")
	}

	// Default format passthrough
	colJSON := &CollectionDef{RecordFile: &RecordFileDef{Format: RecordFormatJSON}}
	data, err := ParseRecordContentForCollection([]byte(`{"a": 1}`), colJSON)
	if err != nil || data["a"] != float64(1) {
		t.Errorf("unexpected JSON parse result: %v, %v", data, err)
	}

	// Markdown parse error
	colMD := &CollectionDef{
		RecordFile: &RecordFileDef{Format: RecordFormatMarkdown},
		Columns:    map[string]*ColumnDef{"name": {Type: "string"}},
	}
	if _, err := ParseRecordContentForCollection([]byte("---\ninvalid: [yaml\n---\nbody"), colMD); err == nil {
		t.Error("expected error on invalid frontmatter")
	}

	// Frontmatter with $id and undeclared column
	mdContent := []byte("---\n$id: my-id\nname: Alice\nundeclared: foo\n---\nHello body\n")
	res, err := ParseRecordContentForCollection(mdContent, colMD)
	if err != nil {
		t.Fatalf("unexpected md parse error: %v", err)
	}
	if res["$id"] != "my-id" || res["name"] != "Alice" || res["$content"] != "Hello body\n" {
		t.Errorf("unexpected md result: %+v", res)
	}
	if _, ok := res["undeclared"]; ok {
		t.Error("expected undeclared column to be omitted")
	}

	// ParseMapOfRecordsContent with INGR
	ingrMapData := map[string]map[string]any{
		"rec1": {"name": "first", "extra": "foo"},
		"rec2": {"name": "second"},
	}
	ingrBytes, err := EncodeMapOfRecordsContent(ingrMapData, RecordFormatINGR, "mycol", []string{"$ID", "name"})
	if err != nil {
		t.Fatalf("EncodeMapOfRecordsContent INGR error: %v", err)
	}
	parsedMap, err := ParseMapOfRecordsContent(ingrBytes, RecordFormatINGR)
	if err != nil {
		t.Fatalf("ParseMapOfRecordsContent INGR error: %v", err)
	}
	if len(parsedMap) != 2 || parsedMap["rec1"]["name"] != "first" {
		t.Errorf("unexpected parsedMap: %+v", parsedMap)
	}

	// parseINGRAsMap error cases
	if _, err := parseINGRAsMap([]byte("not ingr format")); err == nil {
		t.Error("expected error on bad INGR")
	}
	// Missing $ID
	missingIDIngr := makeINGR("test", []string{"col"}, []map[string]any{{"col": "val"}})
	if _, err := parseINGRAsMap(missingIDIngr); err == nil {
		t.Error("expected error on missing $ID")
	}
	// Non-string $ID
	nonStringIDIngr := makeINGR("test", []string{"$ID"}, []map[string]any{{"$ID": 12345}})
	if _, err := parseINGRAsMap(nonStringIDIngr); err == nil {
		t.Error("expected error on non-string $ID")
	}
	// Duplicate $ID
	dupIDIngr := makeINGR("test", []string{"$ID"}, []map[string]any{{"$ID": "dup"}, {"$ID": "dup"}})
	if _, err := parseINGRAsMap(dupIDIngr); err == nil {
		t.Error("expected error on duplicate $ID")
	}

	// EncodeMapOfRecordsContent with non-INGR formats
	for _, fmtName := range []RecordFormat{RecordFormatYAML, RecordFormatJSON, RecordFormatTOML} {
		out, err := EncodeMapOfRecordsContent(ingrMapData, fmtName, "mycol", nil)
		if err != nil || len(out) == 0 {
			t.Errorf("EncodeMapOfRecordsContent %s error: %v", fmtName, err)
		}
	}
	// Unsupported format
	if _, err := marshalForFormat(map[string]any{"a": 1}, "unsupported"); err == nil {
		t.Error("expected error on unsupported format")
	}

	// YAML marshal error
	origYAMLMarshal := yamlMarshal
	yamlMarshal = func(any) ([]byte, error) { return nil, errors.New("fail yaml") }
	if _, err := marshalForFormat(map[string]any{"a": 1}, RecordFormatYAML); err == nil {
		t.Error("expected error on yamlMarshal fail")
	}
	yamlMarshal = origYAMLMarshal

	// JSON marshal error
	if _, err := marshalForFormat(map[string]any{"bad": make(chan int)}, RecordFormatJSON); err == nil {
		t.Error("expected error on json marshal fail")
	}

	// TOML marshal error
	origTOMLMarshal := tomlMarshal
	tomlMarshal = func(any) ([]byte, error) { return nil, errors.New("fail toml") }
	if _, err := marshalForFormat(map[string]any{"a": 1}, RecordFormatTOML); err == nil {
		t.Error("expected error on tomlMarshal fail")
	}
	tomlMarshal = origTOMLMarshal

	// EncodeRecordContentForCollection checks
	if _, err := EncodeRecordContentForCollection(map[string]any{}, nil); err == nil {
		t.Error("expected error on nil colDef")
	}
	if _, err := EncodeRecordContentForCollection(map[string]any{}, &CollectionDef{}); err == nil {
		t.Error("expected error on nil RecordFile")
	}
	colJSONRec := &CollectionDef{RecordFile: &RecordFileDef{Format: RecordFormatJSON}}
	jsonOut, err := EncodeRecordContentForCollection(map[string]any{"k": "v"}, colJSONRec)
	if err != nil || len(jsonOut) == 0 {
		t.Errorf("unexpected JSON encode error: %v", err)
	}

	// mock failing newRecordsWriter
	origRecordsWriter := newRecordsWriter
	defer func() { newRecordsWriter = origRecordsWriter }()

	newRecordsWriter = func(w io.Writer) ingr.RecordsWriter {
		return &mockFailRecordsWriter{failHeader: true}
	}
	if _, err := encodeINGRFromMap(ingrMapData, "mycol", nil); err == nil {
		t.Error("expected error on failHeader")
	}

	newRecordsWriter = func(w io.Writer) ingr.RecordsWriter {
		return &mockFailRecordsWriter{failRecords: true}
	}
	if _, err := encodeINGRFromMap(ingrMapData, "mycol", nil); err == nil {
		t.Error("expected error on failRecords")
	}

	newRecordsWriter = func(w io.Writer) ingr.RecordsWriter {
		return &mockFailRecordsWriter{failClose: true}
	}
	if _, err := encodeINGRFromMap(ingrMapData, "mycol", nil); err == nil {
		t.Error("expected error on failClose")
	}

	// Call default newRecordsWriter to cover seams_parse.go
	origRecordsWriter(&bytes.Buffer{})
}

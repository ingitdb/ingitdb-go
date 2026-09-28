package datavalidator

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestValidator_RemainingCoverage(t *testing.T) {
	dir := t.TempDir()

	// 1. validateCollectionRecords: ListOfRecords & unsupported
	listCol := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			RecordType: ingitdb.ListOfRecords,
			Format:     ingitdb.RecordFormatJSON,
		},
	}
	validateCollectionRecords("listcol", listCol)

	badCol := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			RecordType: "unsupported",
			Format:     ingitdb.RecordFormatJSON,
		},
	}
	_, _, errs := validateCollectionRecords("bad", badCol)
	if len(errs) != 1 {
		t.Errorf("expected 1 error for unsupported record type, got %d", len(errs))
	}

	// 2. validateSingleRecordFiles: glob error
	badGlobCol := &ingitdb.CollectionDef{
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "[a-",
			RecordType: ingitdb.SingleRecord,
		},
	}
	_, _, globErrs := validateSingleRecordFiles("bad_glob", badGlobCol)
	if len(globErrs) != 1 {
		t.Errorf("expected 1 error for bad glob, got %d", len(globErrs))
	}

	// 3. validateSingleRecordFile branches
	singleDir := filepath.Join(dir, "single_branches")
	_ = os.MkdirAll(singleDir, 0o755)
	subDir := filepath.Join(singleDir, "subdir")
	_ = os.MkdirAll(subDir, 0o755)
	unreadableSingle := filepath.Join(singleDir, "unreadable.yaml")
	_ = os.WriteFile(unreadableSingle, []byte("a: 1"), 0o000)
	defer func() { _ = os.Chmod(unreadableSingle, 0o644) }()

	singleCol := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	// skipRecordPath
	p, tot, _ := validateSingleRecordFile("c", singleCol, filepath.Join(singleDir, ".hidden.yaml"))
	if p != 0 || tot != 0 {
		t.Error("expected 0, 0 for hidden file")
	}
	// os.Stat error
	_, statTot, statErrs := validateSingleRecordFile("c", singleCol, filepath.Join(singleDir, "missing.yaml"))
	if statTot != 1 || len(statErrs) != 1 {
		t.Error("expected 1 error for missing file stat")
	}
	// info.IsDir()
	p, tot, _ = validateSingleRecordFile("c", singleCol, subDir)
	if p != 0 || tot != 0 {
		t.Error("expected 0, 0 for directory")
	}
	// readErr
	_, readTot, readErrs := validateSingleRecordFile("c", singleCol, unreadableSingle)
	if readTot != 1 || len(readErrs) != 1 {
		t.Error("expected 1 error for unreadable file")
	}

	// 4. recordKeyFromCollectionFilePath: filepath.Rel error
	relCol := &ingitdb.CollectionDef{
		DirPath: "relative/path",
		RecordFile: &ingitdb.RecordFileDef{
			Name: "{key}.yaml",
		},
	}
	key := recordKeyFromCollectionFilePath(relCol, "/abs/path/file.yaml")
	if key != "file" {
		t.Errorf("got %q, want 'file'", key)
	}

	// 5. validateMapOfRecordsFile branches
	mapColMissing := &ingitdb.CollectionDef{
		DirPath: filepath.Join(dir, "missing_map"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "missing.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	p, tot, _ = validateMapOfRecordsFile("c", mapColMissing)
	if p != 0 || tot != 0 {
		t.Error("expected 0, 0 for missing map file")
	}

	mapDir := filepath.Join(dir, "map_branches")
	_ = os.MkdirAll(mapDir, 0o755)
	unreadableMap := filepath.Join(mapDir, "unreadable.yaml")
	_ = os.WriteFile(unreadableMap, []byte("k: v"), 0o000)
	defer func() { _ = os.Chmod(unreadableMap, 0o644) }()
	mapColUnreadable := &ingitdb.CollectionDef{
		DirPath: mapDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "unreadable.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	_, mapReadTot, mapReadErrs := validateMapOfRecordsFile("c", mapColUnreadable)
	if mapReadTot != 1 || len(mapReadErrs) != 1 {
		t.Error("expected 1 error for unreadable map file")
	}

	badYAMLMap := filepath.Join(mapDir, "bad.yaml")
	_ = os.WriteFile(badYAMLMap, []byte("bad: [yaml"), 0o644)
	mapColBadYAML := &ingitdb.CollectionDef{
		DirPath: mapDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "bad.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	_, badYAMLTot, badYAMLErrs := validateMapOfRecordsFile("c", mapColBadYAML)
	if badYAMLTot != 1 || len(badYAMLErrs) != 1 {
		t.Error("expected 1 error for bad YAML map file")
	}

	// 6. validateListOfRecordsFile branches
	listColMissing := &ingitdb.CollectionDef{
		DirPath: filepath.Join(dir, "missing_list"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "missing.json",
			Format:     ingitdb.RecordFormatJSON,
			RecordType: ingitdb.ListOfRecords,
		},
	}
	p, tot, _ = validateListOfRecordsFile("c", listColMissing)
	if p != 0 || tot != 0 {
		t.Error("expected 0, 0 for missing list file")
	}

	unreadableList := filepath.Join(mapDir, "unreadable.json")
	_ = os.WriteFile(unreadableList, []byte("[]"), 0o000)
	defer func() { _ = os.Chmod(unreadableList, 0o644) }()
	listColUnreadable := &ingitdb.CollectionDef{
		DirPath: mapDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "unreadable.json",
			Format:     ingitdb.RecordFormatJSON,
			RecordType: ingitdb.ListOfRecords,
		},
	}
	_, listReadTot, listReadErrs := validateListOfRecordsFile("c", listColUnreadable)
	if listReadTot != 1 || len(listReadErrs) != 1 {
		t.Error("expected 1 error for unreadable list file")
	}

	badJSONList := filepath.Join(mapDir, "bad.json")
	_ = os.WriteFile(badJSONList, []byte("bad json"), 0o644)
	listColBadJSON := &ingitdb.CollectionDef{
		DirPath: mapDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "bad.json",
			Format:     ingitdb.RecordFormatJSON,
			RecordType: ingitdb.ListOfRecords,
		},
	}
	_, badJSONTot, badJSONErrs := validateListOfRecordsFile("c", listColBadJSON)
	if badJSONTot != 1 || len(badJSONErrs) != 1 {
		t.Error("expected 1 error for bad JSON list file")
	}

	// List with unresolvable key and validation error
	mixedList := filepath.Join(mapDir, "mixed.json")
	_ = os.WriteFile(mixedList, []byte(`[{"no_key": 1}, {"id": "1", "qty": "not_an_int"}, {"id": "2", "qty": 10}]`), 0o644)
	listColMixed := &ingitdb.CollectionDef{
		DirPath: mapDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "mixed.json",
			Format:     ingitdb.RecordFormatJSON,
			RecordType: ingitdb.ListOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"id":  {Type: ingitdb.ColumnTypeString},
			"qty": {Type: ingitdb.ColumnTypeInt},
		},
	}
	pPassed, mixedTot, mixedErrs := validateListOfRecordsFile("c", listColMixed)
	if mixedTot != 3 || len(mixedErrs) < 2 || pPassed != 1 {
		t.Errorf("expected 1 passed, 3 tot, at least 2 errs, got p=%d, tot=%d, len=%d", pPassed, mixedTot, len(mixedErrs))
	}

	// 7. parseListRows: CSV errors and INGR format
	csvColBad := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Format: ingitdb.RecordFormatCSV,
		},
		ColumnsOrder: []string{"a", "b"},
	}
	if _, err := parseListRows([]byte("invalid csv"), csvColBad); err == nil {
		t.Error("expected error for invalid CSV in parseListRows")
	}

	// Valid CSV in parseListRows
	validCSVRows, err := parseListRows([]byte("a,b\n1,2\n"), csvColBad)
	if err != nil || len(validCSVRows) != 1 {
		t.Errorf("unexpected parseListRows valid CSV: %v, %v", validCSVRows, err)
	}

	// INGR format in parseListRows
	ingrCol := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Format: ingitdb.RecordFormatINGR,
		},
	}
	// INGR parse error
	if _, err := parseListRows([]byte("bad ingr"), ingrCol); err == nil {
		t.Error("expected error for bad INGR in parseListRows")
	}
	// INGR valid records
	ingrData := map[string]map[string]any{"rec1": {"name": "test"}}
	ingrBytes, _ := ingitdb.EncodeMapOfRecordsContent(ingrData, ingitdb.RecordFormatINGR, "mycol", nil)
	rows, err := parseListRows(ingrBytes, ingrCol)
	if err != nil || len(rows) != 1 || rows[0]["$ID"] != "rec1" {
		t.Fatalf("unexpected INGR parseListRows result: %v, %v", rows, err)
	}

	// 8. columnIsRequired with formula error
	colReqWhenErr := &ingitdb.ColumnDef{
		RequiredWhen: "1 / 0",
	}
	if _, err := columnIsRequired(colReqWhenErr, &ingitdb.CollectionDef{}, nil); err == nil {
		t.Error("expected error for division by zero in required_when")
	}

	// 9. Type checks & bounds
	// numericValue
	if _, ok := numericValue(int32(1)); !ok {
		t.Error("int32 should be numeric")
	}
	if _, ok := numericValue(int64(1)); !ok {
		t.Error("int64 should be numeric")
	}
	if _, ok := numericValue(float32(1)); !ok {
		t.Error("float32 should be numeric")
	}
	if _, ok := numericValue(float64(1)); !ok {
		t.Error("float64 should be numeric")
	}
	if _, ok := numericValue("str"); ok {
		t.Error("string should not be numeric")
	}

	// valueMatchesColumnType
	if !valueMatchesColumnType(float32(1.5), ingitdb.ColumnTypeFloat) {
		t.Error("float32 should match float")
	}
	if !valueMatchesColumnType(true, ingitdb.ColumnTypeBool) || valueMatchesColumnType("true", ingitdb.ColumnTypeBool) {
		t.Error("bool type mismatch")
	}
	if !valueMatchesColumnType(time.Now(), ingitdb.ColumnTypeDate) || !valueMatchesColumnType("2026-01-01", ingitdb.ColumnTypeDate) || valueMatchesColumnType(123, ingitdb.ColumnTypeDate) {
		t.Error("temporal type mismatch")
	}
	if !valueMatchesColumnType(map[string]string{"en": "hello"}, ingitdb.ColumnTypeL10N) {
		t.Error("l10n mismatch")
	}
	if valueMatchesColumnType("val", ingitdb.ColumnType("unrecognized")) {
		t.Error("unrecognized type should return false")
	}

	// isIntegerValue
	if !isIntegerValue(uint(5)) || !isIntegerValue(uint8(5)) || !isIntegerValue(uint16(5)) || !isIntegerValue(uint32(5)) || !isIntegerValue(uint64(5)) {
		t.Error("uint types should be integer")
	}
	if !isIntegerValue(int8(5)) || !isIntegerValue(int16(5)) {
		t.Error("int8/16 should be integer")
	}
	if !isIntegerValue(float32(5.0)) || isIntegerValue(float32(5.5)) {
		t.Error("float32 int checks failed")
	}
	if !isIntegerValue(float64(5.0)) || isIntegerValue(float64(5.5)) {
		t.Error("float64 int checks failed")
	}
	if isIntegerValue("not an int") {
		t.Error("string is not an int")
	}

	// isNumberValue
	if !isNumberValue(uint(1)) || !isNumberValue(int8(1)) || isNumberValue("str") {
		t.Error("isNumberValue failed")
	}

	// isTemporalValue
	if !isTemporalValue(time.Now()) || !isTemporalValue("2026") || isTemporalValue(123) {
		t.Error("isTemporalValue failed")
	}

	// isStringMap
	if !isStringMap(map[string]string{"a": "b"}) {
		t.Error("map[string]string should be string map")
	}
	if !isStringMap(map[string]any{"a": "hello"}) {
		t.Error("map[string]any with string values should be string map")
	}
	if isStringMap(map[string]any{"a": 123}) {
		t.Error("map[string]any with non-string should not be string map")
	}
	if isStringMap(123) {
		t.Error("int is not string map")
	}

	// isMapValue
	if !isMapValue(map[string]string{"a": "b"}) || isMapValue(123) {
		t.Error("isMapValue failed")
	}

	// checkValueRange with non-numeric value
	zero := float64(0)
	if err := checkValueRange("f", "non-num", &zero, &zero); err != nil {
		t.Error("checkValueRange should return nil for non-numeric value")
	}

	// checkLength with non-length value (int)
	zeroInt := 0
	if err := checkLength("f", 123, &zeroInt, nil, nil); err != nil {
		t.Error("checkLength should return nil for non-length value")
	}
	// valueLength with map[string]any
	if l, ok := valueLength(map[string]any{"a": 1, "b": 2}); !ok || l != 2 {
		t.Errorf("valueLength map failed: %d, %v", l, ok)
	}
}

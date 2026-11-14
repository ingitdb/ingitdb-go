package datavalidator

// specscore: feature/column-validation
// specscore: feature/record-count-constraints
// specscore: feature/subcollection-record-validation
// specscore: feature/explicit-record-base-directory

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

const listRecordsKey = "$records"

// NewValidator creates a data validator that parses records and checks basic schema constraints.
func NewValidator() DataValidator {
	return &simpleValidator{}
}

type simpleValidator struct{}

// Validate performs basic validation of records against their collection schemas.
// Returns a ValidationResult with any errors found.
func (sv *simpleValidator) Validate(_ context.Context, dbPath string, def *ingitdb.Definition) (*ingitdb.ValidationResult, error) {
	result := &ingitdb.ValidationResult{}

	for collectionKey, colDef := range def.Collections {
		passed, total, errors := validateCollectionRecords(collectionKey, colDef)
		for _, validationErr := range errors {
			result.Append(validationErr)
		}
		for _, validationErr := range validateStorageClassFiles(collectionKey, colDef) {
			result.Append(validationErr)
		}
		// Record-count bounds are a collection-level invariant checked against
		// the record count the schema pass just produced. Only root collections
		// are walked here (this loop, like the schema pass, iterates
		// def.Collections, never subcollections) — see the Feature's
		// root-vs-subcollection note.
		for _, validationErr := range checkRecordCountConstraints(collectionKey, colDef, total) {
			result.Append(validationErr)
		}
		result.SetRecordCounts(collectionKey, passed, total)
		result.SetRecordCount(collectionKey, total)
	}

	// Subcollection records are not reached by the root loop above (it iterates
	// def.Collections only). This additive pass walks every collection's
	// SubCollections recursively, resolving each subcollection's on-disk data
	// per parent record, and validates their records with the same per-record
	// checks and record-count enforcement. See
	// spec/features/subcollection-record-validation.
	validateSubCollections(dbPath, def, result)
	for _, finding := range mislaidSubCollectionFindings(dbPath, def) {
		result.Append(finding)
	}

	// Referential integrity is a whole-definition concern — a value must exist
	// as a key in the resolved target collection — so it runs after the
	// per-collection schema pass, once every collection's keys are known. It
	// covers both root and subcollection records (shared walk).
	for _, validationErr := range validateForeignKeyReferences(dbPath, def) {
		result.Append(validationErr)
	}
	for _, validationErr := range validateSourceForeignKeys(def) {
		result.Append(validationErr)
	}

	return result, nil
}

func validateCollectionRecords(collectionKey string, colDef *ingitdb.CollectionDef) (int, int, []ingitdb.ValidationError) {
	total, err := countRecords(colDef)
	if err != nil {
		total = 0
	}
	if shouldSkipRecordParsing(colDef) {
		return total, total, nil
	}
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		return validateSingleRecordFiles(collectionKey, colDef)
	case ingitdb.MapOfRecords:
		return validateMapOfRecordsFile(collectionKey, colDef)
	case ingitdb.ListOfRecords:
		return validateListOfRecordsFile(collectionKey, colDef)
	default:
		validationErr := newValidationError(collectionKey, "", "", "", "unsupported record type", nil)
		return 0, 0, []ingitdb.ValidationError{validationErr}
	}
}

func shouldSkipRecordParsing(colDef *ingitdb.CollectionDef) bool {
	if colDef == nil || colDef.RecordFile == nil {
		return true
	}
	if colDef.RecordFile.Format == "" || colDef.RecordFile.RecordType == "" {
		return true
	}
	return false
}

func validateSingleRecordFiles(collectionKey string, colDef *ingitdb.CollectionDef) (int, int, []ingitdb.ValidationError) {
	pattern := singleRecordGlobPattern(colDef)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		validationErr := newValidationError(collectionKey, pattern, "", "", "failed to glob record files", err)
		return 0, 0, []ingitdb.ValidationError{validationErr}
	}
	passed := 0
	total := 0
	var errors []ingitdb.ValidationError
	for _, filePath := range matches {
		filePassed, fileTotal, fileErrors := validateSingleRecordFile(collectionKey, colDef, filePath)
		passed += filePassed
		total += fileTotal
		errors = append(errors, fileErrors...)
	}
	return passed, total, append(errors, caseCollisionFindings(collectionKey, colDef, matches)...)
}

// caseCollisionFindings reports, for a collection that declares a key
// encoding, records whose key names differ only in letter case, whether the
// name is a file ({key}.yaml) or a directory ({key}/index.yaml): on a
// case-insensitive file system (Windows, macOS by default) they are one
// entry, so a checkout there loses all but one record.
func caseCollisionFindings(collectionKey string, colDef *ingitdb.CollectionDef, matches []string) []ingitdb.ValidationError {
	if !colDef.RecordFile.EncodesKeys() {
		return nil
	}
	baseDir := recordsBaseDir(colDef)
	var names []string
	pathOf := map[string]string{}
	for _, filePath := range matches {
		if skipRecordPath(filePath, colDef.RecordFile) {
			continue
		}
		name := recordNameFromCollectionFilePath(colDef, filePath)
		names = append(names, name)
		pathOf[name] = filePath
	}
	var findings []ingitdb.ValidationError
	for _, group := range ingitdb.CaseOnlyDifferences(names) {
		rels := make([]string, 0, len(group))
		for _, name := range group {
			rel, _ := filepath.Rel(baseDir, pathOf[name])
			rels = append(rels, filepath.ToSlash(rel))
		}
		message := fmt.Sprintf("record files %s in %s differ only in letter case: on a case-insensitive file system (Windows, macOS by default) they are one file and all but one record is lost; give the records keys that differ in more than case",
			strings.Join(rels, ", "), baseDir)
		findings = append(findings, newValidationError(collectionKey, pathOf[group[0]], "", "", message, nil))
	}
	return findings
}

// validateSingleRecordFile validates one single-record file: it stats, reads,
// parses, and schema-validates the record. It returns the per-file passed/total
// counts (a skipped or directory path counts as 0/0) and any errors found.
func validateSingleRecordFile(collectionKey string, colDef *ingitdb.CollectionDef, filePath string) (int, int, []ingitdb.ValidationError) {
	if skipRecordPath(filePath, colDef.RecordFile) {
		return 0, 0, nil
	}
	info, statErr := os.Stat(filePath)
	if statErr != nil {
		validationErr := newValidationError(collectionKey, filePath, "", "", "failed to stat record file", statErr)
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	if info.IsDir() {
		return 0, 0, nil
	}
	content, readErr := os.ReadFile(filePath)
	if readErr != nil {
		validationErr := newValidationError(collectionKey, filePath, "", "", "failed to read record file", readErr)
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	data, parseErr := ingitdb.ParseRecordContentForCollection(content, colDef)
	if parseErr != nil {
		validationErr := newValidationError(collectionKey, filePath, "", "", "failed to parse record file", parseErr)
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	recordKey, nameErr := decodedRecordKey(colDef, filePath)
	if nameErr != nil {
		return 0, 1, []ingitdb.ValidationError{newValidationError(collectionKey, filePath, recordKey, "", fileNameFinding(nameErr), nil)}
	}
	recordErrors := validateRecordData(collectionKey, filePath, recordKey, colDef, data)
	if len(recordErrors) > 0 {
		return 0, 1, recordErrors
	}
	return 1, 1, nil
}

func singleRecordGlobPattern(colDef *ingitdb.CollectionDef) string {
	baseDir := filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
	fileName := colDef.RecordFile.Name
	if strings.Contains(fileName, "{key}") {
		globName := strings.ReplaceAll(fileName, "{key}", "*")
		return filepath.Join(baseDir, globName)
	}
	return filepath.Join(baseDir, fileName)
}

func skipRecordPath(filePath string, rfd *ingitdb.RecordFileDef) bool {
	name := filepath.Base(filePath)
	if strings.HasPrefix(name, ".") {
		return true
	}
	return rfd.IsExcluded(name)
}

func recordKeyFromFilePath(filePath string) string {
	name := filepath.Base(filePath)
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

// decodedRecordKey returns the record key of a record file path: its key
// name decoded with the collection's key_encoding. When the name is not one
// the encoding writes, or (with a declared encoding) is longer than a file
// name may be, it returns the name as it is with the error.
func decodedRecordKey(colDef *ingitdb.CollectionDef, filePath string) (string, error) {
	name := recordNameFromCollectionFilePath(colDef, filePath)
	if colDef == nil || colDef.RecordFile == nil {
		return name, nil
	}
	key, err := colDef.RecordFile.DecodeKey(name)
	if err != nil {
		return name, err
	}
	if err := colDef.RecordFile.CheckKeyFits(key); err != nil {
		return key, err
	}
	return key, nil
}

// fileNameFinding is the message of a record file whose name names no record:
// one the key encoding never writes, or one over the file-name length limit.
// The reason is said once (the finding carries no separate error).
func fileNameFinding(err error) string {
	if errors.Is(err, ingitdb.ErrKeyNotStorable) {
		return "the file name is too long to be stored on every file system: " + err.Error()
	}
	return "the file name is not one the collection's key_encoding writes, so it names no record; rename the file: " + err.Error()
}

// recordKeyFromCollectionFilePath is decodedRecordKey without the error.
func recordKeyFromCollectionFilePath(colDef *ingitdb.CollectionDef, filePath string) string {
	key, _ := decodedRecordKey(colDef, filePath)
	return key
}

// recordNameFromCollectionFilePath returns the key name part of a record
// file path, as it is on disk.
func recordNameFromCollectionFilePath(colDef *ingitdb.CollectionDef, filePath string) string {
	if colDef == nil || colDef.RecordFile == nil || !strings.Contains(colDef.RecordFile.Name, "{key}") {
		return recordKeyFromFilePath(filePath)
	}
	baseDir := filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
	rel, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		return recordKeyFromFilePath(filePath)
	}
	rel = filepath.ToSlash(rel)
	prefix, rest, _ := strings.Cut(filepath.ToSlash(colDef.RecordFile.Name), "{key}")
	keySuffix, _, _ := strings.Cut(rest, "/")
	rel = strings.TrimPrefix(rel, prefix)
	if before, _, found := strings.Cut(rel, "/"); found {
		return strings.TrimSuffix(before, keySuffix)
	}
	return strings.TrimSuffix(rel, keySuffix)
}

func validateMapOfRecordsFile(collectionKey string, colDef *ingitdb.CollectionDef) (int, int, []ingitdb.ValidationError) {
	filePath := collectionRecordFilePath(colDef)
	content, ok, validationErr := readRecordsFile(collectionKey, filePath)
	if !ok {
		if validationErr.Message == "" {
			return 0, 0, nil
		}
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	records, err := ingitdb.ParseMapOfRecordsContent(content, colDef.RecordFile.Format)
	if err != nil {
		validationErr = newValidationError(collectionKey, filePath, "", "", "failed to parse records file", err)
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	passed := 0
	var errors []ingitdb.ValidationError
	for recordKey, data := range records {
		recordErrors := validateRecordData(collectionKey, filePath, recordKey, colDef, data)
		if len(recordErrors) > 0 {
			errors = append(errors, recordErrors...)
			continue
		}
		passed++
	}
	total := len(records)
	return passed, total, errors
}

func validateListOfRecordsFile(collectionKey string, colDef *ingitdb.CollectionDef) (int, int, []ingitdb.ValidationError) {
	filePath := collectionRecordFilePath(colDef)
	content, ok, validationErr := readRecordsFile(collectionKey, filePath)
	if !ok {
		if validationErr.Message == "" {
			return 0, 0, nil
		}
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	rows, err := parseListRows(content, colDef)
	if err != nil {
		validationErr = newValidationError(collectionKey, filePath, "", "", "failed to parse records file", err)
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	passed := 0
	var errors []ingitdb.ValidationError
	seenImportedIDs := make(map[string]bool)
	for _, row := range rows {
		recordKey, keyOK := ingitdb.ResolveListRecordKey(row, colDef)
		if !keyOK {
			validationErr = newValidationError(collectionKey, filePath, "", "", "list record has no resolvable key", nil)
			errors = append(errors, validationErr)
			continue
		}
		if colDef.SourceSchema != nil && colDef.SourceSchema.KeyMode != "" {
			if seenImportedIDs[recordKey] {
				errors = append(errors, newValidationError(collectionKey, filePath, recordKey, "$ID", "duplicate imported transport ID", nil))
				continue
			}
			seenImportedIDs[recordKey] = true
		}
		recordErrors := validateRecordData(collectionKey, filePath, recordKey, colDef, row)
		if len(recordErrors) > 0 {
			errors = append(errors, recordErrors...)
			continue
		}
		passed++
	}
	total := len(rows)
	return passed, total, errors
}

func collectionRecordFilePath(colDef *ingitdb.CollectionDef) string {
	baseDir := filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
	return filepath.Join(baseDir, colDef.RecordFile.Name)
}

func readRecordsFile(collectionKey, filePath string) ([]byte, bool, ingitdb.ValidationError) {
	content, err := os.ReadFile(filePath)
	if err == nil {
		return content, true, ingitdb.ValidationError{}
	}
	if os.IsNotExist(err) {
		return nil, false, ingitdb.ValidationError{}
	}
	validationErr := newValidationError(collectionKey, filePath, "", "", "failed to read records file", err)
	return nil, false, validationErr
}

func parseListRows(content []byte, colDef *ingitdb.CollectionDef) ([]map[string]any, error) {
	switch colDef.RecordFile.Format {
	case ingitdb.RecordFormatCSV:
		data, err := ingitdb.ParseRecordContentForCollection(content, colDef)
		if err != nil {
			return nil, err
		}
		return data[listRecordsKey].([]map[string]any), nil
	case ingitdb.RecordFormatINGR:
		records, err := ingitdb.ParseMapOfRecordsContent(content, ingitdb.RecordFormatINGR)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]any, 0, len(records))
		for key, record := range records {
			record["$ID"] = key
			rows = append(rows, record)
		}
		return rows, nil
	default:
		return ingitdb.ParseListOfRecordsContent(content, colDef.RecordFile.Format)
	}
}

// reservedFieldPrefix marks a key the library owns rather than the schema.
// datavalidator sets record["$ID"] for INGR input and parse.go sets row["$ID"]
// for CSV, so these keys appear in records without ever being declared.
const reservedFieldPrefix = "$"

// reservedIDField is the synthetic per-record key the library supplies from the
// record's identity: validator.go sets it for INGR, parse.go for CSV, and
// validateRecordData binds it from recordKey for per-file records.
const reservedIDField = "$ID"

// undeclaredFieldErrors reports record fields with no corresponding column.
//
// This walks the record's keys, not the schema's columns. Iterating columns —
// which is all the rest of validateRecordData does — can never surface a field
// the schema does not mention, so a typo'd field name was silently stored and
// silently ignored forever.
//
// Keys prefixed with reservedFieldPrefix are exempt whether or not the
// definition declares them: they are injected by the library itself, so
// flagging them would fail every INGR and CSV collection on a field inGitDB
// added. A definition may still declare a $-prefixed column (demo-ingitdb's
// order_details declares "$ID"), and doing so does not change the outcome.
func undeclaredFieldErrors(
	collectionKey string,
	filePath string,
	recordKey string,
	colDef *ingitdb.CollectionDef,
	data map[string]any,
) []ingitdb.ValidationError {
	var errors []ingitdb.ValidationError
	fields := make([]string, 0, len(data))
	for fieldName := range data {
		fields = append(fields, fieldName)
	}
	// Deterministic order: Go map ranging is randomised, and callers compare
	// and display these.
	slices.Sort(fields)
	for _, fieldName := range fields {
		if strings.HasPrefix(fieldName, reservedFieldPrefix) {
			continue
		}
		if _, declared := colDef.Columns[fieldName]; declared {
			continue
		}
		message := fmt.Sprintf("undeclared field %q: no column of that name in the collection definition", fieldName)
		errors = append(errors, newValidationError(collectionKey, filePath, recordKey, fieldName, message, nil))
	}
	return errors
}

// columnIsRequired reports whether a column with no value in this record is
// required: directly via `required`, or conditionally via `required_when`.
//
// The two are mutually exclusive at definition-load time, so there is no
// precedence to resolve here.
func columnIsRequired(columnDef *ingitdb.ColumnDef, colDef *ingitdb.CollectionDef, data map[string]any) (bool, error) {
	if columnDef.RequiredWhen == "" {
		return columnDef.Required, nil
	}
	result, err := ingitdb.EvaluateFormula(columnDef.RequiredWhen, storedFields(colDef, data))
	if err != nil {
		return false, fmt.Errorf("required_when: %w", err)
	}
	required, ok := result.(bool)
	if !ok {
		// Deliberately not a truthiness coercion: `required_when: 'name'` must
		// be an error, not "required when name is non-empty".
		return false, fmt.Errorf("required_when must evaluate to True or False, got %T", result)
	}
	return required, nil
}

// storedFields builds the binding environment for a required_when expression:
// every stored (non-computed) column of the collection, taking the record's
// value where present and nil otherwise.
//
// Binding every declared column rather than only the record's own keys matters
// twice. A sibling the record omits is declared, so asking about it is
// legitimate and must yield None rather than "predeclared variable X is
// uninitialized". And the resulting name set is identical for every record of
// the collection, so the compiled-program cache holds one entry per expression
// instead of one per field shape.
//
// Computed columns are excluded, matching the load-time rule that
// required_when may reference only stored fields.
func storedFields(colDef *ingitdb.CollectionDef, data map[string]any) map[string]any {
	fields := make(map[string]any, len(colDef.Columns))
	for name, def := range colDef.Columns {
		if def.Formula != "" {
			continue
		}
		fields[name] = data[name]
	}
	return fields
}

// ValidateRecordData validates a single in-memory record's field values against
// the collection's schema: declared column types, required fields, and the
// rule that computed-column values must not be stored. It returns the schema
// violations found (empty when the record is valid). Use it to check records
// that are not yet persisted to a file, e.g. an auto-merge result before it is
// staged.
func ValidateRecordData(colDef *ingitdb.CollectionDef, recordKey string, data map[string]any) []ingitdb.ValidationError {
	return validateRecordData(colDef.ID, "", recordKey, colDef, data)
}

func validateRecordData(
	collectionKey string,
	filePath string,
	recordKey string,
	colDef *ingitdb.CollectionDef,
	data map[string]any,
) []ingitdb.ValidationError {
	var errors []ingitdb.ValidationError
	errors = append(errors, undeclaredFieldErrors(collectionKey, filePath, recordKey, colDef, data)...)
	for fieldName, columnDef := range colDef.Columns {
		if columnDef.Formula != "" {
			if _, present := data[fieldName]; present {
				message := fmt.Sprintf("computed column %q must not be stored", fieldName)
				validationErr := newValidationError(collectionKey, filePath, recordKey, fieldName, message, nil)
				errors = append(errors, validationErr)
			}
			continue
		}
		value, ok := data[fieldName]
		if (!ok || value == nil) && fieldName == reservedIDField && recordKey != "" {
			// $ID is the record's own key. INGR and CSV inject it into the
			// record before validation; per-file records legitimately omit it
			// (it is the filename), so bind it here from the known key rather
			// than reporting it missing. Bound locally, never written back into
			// data — the exported entry point's caller serialises that map.
			value, ok = recordKey, true
		}
		if !ok || value == nil {
			required, err := columnIsRequired(columnDef, colDef, data)
			if err != nil {
				errors = append(errors, newValidationError(collectionKey, filePath, recordKey, fieldName, err.Error(), nil))
				continue
			}
			if required {
				validationErr := newValidationError(collectionKey, filePath, recordKey, fieldName, "missing required field", nil)
				errors = append(errors, validationErr)
			}
			continue
		}
		if key, isKey := value.(string); isKey && fieldName == reservedIDField && keyTypeFormats[columnDef.Type] != "" {
			// A record key is always text (a file name, a map key, a CSV cell),
			// so a typed $ID is checked by parsing the key, not by the Go type
			// of the value. See column-validation#req:record-key-type-checked.
			parsed, keyOK := parseRecordKey(key, columnDef.Type)
			if !keyOK {
				message := fmt.Sprintf("wrong type for field %q: %q is not a valid %s (%s)",
					fieldName, key, columnDef.Type, keyTypeFormats[columnDef.Type])
				errors = append(errors, newValidationError(collectionKey, filePath, recordKey, fieldName, message, nil))
				continue
			}
			value = parsed
		} else if !valueMatchesColumnType(value, columnDef.Type) {
			message := fmt.Sprintf("wrong type for field %q: expected %s, got %T", fieldName, columnDef.Type, value)
			validationErr := newValidationError(collectionKey, filePath, recordKey, fieldName, message, nil)
			errors = append(errors, validationErr)
			continue
		}
		if err := checkEnum(fieldName, value, columnDef.Enum); err != nil {
			errors = append(errors, newValidationError(collectionKey, filePath, recordKey, fieldName, err.Error(), nil))
			continue
		}
		if err := checkValueRange(fieldName, value, columnDef.MinValue, columnDef.MaxValue); err != nil {
			errors = append(errors, newValidationError(collectionKey, filePath, recordKey, fieldName, err.Error(), nil))
			continue
		}
		if err := checkLength(fieldName, value, columnDef.Length, columnDef.MinLength, columnDef.MaxLength); err != nil {
			errors = append(errors, newValidationError(collectionKey, filePath, recordKey, fieldName, err.Error(), nil))
			continue
		}
		for _, finding := range shapeFindings(fieldName, value, columnDef) {
			errors = append(errors, newValidationError(collectionKey, filePath, recordKey, finding.field, finding.message, nil))
		}
	}
	return errors
}

// shapeFinding is one violation inside a structured value; field is the path
// to it, e.g. "slots.sl1.durationInMinutes" or "members[2].role".
type shapeFinding struct {
	field   string
	message string
}

// shapeFindings checks a value that already passed its own column checks
// against the column's declared Columns, Values and Items, recursively. A
// column that declares none of them yields nothing, so a column is checked
// exactly as before unless its definition opts in.
func shapeFindings(path string, value any, def *ingitdb.ColumnDef) []shapeFinding {
	var findings []shapeFinding
	if object, isObject := value.(map[string]any); isObject && len(def.Columns) > 0 {
		for _, name := range slices.Sorted(maps.Keys(object)) {
			if _, declared := def.Columns[name]; !declared && !strings.HasPrefix(name, reservedFieldPrefix) {
				message := fmt.Sprintf("undeclared field %q: no nested column of that name in the definition of %q", path+"."+name, path)
				findings = append(findings, shapeFinding{path + "." + name, message})
			}
		}
		for _, name := range slices.Sorted(maps.Keys(def.Columns)) {
			findings = append(findings, nestedValueFindings(path+"."+name, object[name], def.Columns[name])...)
		}
	}
	if def.Values != nil {
		if object, isObject := value.(map[string]any); isObject {
			for _, key := range slices.Sorted(maps.Keys(object)) {
				findings = append(findings, nestedValueFindings(path+"."+key, object[key], def.Values)...)
			}
		}
	}
	if items, isList := value.([]any); isList && def.Items != nil {
		for i, item := range items {
			findings = append(findings, nestedValueFindings(fmt.Sprintf("%s[%d]", path, i), item, def.Items)...)
		}
	}
	return findings
}

// nestedValueFindings checks one nested value with the same checks, in the
// same order and with the same messages, as a top-level column: required,
// type, enum, value range, length, then its own shape.
func nestedValueFindings(path string, value any, def *ingitdb.ColumnDef) []shapeFinding {
	if value == nil {
		if def.Required {
			return []shapeFinding{{path, "missing required field"}}
		}
		return nil
	}
	if !valueMatchesColumnType(value, def.Type) {
		return []shapeFinding{{path, fmt.Sprintf("wrong type for field %q: expected %s, got %T", path, def.Type, value)}}
	}
	if err := checkEnum(path, value, def.Enum); err != nil {
		return []shapeFinding{{path, err.Error()}}
	}
	if err := checkValueRange(path, value, def.MinValue, def.MaxValue); err != nil {
		return []shapeFinding{{path, err.Error()}}
	}
	if err := checkLength(path, value, def.Length, def.MinLength, def.MaxLength); err != nil {
		return []shapeFinding{{path, err.Error()}}
	}
	return shapeFindings(path, value, def)
}

// keyTypeFormats names, for every column type a record key can be parsed as,
// the text form a key of that type must have. Types missing here (string,
// any, maps, lists) keep the plain value type check.
var keyTypeFormats = map[ingitdb.ColumnType]string{
	ingitdb.ColumnTypeInt:      "a base-10 integer without a plus sign or leading zeros",
	ingitdb.ColumnTypeFloat:    "a finite decimal number in shortest form, e.g. 1.5 (not 1.50, 1e+06, +1, .5 or -0)",
	ingitdb.ColumnTypeBool:     "true or false",
	ingitdb.ColumnTypeDate:     "YYYY-MM-DD",
	ingitdb.ColumnTypeTime:     "H:MM or H:MM:SS (hour of one or two digits), optional fractional seconds",
	ingitdb.ColumnTypeDateTime: "YYYY-MM-DDTHH:MM:SS, optional fractional seconds and zone, e.g. 2006-01-02T15:04:05Z",
}

// parseRecordKey parses a record key as columnType, one of the keyTypeFormats
// types. Numbers and booleans come back parsed, so enum and range checks
// compare them as numbers; temporal keys stay the key text.
func parseRecordKey(key string, columnType ingitdb.ColumnType) (any, bool) {
	switch columnType {
	case ingitdb.ColumnTypeInt:
		// Canonical form only, so "7" and "007" cannot be two records with
		// one value.
		n, err := strconv.ParseInt(key, 10, 64)
		return n, err == nil && strconv.FormatInt(n, 10) == key
	case ingitdb.ColumnTypeFloat:
		// Canonical (shortest decimal) form only, as for int keys, so "1.5"
		// and "1.50" cannot be two records with one value. This also rules
		// out NaN, Inf, hex, exponents, "+", "_" and "-0".
		f, err := strconv.ParseFloat(key, 64)
		finite := err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
		return f, finite && key != "-0" && strconv.FormatFloat(f, 'f', -1, 64) == key
	case ingitdb.ColumnTypeBool:
		return key == "true", key == "true" || key == "false"
	case ingitdb.ColumnTypeDate:
		_, err := time.Parse(time.DateOnly, key)
		return key, err == nil
	case ingitdb.ColumnTypeTime:
		_, err := time.Parse(time.TimeOnly, key)
		if err != nil {
			_, err = time.Parse("15:04", key)
		}
		return key, err == nil
	default: // ingitdb.ColumnTypeDateTime
		_, err := time.Parse(time.RFC3339Nano, key)
		if err != nil {
			_, err = time.Parse("2006-01-02T15:04:05", key) // no zone
		}
		return key, err == nil
	}
}

// checkEnum reports whether value is one of the column's permitted members.
// An empty enum means the column is unconstrained by this rule.
func checkEnum(fieldName string, value any, enum []any) error {
	if len(enum) == 0 {
		return nil
	}
	for _, member := range enum {
		if enumValuesEqual(member, value) {
			return nil
		}
	}
	permitted := make([]string, 0, len(enum))
	for _, member := range enum {
		permitted = append(permitted, fmt.Sprintf("%v", member))
	}
	return fmt.Errorf("value %v for field %q is not one of the permitted values: %s",
		value, fieldName, strings.Join(permitted, ", "))
}

// enumValuesEqual compares a declared enum member against a record value.
// YAML and JSON decode numbers into different Go types (int vs float64), so a
// direct == would reject a legitimate member on type alone; compare numerics by
// value and everything else directly.
func enumValuesEqual(member, value any) bool {
	if member == value {
		return true
	}
	mf, mok := numericValue(member)
	vf, vok := numericValue(value)
	return mok && vok && mf == vf
}

func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func valueMatchesColumnType(value any, columnType ingitdb.ColumnType) bool {
	switch columnType {
	case ingitdb.ColumnTypeAny:
		return true
	case ingitdb.ColumnTypeString:
		_, ok := value.(string)
		return ok
	case ingitdb.ColumnTypeInt:
		return isIntegerValue(value)
	case ingitdb.ColumnTypeFloat:
		return isNumberValue(value)
	case ingitdb.ColumnTypeBool:
		_, ok := value.(bool)
		return ok
	case ingitdb.ColumnTypeDate, ingitdb.ColumnTypeTime, ingitdb.ColumnTypeDateTime:
		return isTemporalValue(value)
	case ingitdb.ColumnTypeL10N:
		return isStringMap(value)
	default:
		if elem, ok := ingitdb.ListElementType(columnType); ok {
			return valueMatchesListType(value, elem)
		}
		if strings.HasPrefix(string(columnType), "map[") {
			return isMapValue(value)
		}
		// Unrecognised types are NOT permissive. Returning true here is what
		// let `type: number` validate every value in e2e-test-ingitdb. Column
		// types are rejected at definition-load by ingitdb.ValidateColumnType,
		// so a loaded definition never reaches this; a caller constructing a
		// CollectionDef in memory can, and must not be silently trusted.
		return false
	}
}

func isIntegerValue(value any) bool {
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		floatValue := float64(v)
		return math.Trunc(floatValue) == floatValue
	case float64:
		return math.Trunc(v) == v
	default:
		return false
	}
}

func isNumberValue(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	default:
		return false
	}
}

func isTemporalValue(value any) bool {
	switch value.(type) {
	case string, time.Time:
		return true
	default:
		return false
	}
}

func isStringMap(value any) bool {
	switch typed := value.(type) {
	case map[string]string:
		return true
	case map[string]any:
		for _, item := range typed {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isMapValue(value any) bool {
	switch value.(type) {
	case map[string]any, map[string]string:
		return true
	default:
		return false
	}
}

func newValidationError(collectionKey, filePath, recordKey, fieldName, message string, err error) ingitdb.ValidationError {
	return ingitdb.ValidationError{
		Severity:     ingitdb.SeverityError,
		CollectionID: collectionKey,
		FilePath:     filePath,
		RecordKey:    recordKey,
		FieldName:    fieldName,
		Message:      message,
		Err:          err,
	}
}

// checkRecordCountConstraints enforces a collection's min_records_count /
// max_records_count against how many records it holds. count is the record
// count already produced by the schema pass (validateCollectionRecords' total):
// for a single-record-per-file collection that is the file count, and for a
// map- or list-of-records collection it is the number of parsed entries — which
// is what an author means by "records count". It deliberately is NOT a raw
// countRecords directory scan, which would report 1 for a map/list collection
// whose records all live in one backing file.
//
// A violation is a collection-level finding: no field, no record key, naming
// the collection, the bound, and the actual count. Bounds are inclusive, and an
// impossible bound is already rejected at definition-load, so both pointers here
// are either nil or a sane value.
func checkRecordCountConstraints(collectionKey string, colDef *ingitdb.CollectionDef, count int) []ingitdb.ValidationError {
	var errors []ingitdb.ValidationError
	if colDef.MinRecordsCount != nil && count < *colDef.MinRecordsCount {
		message := fmt.Sprintf("collection %q holds %d record(s), fewer than the declared min_records_count of %d",
			collectionKey, count, *colDef.MinRecordsCount)
		errors = append(errors, newValidationError(collectionKey, "", "", "", message, nil))
	}
	if colDef.MaxRecordsCount != nil && count > *colDef.MaxRecordsCount {
		message := fmt.Sprintf("collection %q holds %d record(s), more than the declared max_records_count of %d",
			collectionKey, count, *colDef.MaxRecordsCount)
		errors = append(errors, newValidationError(collectionKey, "", "", "", message, nil))
	}
	return errors
}

// countRecords counts the number of record keys in a collection directory.
// When a $records/ subdirectory exists (used for per-key record files), it
// counts entries inside that directory instead of at the collection root.
func countRecords(colDef *ingitdb.CollectionDef) (int, error) {
	collectionPath := colDef.DirPath
	exts := expectedRecordExtensions(colDef)
	recordsSubDir := filepath.Join(collectionPath, "$records")
	if info, err := os.Stat(recordsSubDir); err == nil && info.IsDir() {
		return countEntries(recordsSubDir, exts, colDef.RecordFile)
	}
	return countEntries(collectionPath, exts, colDef.RecordFile)
}

// expectedRecordExtensions returns the file extensions that count as record
// files for this collection.
//
// The authoritative source is the collection's `record_file.name` template
// (e.g. `{key}.md`) — whatever extension it ends with is the single
// extension that records use. This naturally extends to any future format
// without needing changes here.
//
// When no `RecordFile` is declared (older test fixtures), the legacy
// permissive set (`.yaml`, `.yml`, `.json`) is returned so existing
// behavior is preserved.
func expectedRecordExtensions(colDef *ingitdb.CollectionDef) map[string]struct{} {
	if colDef.RecordFile != nil && colDef.RecordFile.Name != "" {
		ext := strings.ToLower(filepath.Ext(colDef.RecordFile.Name))
		if ext != "" {
			return map[string]struct{}{ext: {}}
		}
	}
	return map[string]struct{}{".yaml": {}, ".yml": {}, ".json": {}}
}

func countEntries(dirPath string, exts map[string]struct{}, rfd *ingitdb.RecordFileDef) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	// Count unique record keys. A key may appear as a plain file (e.g. USD.yaml)
	// or as a subdirectory (e.g. ord001/ holding subcollection data), or both.
	// We deduplicate by stripping the file extension so that a record with both
	// an ord001.yaml and an ord001/ directory is counted only once.
	seen := make(map[string]struct{})
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "$records" {
			continue
		}
		if rfd != nil && rfd.IsExcluded(name) {
			continue
		}
		if entry.IsDir() {
			seen[name] = struct{}{}
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := exts[ext]; ok {
			seen[strings.TrimSuffix(name, filepath.Ext(name))] = struct{}{}
		}
	}
	return len(seen), nil
}

// valueMatchesListType reports whether value is a list whose every element
// matches elem. A []T column requires the value to BE a list — including
// []any, which accepts any element but not a non-list.
func valueMatchesListType(value any, elem ingitdb.ColumnType) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if !valueMatchesColumnType(item, elem) {
			return false
		}
	}
	return true
}

// checkValueRange enforces min_value/max_value inclusively on a numeric value.
// Both bounds are pointers so a declared zero is distinguishable from unset.
func checkValueRange(fieldName string, value any, minValue, maxValue *float64) error {
	if minValue == nil && maxValue == nil {
		return nil
	}
	n, ok := numericValue(value)
	if !ok {
		// A non-numeric value in a bounded column is caught by the type check;
		// declaring bounds on a non-numeric column is a definition-load error.
		return nil
	}
	if minValue != nil && n < *minValue {
		return fmt.Errorf("value %v for field %q is below min_value %v", value, fieldName, *minValue)
	}
	if maxValue != nil && n > *maxValue {
		return fmt.Errorf("value %v for field %q is above max_value %v", value, fieldName, *maxValue)
	}
	return nil
}

// checkLength enforces length/min_length/max_length. Length is measured as
// Unicode code points for a string, element count for a list, entry count for
// a map. All three bounds are pointers so a declared zero is enforced rather
// than read as unset.
func checkLength(fieldName string, value any, exact, minLen, maxLen *int) error {
	if exact == nil && minLen == nil && maxLen == nil {
		return nil
	}
	n, ok := valueLength(value)
	if !ok {
		// Length is undefined for this value's kind; declaring a length
		// constraint on such a column is a definition-load error.
		return nil
	}
	if exact != nil && n != *exact {
		return fmt.Errorf("length %d for field %q does not match required length %d", n, fieldName, *exact)
	}
	if minLen != nil && n < *minLen {
		return fmt.Errorf("length %d for field %q is below min_length %d", n, fieldName, *minLen)
	}
	if maxLen != nil && n > *maxLen {
		return fmt.Errorf("length %d for field %q is above max_length %d", n, fieldName, *maxLen)
	}
	return nil
}

// valueLength reports a value's length and whether length is defined for it.
// Strings count runes rather than bytes, so a multi-byte character counts once.
func valueLength(value any) (int, bool) {
	switch v := value.(type) {
	case string:
		return utf8.RuneCountInString(v), true
	case []any:
		return len(v), true
	case map[string]any:
		return len(v), true
	default:
		return 0, false
	}
}

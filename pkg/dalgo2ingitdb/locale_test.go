package dalgo2ingitdb

import (
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestApplyLocaleToRead(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
		"titles": {Type: ingitdb.ColumnTypeL10N},
	}
	data := map[string]any{
		"titles": map[string]any{"en": "Work", "ru": "Работа"},
	}
	result := ApplyLocaleToRead(data, cols)

	if result["title"] != "Work" {
		t.Fatalf("expected title=Work, got %v", result["title"])
	}
	titles, ok := result["titles"].(map[string]any)
	if !ok {
		t.Fatalf("expected titles to be a map, got %T", result["titles"])
	}
	if _, hasEN := titles["en"]; hasEN {
		t.Fatal("expected 'en' removed from titles")
	}
	if titles["ru"] != "Работа" {
		t.Fatalf("expected titles.ru=Работа, got %v", titles["ru"])
	}
}

func TestApplyLocaleToWrite_ShortcutKept(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
		"titles": {Type: ingitdb.ColumnTypeL10N},
	}
	data := map[string]any{
		"title":  "Work",
		"titles": map[string]any{"ru": "Работа"},
	}
	result := ApplyLocaleToWrite(data, cols)

	if result["title"] != "Work" {
		t.Fatalf("expected title=Work kept in result, got %v", result["title"])
	}
	titles, ok := result["titles"].(map[string]any)
	if !ok {
		t.Fatalf("expected titles to be a map, got %T", result["titles"])
	}
	if _, hasEN := titles["en"]; hasEN {
		t.Fatal("expected 'en' NOT in titles (value already in shortcut column)")
	}
	if titles["ru"] != "Работа" {
		t.Fatalf("expected titles.ru=Работа preserved, got %v", titles["ru"])
	}
}

func TestApplyLocaleToWrite_PrimaryLocalePromoted(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
		"titles": {Type: ingitdb.ColumnTypeL10N},
	}
	// Primary locale supplied inside the pair map — should be promoted to shortcut column.
	data := map[string]any{
		"titles": map[string]any{"en": "Work", "ru": "Работа"},
	}
	result := ApplyLocaleToWrite(data, cols)

	if result["title"] != "Work" {
		t.Fatalf("expected title=Work promoted from titles.en, got %v", result["title"])
	}
	titles, ok := result["titles"].(map[string]any)
	if !ok {
		t.Fatalf("expected titles to be a map, got %T", result["titles"])
	}
	if _, hasEN := titles["en"]; hasEN {
		t.Fatal("expected 'en' removed from titles after promotion")
	}
	if titles["ru"] != "Работа" {
		t.Fatalf("expected titles.ru=Работа preserved, got %v", titles["ru"])
	}
}

func TestApplyLocaleToWrite_TitleOnlyNoTitles(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
		"titles": {Type: ingitdb.ColumnTypeL10N},
	}
	data := map[string]any{"title": "Solo"}
	result := ApplyLocaleToWrite(data, cols)

	if result["title"] != "Solo" {
		t.Fatalf("expected title=Solo kept, got %v", result["title"])
	}
	if _, hasTitles := result["titles"]; hasTitles {
		t.Fatal("expected no 'titles' key when none was provided")
	}
}

func TestApplyLocaleToWrite_PrimaryLocaleOnlyInTitles(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
		"titles": {Type: ingitdb.ColumnTypeL10N},
	}
	// Only primary locale in titles — after promotion titles should be dropped.
	data := map[string]any{
		"titles": map[string]any{"en": "Solo"},
	}
	result := ApplyLocaleToWrite(data, cols)

	if result["title"] != "Solo" {
		t.Fatalf("expected title=Solo promoted, got %v", result["title"])
	}
	if _, hasTitles := result["titles"]; hasTitles {
		t.Fatal("expected empty 'titles' map to be dropped")
	}
}

func TestApplyLocaleToRead_EmptyColumns(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"title":  "Work",
		"titles": map[string]any{"en": "Work", "ru": "Работа"},
	}
	result := ApplyLocaleToRead(data, map[string]*ingitdb.ColumnDef{})

	// With no column definitions, data should be returned unchanged.
	if len(result) != len(data) {
		t.Errorf("expected result length %d, got %d", len(data), len(result))
	}
	if result["title"] != "Work" {
		t.Errorf("expected title=Work, got %v", result["title"])
	}
}

func TestApplyLocaleToRead_NoPairField(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
		"titles": {Type: ingitdb.ColumnTypeL10N},
	}
	// Data has only the shortcut column, no pair map.
	data := map[string]any{
		"title": "Work",
	}
	result := ApplyLocaleToRead(data, cols)

	if result["title"] != "Work" {
		t.Errorf("expected title=Work, got %v", result["title"])
	}
	if _, hasTitles := result["titles"]; hasTitles {
		t.Error("expected no 'titles' key when not present in input")
	}
}

func TestApplyLocaleToRead_PairFieldNotMap(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
		"titles": {Type: ingitdb.ColumnTypeL10N},
	}
	// Pair field exists but is not a map — should be skipped.
	data := map[string]any{
		"titles": "not a map",
	}
	result := ApplyLocaleToRead(data, cols)

	// "titles" should remain unchanged since it's not a map.
	if result["titles"] != "not a map" {
		t.Errorf("expected titles='not a map', got %v", result["titles"])
	}
	if _, hasTitle := result["title"]; hasTitle {
		t.Error("expected no 'title' key when pair field is not a map")
	}
}

func TestApplyLocaleToRead_LocaleNotInPairMap(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
		"titles": {Type: ingitdb.ColumnTypeL10N},
	}
	// Pair map exists but doesn't contain the locale key.
	data := map[string]any{
		"titles": map[string]any{"ru": "Работа", "de": "Arbeit"},
	}
	result := ApplyLocaleToRead(data, cols)

	// "title" should not be set since "en" is not in the map.
	if _, hasTitle := result["title"]; hasTitle {
		t.Error("expected no 'title' key when locale not in pair map")
	}
	titles, ok := result["titles"].(map[string]any)
	if !ok {
		t.Fatalf("expected titles to be a map, got %T", result["titles"])
	}
	if titles["ru"] != "Работа" {
		t.Errorf("expected titles.ru=Работа, got %v", titles["ru"])
	}
}

func TestApplyLocaleToWrite_EmptyColumns(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"title":  "Work",
		"titles": map[string]any{"en": "Work", "ru": "Работа"},
	}
	result := ApplyLocaleToWrite(data, map[string]*ingitdb.ColumnDef{})

	// With no column definitions, data should be returned unchanged.
	if len(result) != len(data) {
		t.Errorf("expected result length %d, got %d", len(data), len(result))
	}
	if result["title"] != "Work" {
		t.Errorf("expected title=Work, got %v", result["title"])
	}
}

func TestApplyLocaleToWrite_PairFieldNotMap(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
		"titles": {Type: ingitdb.ColumnTypeL10N},
	}
	// Pair field exists but is not a map — should be skipped.
	data := map[string]any{
		"title":  "Work",
		"titles": "not a map",
	}
	result := ApplyLocaleToWrite(data, cols)

	// Data should be unchanged since pair field is not a map.
	if result["title"] != "Work" {
		t.Errorf("expected title=Work, got %v", result["title"])
	}
	if result["titles"] != "not a map" {
		t.Errorf("expected titles='not a map', got %v", result["titles"])
	}
}

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

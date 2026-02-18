package dalgo2ingitdb

import (
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func collectionForKeyDef() *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"countries": {
				ID: "countries",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}/{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
			"todo.tags": {
				ID: "todo.tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
			"todo.tasks": {
				ID: "todo.tasks",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
		},
	}
}

func TestCollectionForKey_SlashSeparatedID(t *testing.T) {
	t.Parallel()

	def := collectionForKeyDef()
	colDef, key, err := CollectionForKey(def, "countries/ie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if colDef.ID != "countries" {
		t.Errorf("colDef.ID = %q, want %q", colDef.ID, "countries")
	}
	if key != "ie" {
		t.Errorf("key = %q, want %q", key, "ie")
	}
}

func TestCollectionForKey_DotSeparatedNamespacedID(t *testing.T) {
	t.Parallel()

	// "todo.tags/abc" uses "." as namespace separator in the collection part.
	def := collectionForKeyDef()
	colDef, key, err := CollectionForKey(def, "todo.tags/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if colDef.ID != "todo.tags" {
		t.Errorf("colDef.ID = %q, want %q", colDef.ID, "todo.tags")
	}
	if key != "abc" {
		t.Errorf("key = %q, want %q", key, "abc")
	}
}

func TestCollectionForKey_SlashNormalizedNamespacedID(t *testing.T) {
	t.Parallel()

	// "todo/tags/abc" uses "/" as namespace separator (legacy format still accepted).
	def := collectionForKeyDef()
	colDef, key, err := CollectionForKey(def, "todo/tags/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if colDef.ID != "todo.tags" {
		t.Errorf("colDef.ID = %q, want %q", colDef.ID, "todo.tags")
	}
	if key != "abc" {
		t.Errorf("key = %q, want %q", key, "abc")
	}
}

func TestCollectionForKey_LongestMatchWins(t *testing.T) {
	t.Parallel()

	// When two collections share a prefix (e.g. "todo.tags" vs "todo.tasks"),
	// the correct one is selected.
	def := collectionForKeyDef()

	colDef, key, err := CollectionForKey(def, "todo.tasks/task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if colDef.ID != "todo.tasks" {
		t.Errorf("colDef.ID = %q, want %q", colDef.ID, "todo.tasks")
	}
	if key != "task-1" {
		t.Errorf("key = %q, want %q", key, "task-1")
	}
}

func TestCollectionForKey_CollectionNotFound(t *testing.T) {
	t.Parallel()

	def := collectionForKeyDef()
	_, _, err := CollectionForKey(def, "no/such/collection/key")
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
}

func TestCollectionForKey_MissingRecordKey(t *testing.T) {
	t.Parallel()

	def := collectionForKeyDef()
	// ID ends right after the collection prefix — no key part.
	_, _, err := CollectionForKey(def, "countries/")
	if err == nil {
		t.Fatal("expected error when record key is empty")
	}
}

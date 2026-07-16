package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// #9 — geo-ingitdb uses the older layout (a single .ingitdb.yaml plus
// .ingitdb-collection.yaml / .ingitdb-subcol.*) while the reader expects
// .ingitdb/root-collections.yaml. It resolved to zero collections with NO
// error — silent success on an unreadable database, the worst failure mode.
// ReadDefinition must now refuse it.
func TestReadDefinition_RejectsLegacyLayoutSilentEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// The old singular config file, no .ingitdb/ directory.
	if err := os.WriteFile(filepath.Join(dir, ".ingitdb.yaml"), []byte("rootCollections:\n  countries: countries\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadDefinition(dir, ingitdb.Validate())
	if err == nil {
		t.Fatal("a legacy-layout database resolving to zero collections must error, not succeed silently")
	}
	if !strings.Contains(err.Error(), ".ingitdb.yaml") {
		t.Errorf("error should name the legacy marker, got: %v", err)
	}
	if !strings.Contains(err.Error(), "zero collections") {
		t.Errorf("error should explain nothing resolved, got: %v", err)
	}
}

// A legacy .ingitdb-collection.yaml at the root is also a marker.
func TestReadDefinition_RejectsLegacyCollectionMarker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ingitdb-collection.yaml"), []byte("columns:\n  id:\n    type: string\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadDefinition(dir, ingitdb.Validate())
	if err == nil {
		t.Fatal("a legacy .ingitdb-collection.yaml marker with zero collections must error")
	}
}

// A directory with no inGitDB markers at all is NOT a false positive: the
// caller may have pointed at the wrong place, but we do not invent a database
// signal that is not there. Zero collections, no legacy-layout error.
func TestReadDefinition_PlainEmptyDirIsNotFlaggedAsLegacy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def, err := ReadDefinition(dir)
	// It may error for other reasons (no config), but not with the legacy
	// message, and if it succeeds it must be empty.
	if err != nil && strings.Contains(err.Error(), "older layout") {
		t.Errorf("a plain empty dir must not be reported as legacy layout, got: %v", err)
	}
	if err == nil && len(def.Collections) != 0 {
		t.Errorf("expected zero collections, got %d", len(def.Collections))
	}
}

// The current layout still loads. A .ingitdb/ directory is not a legacy marker.
func TestReadDefinition_CurrentLayoutStillLoads(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ingitdb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ingitdb", "root-collections.yaml"), []byte("things: things\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	colDir := filepath.Join(dir, "things", ".collection")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatal(err)
	}
	def := "record_file:\n  name: \"{key}.json\"\n  type: \"map[string]any\"\n  format: json\ncolumns:\n  id:\n    type: string\n"
	if err := os.WriteFile(filepath.Join(colDir, "definition.yaml"), []byte(def), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadDefinition(dir, ingitdb.Validate())
	if err != nil {
		t.Fatalf("current-layout database must load, got: %v", err)
	}
	if len(got.Collections) != 1 {
		t.Errorf("expected 1 collection, got %d", len(got.Collections))
	}
}

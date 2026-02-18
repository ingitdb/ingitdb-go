package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestUpdate_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	initial, err := yaml.Marshal(map[string]any{"name": "Old"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err = os.WriteFile(filepath.Join(dir, "item.yaml"), initial, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2ingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Update(homeDir, getWd, readDef, newDB, logf)
	runErr := runCLICommand(cmd, "--path="+dir, "--id=test/items/item", "--set={name: New}")
	if runErr != nil {
		t.Fatalf("Update: %v", runErr)
	}

	content, readErr := os.ReadFile(filepath.Join(dir, "item.yaml"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]any
	if err = yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if got["name"] != "New" {
		t.Fatalf("expected name=New, got %v", got["name"])
	}
}

func TestUpdate_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2ingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Update(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "--path="+dir, "--id=test/items/ghost", "--set={name: X}")
	if err == nil {
		t.Fatal("expected error for not-found record")
	}
}

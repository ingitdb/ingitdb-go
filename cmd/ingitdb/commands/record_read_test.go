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

func TestReadRecord_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	content, err := yaml.Marshal(map[string]any{"name": "Test"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err = os.WriteFile(filepath.Join(dir, "r1.yaml"), content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2ingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := ReadRecord(homeDir, getWd, readDef, newDB, logf)
	runErr := runCLICommand(cmd, "--path="+dir, "--id=test/items/r1")
	if runErr != nil {
		t.Fatalf("ReadRecord: %v", runErr)
	}
}

func TestReadRecord_NotFound(t *testing.T) {
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

	cmd := ReadRecord(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "--path="+dir, "--id=test/items/ghost")
	if err == nil {
		t.Fatal("expected error for not-found record")
	}
}

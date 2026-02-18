package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type fakeFileReader struct {
	files map[string][]byte
}

func (f fakeFileReader) ReadFile(_ context.Context, filePath string) ([]byte, bool, error) {
	content, ok := f.files[filePath]
	if !ok {
		return nil, false, nil
	}
	return content, true, nil
}

func TestParseGitHubRepoSpec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantRef   string
		wantErr   bool
	}{
		{name: "owner repo only", input: "ingitdb/ingitdb-cli", wantOwner: "ingitdb", wantRepo: "ingitdb-cli"},
		{name: "branch", input: "ingitdb/ingitdb-cli@main", wantOwner: "ingitdb", wantRepo: "ingitdb-cli", wantRef: "main"},
		{name: "tag", input: "ingitdb/ingitdb-cli@v1.2.3", wantOwner: "ingitdb", wantRepo: "ingitdb-cli", wantRef: "v1.2.3"},
		{name: "commit", input: "ingitdb/ingitdb-cli@a1b2c3d", wantOwner: "ingitdb", wantRepo: "ingitdb-cli", wantRef: "a1b2c3d"},
		{name: "invalid", input: "ingitdb", wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec, err := parseGitHubRepoSpec(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGitHubRepoSpec: %v", err)
			}
			if spec.Owner != tt.wantOwner || spec.Repo != tt.wantRepo || spec.Ref != tt.wantRef {
				t.Fatalf("unexpected spec: %+v", spec)
			}
		})
	}
}

func TestResolveRemoteCollectionPath(t *testing.T) {
	t.Parallel()
	rootCollections := map[string]string{
		"countries": "test-ingitdb/countries",
		"todo":      "test-ingitdb/todo/*",
	}
	collectionID, recordKey, collectionPath, err := resolveRemoteCollectionPath(rootCollections, "todo.tags/active")
	if err != nil {
		t.Fatalf("resolveRemoteCollectionPath: %v", err)
	}
	if collectionID != "todo.tags" {
		t.Fatalf("expected collectionID todo.tags, got %s", collectionID)
	}
	if recordKey != "active" {
		t.Fatalf("expected recordKey active, got %s", recordKey)
	}
	if collectionPath != "test-ingitdb/todo/tags" {
		t.Fatalf("expected collectionPath test-ingitdb/todo/tags, got %s", collectionPath)
	}
}

func TestReadRemoteDefinitionForIDWithReader(t *testing.T) {
	t.Parallel()
	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("rootCollections:\n  todo: test-ingitdb/todo/*\n"),
		"test-ingitdb/todo/tags/.ingitdb-collection.yaml": []byte("record_file:\n  name: tags.json\n  type: map[string]map[string]any\n  format: json\ncolumns:\n  title:\n    type: string\n"),
	}}
	def, collectionID, recordKey, err := readRemoteDefinitionForIDWithReader(context.Background(), "todo.tags/active", reader)
	if err != nil {
		t.Fatalf("readRemoteDefinitionForIDWithReader: %v", err)
	}
	if collectionID != "todo.tags" {
		t.Fatalf("expected collectionID todo.tags, got %s", collectionID)
	}
	if recordKey != "active" {
		t.Fatalf("expected recordKey active, got %s", recordKey)
	}
	colDef := def.Collections[collectionID]
	if colDef == nil {
		t.Fatal("expected collection in definition")
	}
	if colDef.DirPath != "test-ingitdb/todo/tags" {
		t.Fatalf("unexpected DirPath: %s", colDef.DirPath)
	}
}

func TestReadRecord_GitHubWithPathUnsupported(t *testing.T) {
	t.Parallel()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("unused")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("unused")
	}
	cmd := readRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=todo.tags/active", "--github=ingitdb/ingitdb-cli", "--path=/tmp/cache")
	if err == nil {
		t.Fatal("expected error for --github with --path")
	}
}

var _ dalgo2ghingitdb.FileReader = (*fakeFileReader)(nil)

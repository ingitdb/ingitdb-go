package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/urfave/cli/v3"
)

type fakeFileReader struct {
	files       map[string][]byte
	directories map[string][]string
}

func (f fakeFileReader) ReadFile(_ context.Context, filePath string) ([]byte, bool, error) {
	content, ok := f.files[filePath]
	if !ok {
		return nil, false, nil
	}
	return content, true, nil
}

func (f fakeFileReader) ListDirectory(_ context.Context, dirPath string) ([]string, error) {
	entries, ok := f.directories[dirPath]
	if !ok {
		return []string{}, nil
	}
	return entries, nil
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
		"todo.tags": "test-ingitdb/todo/tags",
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
		".ingitdb.yaml": []byte("rootCollections:\n  todo.tags: test-ingitdb/todo/tags\n"),
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

func TestGithubToken_FromFlag(t *testing.T) {
	t.Parallel()
	app := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "token"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			token := githubToken(cmd)
			if token != "test-token" {
				t.Fatalf("expected test-token, got %s", token)
			}
			return nil
		},
	}
	err := app.Run(context.Background(), []string{"app", "--token=test-token"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGithubToken_FromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	app := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "token"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			token := githubToken(cmd)
			if token != "env-token" {
				t.Fatalf("expected env-token, got %s", token)
			}
			return nil
		},
	}
	err := app.Run(context.Background(), []string{"app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGithubToken_FlagTakesPrecedence(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	app := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "token"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			token := githubToken(cmd)
			if token != "flag-token" {
				t.Fatalf("expected flag-token, got %s", token)
			}
			return nil
		},
	}
	err := app.Run(context.Background(), []string{"app", "--token=flag-token"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewGitHubConfig(t *testing.T) {
	t.Parallel()
	spec := githubRepoSpec{
		Owner: "testowner",
		Repo:  "testrepo",
		Ref:   "main",
	}
	cfg := newGitHubConfig(spec, "test-token")
	if cfg.Owner != "testowner" {
		t.Fatalf("expected owner testowner, got %s", cfg.Owner)
	}
	if cfg.Repo != "testrepo" {
		t.Fatalf("expected repo testrepo, got %s", cfg.Repo)
	}
	if cfg.Ref != "main" {
		t.Fatalf("expected ref main, got %s", cfg.Ref)
	}
	if cfg.Token != "test-token" {
		t.Fatalf("expected token test-token, got %s", cfg.Token)
	}
}

func TestListCollections_GitHub(t *testing.T) {
	t.Parallel()
	reader := fakeFileReader{
		files: map[string][]byte{
			".ingitdb.yaml": []byte("rootCollections:\n  countries: test-ingitdb/countries\n  todo.tags: test-ingitdb/todo/tags\n"),
		},
	}
	collections, err := listCollectionsFromFileReader(&reader)
	if err != nil {
		t.Fatalf("listCollectionsFromFileReader: %v", err)
	}
	expectedCollections := []string{"countries", "todo.tags"}
	if len(collections) != len(expectedCollections) {
		t.Fatalf("expected %d collections, got %d", len(expectedCollections), len(collections))
	}
	for i, expected := range expectedCollections {
		if collections[i] != expected {
			t.Fatalf("expected collection %q at index %d, got %q", expected, i, collections[i])
		}
	}
}

var _ dalgo2ghingitdb.FileReader = (*fakeFileReader)(nil)

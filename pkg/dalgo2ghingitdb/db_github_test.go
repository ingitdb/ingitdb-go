package dalgo2ghingitdb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type githubFileFixture struct {
	path    string
	content string
}

func TestGitHubDB_GetSingleRecord(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/todo/tags/active.yaml",
		content: "title: Active\n",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "test-ingitdb/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "active")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !record.Exists() {
		t.Fatal("expected record to exist")
	}
	if data["title"] != "Active" {
		t.Fatalf("expected title=Active, got %v", data["title"])
	}
}

func TestGitHubDB_GetMapOfIDRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/todo/tags/tags.json",
		content: `{"active": {"titles": {"en": "Active", "ru": "Активно"}}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "test-ingitdb/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString, Locale: "en"},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "active")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !record.Exists() {
		t.Fatal("expected record to exist")
	}
	if data["title"] != "Active" {
		t.Fatalf("expected title=Active, got %v", data["title"])
	}
}

func TestGitHubDB_GetNotFound(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "test-ingitdb/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "missing")
	record := dal.NewRecordWithData(key, map[string]any{})
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if record.Exists() {
		t.Fatal("expected record to not exist")
	}
}

func newGitHubContentsServer(t *testing.T, fixtures []githubFileFixture) *httptest.Server {
	t.Helper()
	fixtureByPath := make(map[string]string, len(fixtures))
	for _, fixture := range fixtures {
		fixtureByPath[fixture.path] = fixture.content
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathPrefix := "/repos/ingitdb/ingitdb-cli/contents/"
		if !strings.HasPrefix(r.URL.Path, pathPrefix) {
			http.NotFound(w, r)
			return
		}
		requestedPath := strings.TrimPrefix(r.URL.Path, pathPrefix)
		content, ok := fixtureByPath[requestedPath]
		if !ok {
			http.NotFound(w, r)
			return
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		response := map[string]any{
			"type":     "file",
			"encoding": "base64",
			"content":  encoded,
			"name":     path.Base(requestedPath),
			"path":     requestedPath,
		}
		w.Header().Set("Content-Type", "application/json")
		encodeErr := json.NewEncoder(w).Encode(response)
		if encodeErr != nil {
			http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
		}
	})
	server := httptest.NewServer(handler)
	return server
}

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
	"github.com/dal-go/dalgo/update"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/server/auth"
)

// --- fakes ---

// fakeFileReader implements dalgo2ghingitdb.FileReader returning preset content.
type fakeFileReader struct {
	files map[string][]byte
}

func (f *fakeFileReader) ReadFile(_ context.Context, filePath string) ([]byte, bool, error) {
	content, ok := f.files[filePath]
	return content, ok, nil
}

func (f *fakeFileReader) ListDirectory(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// fakeStore holds mutable state shared across fakeTx instances.
type fakeStore struct {
	records map[string]map[string]any // key.String() → data
	deleted map[string]bool
}

func newFakeStore(records map[string]map[string]any) *fakeStore {
	return &fakeStore{records: records, deleted: map[string]bool{}}
}

// fakeReadTx implements dal.ReadTransaction.
type fakeReadTx struct{ s *fakeStore }

var _ dal.ReadTransaction = (*fakeReadTx)(nil)

func (t *fakeReadTx) ID() string                      { return "fake-read" }
func (t *fakeReadTx) Options() dal.TransactionOptions { return nil }
func (t *fakeReadTx) Get(_ context.Context, record dal.Record) error {
	k := record.Key().String()
	if t.s.deleted[k] {
		record.SetError(dal.ErrRecordNotFound)
		return nil
	}
	data, ok := t.s.records[k]
	if !ok {
		record.SetError(dal.ErrRecordNotFound)
		return nil
	}
	record.SetError(nil)
	dst := record.Data().(map[string]any)
	for kk, v := range data {
		dst[kk] = v
	}
	return nil
}
func (t *fakeReadTx) Exists(_ context.Context, key *dal.Key) (bool, error) {
	_, ok := t.s.records[key.String()]
	return ok && !t.s.deleted[key.String()], nil
}
func (t *fakeReadTx) GetMulti(_ context.Context, _ []dal.Record) error { return nil }
func (t *fakeReadTx) ExecuteQueryToRecordsReader(_ context.Context, _ dal.Query) (dal.RecordsReader, error) {
	return nil, fmt.Errorf("not implemented")
}
func (t *fakeReadTx) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	return nil, fmt.Errorf("not implemented")
}

// fakeReadwriteTx implements dal.ReadwriteTransaction.
type fakeReadwriteTx struct{ fakeReadTx }

var _ dal.ReadwriteTransaction = (*fakeReadwriteTx)(nil)

func (t *fakeReadwriteTx) ID() string { return "fake-rw" }
func (t *fakeReadwriteTx) Insert(_ context.Context, record dal.Record, _ ...dal.InsertOption) error {
	record.SetError(nil)
	t.s.records[record.Key().String()] = record.Data().(map[string]any)
	return nil
}
func (t *fakeReadwriteTx) InsertMulti(_ context.Context, _ []dal.Record, _ ...dal.InsertOption) error {
	return nil
}
func (t *fakeReadwriteTx) Set(_ context.Context, record dal.Record) error {
	t.s.records[record.Key().String()] = record.Data().(map[string]any)
	return nil
}
func (t *fakeReadwriteTx) SetMulti(_ context.Context, _ []dal.Record) error { return nil }
func (t *fakeReadwriteTx) Delete(_ context.Context, key *dal.Key) error {
	t.s.deleted[key.String()] = true
	return nil
}
func (t *fakeReadwriteTx) DeleteMulti(_ context.Context, _ []*dal.Key) error { return nil }
func (t *fakeReadwriteTx) Update(_ context.Context, _ *dal.Key, _ []update.Update, _ ...dal.Precondition) error {
	return nil
}
func (t *fakeReadwriteTx) UpdateRecord(_ context.Context, _ dal.Record, _ []update.Update, _ ...dal.Precondition) error {
	return nil
}
func (t *fakeReadwriteTx) UpdateMulti(_ context.Context, _ []*dal.Key, _ []update.Update, _ ...dal.Precondition) error {
	return nil
}

// fakeDB implements dal.DB with a fakeStore.
type fakeDB struct {
	s *fakeStore
}

func (db *fakeDB) ID() string { return "fake" }
func (db *fakeDB) Adapter() dal.Adapter {
	return dal.NewAdapter("fake", "v0.0.1")
}
func (db *fakeDB) Schema() dal.Schema { return nil }
func (db *fakeDB) RunReadonlyTransaction(_ context.Context, f dal.ROTxWorker, _ ...dal.TransactionOption) error {
	return f(context.Background(), &fakeReadTx{s: db.s})
}
func (db *fakeDB) RunReadwriteTransaction(_ context.Context, f dal.RWTxWorker, _ ...dal.TransactionOption) error {
	return f(context.Background(), &fakeReadwriteTx{fakeReadTx: fakeReadTx{s: db.s}})
}
func (db *fakeDB) Get(_ context.Context, record dal.Record) error {
	return (&fakeReadTx{s: db.s}).Get(context.Background(), record)
}
func (db *fakeDB) Exists(_ context.Context, _ *dal.Key) (bool, error) {
	return false, fmt.Errorf("not implemented")
}
func (db *fakeDB) GetMulti(_ context.Context, _ []dal.Record) error {
	return fmt.Errorf("not implemented")
}
func (db *fakeDB) ExecuteQueryToRecordsReader(_ context.Context, _ dal.Query) (dal.RecordsReader, error) {
	return nil, fmt.Errorf("not implemented")
}
func (db *fakeDB) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	return nil, fmt.Errorf("not implemented")
}

// --- helper to build a test handler ---

const rootConfigYAML = `rootCollections:
  countries: data/countries
`

const countryColDefYAML = `record_file:
  name: "{key}.yaml"
  format: yaml
  type: map[string]any
columns:
  title:
    type: string
`

func newTestHandler() (*Handler, *fakeStore) {
	s := newFakeStore(map[string]map[string]any{
		"countries/ie": {"title": "Ireland"},
	})
	h := &Handler{
		newGitHubFileReader: func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
			return &fakeFileReader{files: map[string][]byte{
				".ingitdb.yaml": []byte(rootConfigYAML),
				"data/countries/.ingitdb-collection.yaml": []byte(countryColDefYAML),
			}}, nil
		},
		newGitHubDBWithDef: func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
			return &fakeDB{s: s}, nil
		},
		authConfig: auth.Config{
			GitHubClientID:     "client-id",
			GitHubClientSecret: "client-secret",
			CallbackURL:        "https://api.ingitdb.com/auth/github/callback",
			Scopes:             []string{"public_repo", "read:user"},
			CookieDomain:       ".ingitdb.com",
			CookieName:         "ingitdb_github_token",
			CookieSecure:       true,
			AuthAPIBaseURL:     "https://api.ingitdb.com",
		},
		exchangeCodeForToken: func(ctx context.Context, code string) (string, error) {
			_, _ = ctx, code
			return "oauth-token", nil
		},
		validateToken: func(ctx context.Context, token string) error {
			_, _ = ctx, token
			return nil
		},
		requireAuth: false,
	}
	h.router = h.buildRouter()
	return h, s
}

// --- tests ---

func TestListCollections_MissingDB(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListCollections_InvalidDB(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=badformat", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListCollections_Success(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=owner/repo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var ids []string
	if err := json.NewDecoder(w.Body).Decode(&ids); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(ids) != 1 || ids[0] != "countries" {
		t.Errorf("unexpected collections: %v", ids)
	}
}

func TestReadRecord_MissingKey(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReadRecord_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=countries/xx", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReadRecord_Success(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var data map[string]any
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if data["title"] != "Ireland" {
		t.Errorf("unexpected data: %v", data)
	}
}

func TestCreateRecord_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/de", strings.NewReader("bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateRecord_Success(t *testing.T) {
	t.Parallel()
	h, s := newTestHandler()
	body := `{"title":"Germany"}`
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/de", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := s.records["countries/de"]; !ok {
		t.Error("record not inserted into fake DB")
	}
}

func TestUpdateRecord_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	body := `{"title":"Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/xx", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateRecord_Success(t *testing.T) {
	t.Parallel()
	h, s := newTestHandler()
	body := `{"title":"Ireland Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if title, _ := s.records["countries/ie"]["title"].(string); title != "Ireland Updated" {
		t.Errorf("unexpected title after update: %q", title)
	}
}

func TestDeleteRecord_Success(t *testing.T) {
	t.Parallel()
	h, s := newTestHandler()
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !s.deleted["countries/ie"] {
		t.Error("record not marked as deleted in fake DB")
	}
}

func TestDeleteRecord_MissingKey(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestParseDBParam(t *testing.T) {
	t.Parallel()
	tests := []struct {
		query     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"db=owner/repo", "owner", "repo", false},
		{"db=", "", "", true},
		{"db=badformat", "", "", true},
		{"db=/repo", "", "", true},
		{"db=owner/", "", "", true},
		{"", "", "", true},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?"+tc.query, nil)
		owner, repo, err := parseDBParam(req)
		if (err != nil) != tc.wantErr {
			t.Errorf("query %q: wantErr=%v got err=%v", tc.query, tc.wantErr, err)
		}
		if err == nil && (owner != tc.wantOwner || repo != tc.wantRepo) {
			t.Errorf("query %q: got owner=%q repo=%q", tc.query, owner, repo)
		}
	}
}

func TestGithubToken(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if tok := githubToken(req); tok != "" {
		t.Errorf("expected empty token, got %q", tok)
	}
	req.Header.Set("Authorization", "Bearer mytoken")
	if tok := githubToken(req); tok != "mytoken" {
		t.Errorf("expected mytoken, got %q", tok)
	}
}

func TestGitHubLoginRedirect(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "github.com/login/oauth/authorize") {
		t.Fatalf("unexpected redirect: %s", location)
	}
}

func TestGitHubCallbackSetsCookie(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc&state=state123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state123"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	setCookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "ingitdb_github_token=") {
		t.Fatalf("expected auth cookie to be set, got %q", setCookie)
	}
	if !strings.Contains(w.Body.String(), "Successfully authenticated") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestGitHubStatusWithCookie(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/status", nil)
	req.AddCookie(&http.Cookie{Name: "ingitdb_github_token", Value: "oauth-token"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListCollections_RequiresAuth(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.requireAuth = true
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=owner/repo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestReadDefinitionFromGitHub_Success(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte(rootConfigYAML),
		"data/countries/.ingitdb-collection.yaml": []byte(countryColDefYAML),
	}}
	def, err := readDefinitionFromGitHub(context.Background(), fr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := def.Collections["countries"]; !ok {
		t.Error("expected 'countries' collection in definition")
	}
}

func TestReadDefinitionFromGitHub_MissingRoot(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{files: map[string][]byte{}}
	_, err := readDefinitionFromGitHub(context.Background(), fr)
	if err == nil {
		t.Fatal("expected error for missing root config")
	}
}

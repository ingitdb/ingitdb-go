// Package api implements the REST API server for api.ingitdb.com.
package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/julienschmidt/httprouter"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

//go:embed index.html
var indexHTML []byte

// Handler is the HTTP handler for the API server. Fields can be replaced in
// tests to inject mock implementations.
type Handler struct {
	newGitHubFileReader func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error)
	newGitHubDBWithDef  func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error)
	router              *httprouter.Router
}

// NewHandler creates a Handler with the default (production) GitHub implementations.
func NewHandler() *Handler {
	h := &Handler{
		newGitHubFileReader: dalgo2ghingitdb.NewGitHubFileReader,
		newGitHubDBWithDef:  dalgo2ghingitdb.NewGitHubDBWithDef,
	}
	h.router = h.buildRouter()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func (h *Handler) buildRouter() *httprouter.Router {
	r := httprouter.New()
	r.GET("/", h.serveIndex)
	r.GET("/ingitdb/v0/collections", h.listCollections)
	r.GET("/ingitdb/v0/record", h.readRecord)
	r.POST("/ingitdb/v0/record", h.createRecord)
	r.PUT("/ingitdb/v0/record", h.updateRecord)
	r.DELETE("/ingitdb/v0/record", h.deleteRecord)
	return r
}

// serveIndex serves the API index.html file.
func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexHTML)
}

// parseDBParam parses the "db" query parameter as "owner/repo".
func parseDBParam(r *http.Request) (owner, repo string, err error) {
	db := r.URL.Query().Get("db")
	if db == "" {
		return "", "", fmt.Errorf("missing required query parameter: db")
	}
	parts := strings.SplitN(db, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid db parameter %q: expected owner/repo", db)
	}
	return parts[0], parts[1], nil
}

// githubToken extracts a bearer token from the Authorization header.
func githubToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

// writeJSON writes v as a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an error response as JSON.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// readDefinitionFromGitHub reads the inGitDB definition from a GitHub repository.
func readDefinitionFromGitHub(ctx context.Context, fileReader dalgo2ghingitdb.FileReader) (*ingitdb.Definition, error) {
	rootConfigContent, found, err := fileReader.ReadFile(ctx, config.RootConfigFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", config.RootConfigFileName, err)
	}
	if !found {
		return nil, fmt.Errorf("file not found: %s", config.RootConfigFileName)
	}
	var rootConfig config.RootConfig
	if err = yaml.Unmarshal(rootConfigContent, &rootConfig); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", config.RootConfigFileName, err)
	}
	def := &ingitdb.Definition{Collections: make(map[string]*ingitdb.CollectionDef)}
	for id, colPath := range rootConfig.RootCollections {
		colDefPath := path.Join(colPath, ingitdb.CollectionDefFileName)
		colDefContent, colFound, readErr := fileReader.ReadFile(ctx, colDefPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read collection def %s: %w", colDefPath, readErr)
		}
		if !colFound {
			return nil, fmt.Errorf("collection definition not found: %s", colDefPath)
		}
		colDef := &ingitdb.CollectionDef{}
		if unmarshalErr := yaml.Unmarshal(colDefContent, colDef); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse collection def %s: %w", colDefPath, unmarshalErr)
		}
		colDef.ID = id
		colDef.DirPath = path.Clean(colPath)
		def.Collections[id] = colDef
	}
	return def, nil
}

// listCollections handles GET /ingitdb/v0/collections?db=owner/repo
func (h *Handler) listCollections(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: githubToken(r)}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	ids := make([]string, 0, len(def.Collections))
	for id := range def.Collections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	writeJSON(w, http.StatusOK, ids)
}

// readRecord handles GET /v0/record?db=owner/repo&key=col/record_id
func (h *Handler) readRecord(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: key")
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: githubToken(r)}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid key: %v", err))
		return
	}
	db, err := h.newGitHubDBWithDef(cfg, def)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open database: %v", err))
		return
	}
	dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
	data := map[string]any{}
	record := dal.NewRecordWithData(dalKey, data)
	if err = db.RunReadonlyTransaction(r.Context(), func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read record: %v", err))
		return
	}
	if !record.Exists() {
		writeError(w, http.StatusNotFound, fmt.Sprintf("record not found: %s", key))
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// createRecord handles POST /v0/record?db=owner/repo&key=col/record_id
func (h *Handler) createRecord(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: key")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %v", err))
		return
	}
	var data map[string]any
	if err = json.Unmarshal(body, &data); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: githubToken(r)}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid key: %v", err))
		return
	}
	db, err := h.newGitHubDBWithDef(cfg, def)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open database: %v", err))
		return
	}
	dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
	record := dal.NewRecordWithData(dalKey, data)
	if err = db.RunReadwriteTransaction(r.Context(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create record: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"key": key})
}

// updateRecord handles PUT /v0/record?db=owner/repo&key=col/record_id
func (h *Handler) updateRecord(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: key")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %v", err))
		return
	}
	var patch map[string]any
	if err = json.Unmarshal(body, &patch); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: githubToken(r)}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid key: %v", err))
		return
	}
	db, err := h.newGitHubDBWithDef(cfg, def)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open database: %v", err))
		return
	}
	dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
	if err = db.RunReadwriteTransaction(r.Context(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		data := map[string]any{}
		record := dal.NewRecordWithData(dalKey, data)
		if getErr := tx.Get(ctx, record); getErr != nil {
			return getErr
		}
		if !record.Exists() {
			return fmt.Errorf("record not found: %s", key)
		}
		maps.Copy(data, patch)
		return tx.Set(ctx, record)
	}); err != nil {
		if strings.Contains(err.Error(), "record not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update record: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key})
}

// deleteRecord handles DELETE /v0/record?db=owner/repo&key=col/record_id
func (h *Handler) deleteRecord(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: key")
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: githubToken(r)}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid key: %v", err))
		return
	}
	db, err := h.newGitHubDBWithDef(cfg, def)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open database: %v", err))
		return
	}
	dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
	if err = db.RunReadwriteTransaction(r.Context(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, dalKey)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete record: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key})
}

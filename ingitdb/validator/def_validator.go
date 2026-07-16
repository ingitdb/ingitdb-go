package validator

// specscore: feature/cli/validate
// specscore: feature/column-validation

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/config"
	"gopkg.in/yaml.v3"
)

// decodeCollectionDef parses a collection definition, rejecting any key the
// schema does not model.
//
// yaml.Unmarshal ignores unknown keys, which is what let a plausible-looking
// `enum:` or `one_of:` appear enforced while doing nothing. It is not
// hypothetical: geo-ingitdb declared an `inherits:` hierarchy across four
// files, plus min_records_count/max_records_count and record_labels, none of
// which inGitDB has ever implemented — the keys were read and dropped, so the
// config looked live and did nothing.
//
// KnownFields(true) is already the house pattern (config/root_config.go,
// validator/subscribers_validator.go); this path had simply diverged.
func decodeCollectionDef(fileContent []byte, colDef *ingitdb.CollectionDef) error {
	dec := yaml.NewDecoder(bytes.NewReader(fileContent))
	dec.KnownFields(true)
	return dec.Decode(colDef)
}

// resolveInheritance overlays the base definition referenced by colDef.Inherits
// (transitively) onto colDef in place, then clears the field. defFilePath is the
// path of colDef's own definition file; the `inherits` value is resolved
// relative to its directory. A missing base or an inheritance cycle is a
// load-time error — the point of the feature is that `inherits` is no longer
// silently discarded. Resolution runs before DirPath, subcollections, and views
// are derived, so every downstream reader sees the fully-merged definition. See
// spec/features/definition-inheritance.
func (dl defLoader) resolveInheritance(colDef *ingitdb.CollectionDef, defFilePath string) error {
	if colDef.Inherits == "" {
		return nil
	}
	// seen holds the absolute path of every file already visited on this chain,
	// so a base that (transitively) re-references an ancestor is a load error
	// rather than unbounded recursion. The inheriting file counts as visited.
	seen := map[string]bool{absInheritPath(defFilePath): true}
	base, err := dl.loadInheritedBase(defFilePath, colDef.Inherits, seen)
	if err != nil {
		return err
	}
	overlayCollectionDef(colDef, base)
	colDef.Inherits = ""
	return nil
}

// loadInheritedBase reads and decodes the base partial referenced by `inherits`
// relative to fromFile's directory. When that base itself declares `inherits`,
// the rest of the chain is resolved and overlaid first (a nearer base wins over
// a farther one), so the returned CollectionDef is the fully-resolved base.
func (dl defLoader) loadInheritedBase(fromFile, inherits string, seen map[string]bool) (*ingitdb.CollectionDef, error) {
	basePath := filepath.Join(filepath.Dir(fromFile), inherits)
	baseAbs := absInheritPath(basePath)
	if seen[baseAbs] {
		return nil, fmt.Errorf("inheritance cycle detected: %s re-inherits %s", fromFile, basePath)
	}
	seen[baseAbs] = true

	content, err := dl.readFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read inherited definition %q referenced by %s: %w", basePath, fromFile, err)
	}
	base := new(ingitdb.CollectionDef)
	if err = decodeCollectionDef(content, base); err != nil {
		return nil, fmt.Errorf("failed to parse inherited definition %s: %w", basePath, err)
	}
	if base.Inherits != "" {
		parent, parentErr := dl.loadInheritedBase(basePath, base.Inherits, seen)
		if parentErr != nil {
			return nil, parentErr
		}
		overlayCollectionDef(base, parent)
		base.Inherits = ""
	}
	return base, nil
}

// absInheritPath returns the absolute form of p for cycle-detection keys,
// falling back to p unchanged when the working directory cannot be resolved.
func absInheritPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// overlayCollectionDef fills fields child leaves unset from base and merges
// columns and titles by key, with child winning. Structural fields populated
// from the filesystem (ID, DirPath, SubCollections, Views) are never taken from
// base. See spec/features/definition-inheritance REQ column-merge-child-wins and
// REQ scalar-and-map-field-inheritance.
func overlayCollectionDef(child, base *ingitdb.CollectionDef) {
	// Columns merge by name: base contributes any column child does not declare;
	// a column child declares wholly replaces base's column of the same name.
	if len(base.Columns) > 0 {
		if child.Columns == nil {
			child.Columns = make(map[string]*ingitdb.ColumnDef, len(base.Columns))
		}
		for name, col := range base.Columns {
			if _, ok := child.Columns[name]; !ok {
				child.Columns[name] = col
			}
		}
	}
	// Titles merge per locale: child wins for any locale it declares, base fills
	// the rest.
	if len(base.Titles) > 0 {
		if child.Titles == nil {
			child.Titles = make(map[string]string, len(base.Titles))
		}
		for locale, title := range base.Titles {
			if _, ok := child.Titles[locale]; !ok {
				child.Titles[locale] = title
			}
		}
	}
	if child.RecordFile == nil {
		child.RecordFile = base.RecordFile
	}
	if child.DataDir == "" {
		child.DataDir = base.DataDir
	}
	if len(child.ColumnsOrder) == 0 {
		child.ColumnsOrder = base.ColumnsOrder
	}
	if len(child.PrimaryKey) == 0 {
		child.PrimaryKey = base.PrimaryKey
	}
	if child.DefaultView == nil {
		child.DefaultView = base.DefaultView
	}
	if child.Readme == nil {
		child.Readme = base.Readme
	}
	if child.ConflictResolution == nil {
		child.ConflictResolution = base.ConflictResolution
	}
}

// definitionReader wraps ReadDefinition to satisfy ingitdb.CollectionsReader.
type definitionReader struct{}

// NewCollectionsReader returns an ingitdb.CollectionsReader backed by ReadDefinition.
func NewCollectionsReader() ingitdb.CollectionsReader { return definitionReader{} }

func (definitionReader) ReadDefinition(dbPath string, opts ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
	return ReadDefinition(dbPath, opts...)
}

// defLoader holds the I/O primitives used when reading collection definitions.
// Struct fields allow test code to inject fakes without changing production behaviour.
type defLoader struct {
	readFile func(string) ([]byte, error)
	readDir  func(string) ([]os.DirEntry, error)
}

// newDefLoader returns a defLoader that delegates directly to the OS.
func newDefLoader() defLoader {
	return defLoader{readFile: os.ReadFile, readDir: os.ReadDir}
}

func ReadDefinition(rootPath string, o ...ingitdb.ReadOption) (def *ingitdb.Definition, err error) {
	opts := ingitdb.NewReadOptions(o...)
	var rootConfig config.RootConfig
	rootConfig, err = config.ReadRootConfigFromFile(rootPath, opts)
	if err != nil {
		err = fmt.Errorf("failed to read root config from %s: %v", config.IngitDBDirName, err)
		return
	}
	dl := newDefLoader()
	def, err = dl.readRootCollections(rootPath, rootConfig, opts)
	if err != nil {
		return nil, err
	}
	def.Subscribers, err = ReadSubscribers(rootPath, opts)
	if err != nil {
		return nil, err
	}
	// A database that resolves to zero collections but carries an older-layout
	// marker is not empty — it is unreadable, and returning it with no error is
	// silent success on a broken read (ingitdb-go#9). geo-ingitdb hits exactly
	// this: a single `.ingitdb.yaml` plus `.ingitdb-collection.yaml` /
	// `.ingitdb-subcol.*`, none of which the current reader understands.
	if len(def.Collections) == 0 {
		if marker := detectLegacyLayout(dl, rootPath); marker != "" {
			return nil, fmt.Errorf(
				"%s carries %s, a marker of an unsupported older inGitDB layout, and resolved to zero collections; the reader expects %s/%s and .collection/definition.yaml (see ingitdb-go#9)",
				rootPath, marker, config.IngitDBDirName, config.RootCollectionsFileName)
		}
	}
	// foreign_key targets resolve module-relative to the declaring collection
	// (ingitdb.ResolveForeignKey), so `foreign_key: countries` in
	// commerce.addresses reaches commerce.countries without hard-coding the
	// mount. A target that resolves to no collection is a load-time error.
	if opts.IsValidationRequired() {
		if err = ingitdb.ValidateForeignKeys(def); err != nil {
			return nil, err
		}
	}
	return def, nil
}

// legacyLayoutMarkers are root-level files that mean "this is an inGitDB
// database written in the older layout": a single `.ingitdb.yaml` config (the
// current reader wants a `.ingitdb/` directory) and a bare
// `.ingitdb-collection.yaml` (the current reader wants `.collection/`).
var legacyLayoutMarkers = []string{".ingitdb.yaml", ".ingitdb-collection.yaml"}

// detectLegacyLayout reports the first legacy-layout marker present at rootPath,
// or "" if none. Used only to turn a zero-collection result into a loud error
// rather than silent success, so it is deliberately conservative: it checks a
// couple of unambiguous root-level filenames rather than scanning the tree, so
// it never flags a directory the caller merely pointed at by mistake.
func detectLegacyLayout(dl defLoader, rootPath string) string {
	for _, name := range legacyLayoutMarkers {
		if _, err := dl.readFile(filepath.Join(rootPath, name)); err == nil {
			return name
		}
	}
	return ""
}

func (dl defLoader) readRootCollections(rootPath string, rootConfig config.RootConfig, o ingitdb.ReadOptions) (def *ingitdb.Definition, err error) {
	def = new(ingitdb.Definition)
	def.Collections = make(map[string]*ingitdb.CollectionDef)
	for id, colPath := range rootConfig.RootCollections {
		if strings.Contains(colPath, "*") {
			err = fmt.Errorf("wildcard root collection paths are not supported, ID=%s, path=%s", id, colPath)
			return
		}
		var colDef *ingitdb.CollectionDef
		if colDef, err = dl.readCollectionDef(rootPath, colPath, "", id, nil, o); err != nil {
			err = fmt.Errorf("failed to validate root collection def ID=%s: %w", id, err)
			return
		}
		def.Collections[id] = colDef
	}
	return
}

func (dl defLoader) readCollectionDef(rootPath, relPath, parentPath, id string, subPath []string, o ingitdb.ReadOptions) (colDef *ingitdb.CollectionDef, err error) {
	colDir := filepath.Join(rootPath, relPath)
	schemaDir := filepath.Join(colDir, ingitdb.SchemaDir)

	var fileContent []byte
	isNewLayout := false

	if len(subPath) > 0 {
		// Old-layout subcollection: navigate via subPath.
		for _, p := range subPath {
			schemaDir = filepath.Join(schemaDir, "subcollections", p)
		}
		colDefFilePath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
		fileContent, err = dl.readFile(colDefFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", colDefFilePath, err)
		}
	} else {
		// Detect layout by trying both definition file locations.
		oldDefPath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
		newDefPath := filepath.Join(colDir, ingitdb.CollectionDefFileName)
		oldContent, oldErr := dl.readFile(oldDefPath)
		newContent, newErr := dl.readFile(newDefPath)
		oldExists := oldErr == nil
		newExists := newErr == nil
		switch {
		case oldExists && newExists:
			return nil, fmt.Errorf("collection %q: both %s and %s exist; use only one layout", id, oldDefPath, newDefPath)
		case oldExists:
			fileContent = oldContent
		case newExists:
			fileContent = newContent
			schemaDir = colDir
			isNewLayout = true
		default:
			if !os.IsNotExist(oldErr) {
				return nil, fmt.Errorf("failed to read file %s: %w", oldDefPath, oldErr)
			}
			return nil, fmt.Errorf("failed to read file %s: %w", newDefPath, newErr)
		}
	}

	colDef = new(ingitdb.CollectionDef)
	colDefFilePath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
	if err = decodeCollectionDef(fileContent, colDef); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", colDefFilePath, err)
	}
	colDef.ID = id

	// Resolve `inherits` before DirPath/data_dir, subcollections, and views are
	// derived, so the merge is layout-agnostic and every downstream reader sees
	// the fully-merged definition. A missing base or a cycle is a load error.
	if err = dl.resolveInheritance(colDef, colDefFilePath); err != nil {
		return nil, err
	}

	var dataBase string
	if isNewLayout {
		dataBase = colDir
		if filepath.Base(filepath.Dir(colDir)) == ingitdb.CollectionsDir {
			dataBase = filepath.Dir(filepath.Dir(colDir))
		}
		if colDef.DataDir != "" {
			colDef.DirPath = filepath.Join(dataBase, colDef.DataDir)
		} else {
			colDef.DirPath = dataBase
		}
	} else if len(subPath) == 0 {
		colDef.DirPath = colDir
	} else {
		colDef.DirPath = schemaDir
	}

	fullPath := id
	if parentPath != "" {
		fullPath = parentPath + "/" + id
	}

	if o.IsValidationRequired() {
		if err = colDef.Validate(); err != nil {
			if len(subPath) > 0 {
				return nil, fmt.Errorf("not valid definition of subcollection '%s': %w", fullPath, err)
			}
			return nil, fmt.Errorf("not valid definition of collection '%s': %w", fullPath, err)
		}
		if len(subPath) > 0 {
			log.Printf("Definition of subcollection '%s' is valid", fullPath)
		} else {
			log.Printf("Definition of collection '%s' is valid", fullPath)
		}
	}

	if isNewLayout {
		if colDef.SubCollections, err = dl.loadSubCollectionsShared(schemaDir, dataBase, fullPath, o); err != nil {
			return nil, fmt.Errorf("failed to load subcollections for '%s': %w", id, err)
		}
		viewsDir := filepath.Join(schemaDir, ingitdb.SharedViewsDir)
		if colDef.Views, err = dl.loadViews(viewsDir, o); err != nil {
			return nil, fmt.Errorf("failed to load views for '%s': %w", id, err)
		}
	} else {
		if colDef.SubCollections, err = dl.loadSubCollections(rootPath, relPath, subPath, fullPath, o); err != nil {
			return nil, fmt.Errorf("failed to load subcollections for '%s': %w", id, err)
		}
		viewsDir := filepath.Join(schemaDir, "views")
		if colDef.Views, err = dl.loadViews(viewsDir, o); err != nil {
			return nil, fmt.Errorf("failed to load views for '%s': %w", id, err)
		}
	}

	if colDef.DefaultView != nil {
		colDef.DefaultView.ID = ingitdb.DefaultViewID
		colDef.DefaultView.IsDefault = true
		if colDef.Views == nil {
			colDef.Views = make(map[string]*ingitdb.ViewDef)
		}
		colDef.Views[ingitdb.DefaultViewID] = colDef.DefaultView
	}

	return
}

// readCollectionDefShared reads a single collection from the new shared-directory
// layout. schemaDir is the absolute path to the .collections/{name}/ directory.
// dataBaseDir is the parent of .collections/ (the shared data root).
func (dl defLoader) readCollectionDefShared(schemaDir, dataBaseDir, parentPath, id string, o ingitdb.ReadOptions) (*ingitdb.CollectionDef, error) {
	colDefFilePath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
	fileContent, err := dl.readFile(colDefFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", colDefFilePath, err)
	}

	colDef := new(ingitdb.CollectionDef)
	if err = decodeCollectionDef(fileContent, colDef); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", colDefFilePath, err)
	}
	colDef.ID = id

	// Resolve `inherits` before data_dir, subcollections, and views are derived
	// (see readCollectionDef). A missing base or a cycle is a load error.
	if err = dl.resolveInheritance(colDef, colDefFilePath); err != nil {
		return nil, err
	}

	if colDef.DataDir != "" {
		colDef.DirPath = filepath.Join(dataBaseDir, colDef.DataDir)
	} else {
		colDef.DirPath = dataBaseDir
	}

	fullPath := id
	if parentPath != "" {
		fullPath = parentPath + "/" + id
	}

	if o.IsValidationRequired() {
		if err = colDef.Validate(); err != nil {
			return nil, fmt.Errorf("not valid definition of collection '%s': %w", fullPath, err)
		}
		log.Printf("Definition of collection '%s' is valid", fullPath)
	}

	colDef.SubCollections, err = dl.loadSubCollectionsShared(schemaDir, dataBaseDir, fullPath, o)
	if err != nil {
		return nil, fmt.Errorf("failed to load subcollections for '%s': %w", id, err)
	}

	viewsDir := filepath.Join(schemaDir, ingitdb.SharedViewsDir)
	colDef.Views, err = dl.loadViews(viewsDir, o)
	if err != nil {
		return nil, fmt.Errorf("failed to load views for '%s': %w", id, err)
	}

	if colDef.DefaultView != nil {
		colDef.DefaultView.ID = ingitdb.DefaultViewID
		colDef.DefaultView.IsDefault = true
		if colDef.Views == nil {
			colDef.Views = make(map[string]*ingitdb.ViewDef)
		}
		colDef.Views[ingitdb.DefaultViewID] = colDef.DefaultView
	}

	return colDef, nil
}

// loadSubCollectionsShared discovers subcollections in the new shared-directory
// layout. Each non-$-prefixed sub-directory of schemaDir that contains a
// definition.yaml is treated as a subcollection.
func (dl defLoader) loadSubCollectionsShared(schemaDir, dataBaseDir, parentPath string, o ingitdb.ReadOptions) (map[string]*ingitdb.CollectionDef, error) {
	entries, err := dl.readDir(schemaDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read schema directory: %w", err)
	}

	var subCollections map[string]*ingitdb.CollectionDef
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), "$") {
			continue
		}
		subID := entry.Name()
		subSchemaDir := filepath.Join(schemaDir, subID)

		colDef, subErr := dl.readCollectionDefShared(subSchemaDir, dataBaseDir, parentPath, subID, o)
		if subErr != nil {
			if errors.Is(subErr, os.ErrNotExist) {
				continue // no definition.yaml — not a collection, skip
			}
			return nil, subErr
		}

		if subCollections == nil {
			subCollections = make(map[string]*ingitdb.CollectionDef)
		}
		subCollections[subID] = colDef
	}
	return subCollections, nil
}

func (dl defLoader) loadSubCollections(rootPath, relPath string, subPath []string, parentPath string, o ingitdb.ReadOptions) (map[string]*ingitdb.CollectionDef, error) {
	schemaDir := filepath.Join(rootPath, relPath, ingitdb.SchemaDir)
	if len(subPath) > 0 {
		for _, p := range subPath {
			schemaDir = filepath.Join(schemaDir, "subcollections", p)
		}
	}
	subCollectionsPath := filepath.Join(schemaDir, "subcollections")

	entries, err := dl.readDir(subCollectionsPath)
	if os.IsNotExist(err) {
		return nil, nil // No subcollections
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read subcollections directory: %w", err)
	}

	var subCollections map[string]*ingitdb.CollectionDef

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		childSubPath := append(append([]string(nil), subPath...), id)

		colDef, err := dl.readCollectionDef(rootPath, relPath, parentPath, id, childSubPath, o)
		if err != nil {
			return nil, err
		}

		if subCollections == nil {
			subCollections = make(map[string]*ingitdb.CollectionDef)
		}
		subCollections[id] = colDef
	}
	return subCollections, nil
}

func (dl defLoader) loadViews(viewsDir string, o ingitdb.ReadOptions) (map[string]*ingitdb.ViewDef, error) {
	entries, err := dl.readDir(viewsDir)
	if os.IsNotExist(err) {
		return nil, nil // No views
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read views directory: %w", err)
	}

	var views map[string]*ingitdb.ViewDef

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".yaml")
		viewDefFilePath := filepath.Join(viewsDir, entry.Name())

		fileContent, err := dl.readFile(viewDefFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", viewDefFilePath, err)
		}

		viewDef := new(ingitdb.ViewDef)
		if err = yaml.Unmarshal(fileContent, viewDef); err != nil {
			return nil, fmt.Errorf("failed to parse YAML file %s: %w", viewDefFilePath, err)
		}
		viewDef.ID = id

		if o.IsValidationRequired() {
			if err = viewDef.Validate(); err != nil {
				return nil, fmt.Errorf("not valid definition of view '%s': %w", id, err)
			}
			log.Printf("Definition of view '%s' is valid", id)
		}

		if views == nil {
			views = make(map[string]*ingitdb.ViewDef)
		}
		views[id] = viewDef
	}
	return views, nil
}

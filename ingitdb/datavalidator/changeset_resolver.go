package datavalidator

// specscore: feature/cli/validate

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// NewChangeSetResolver returns the default ChangeSetResolver, which maps changed
// files to the collection records they belong to. It assumes the database
// directory is the git repository root (changed-file paths are joined onto it).
func NewChangeSetResolver() ChangeSetResolver {
	return changeSetResolver{}
}

type changeSetResolver struct{}

// Resolve maps each changed file to the collection record it affects, in a
// root collection or in a subcollection instance (CollectionID is then the
// full path, e.g. "orders/order_details"). Deleted files are skipped (there
// is nothing to open; orphaned-reference checks are out of scope). For
// single-record layouts the affected record key is derived from the file
// name; for map/list layouts the whole shared file is marked affected
// (RecordKey == "").
func (r changeSetResolver) Resolve(dbPath string, def *ingitdb.Definition, changedFiles []ingitdb.ChangedFile) ([]AffectedRecord, error) {
	return r.resolveWith(newPathResolver(def), dbPath, changedFiles), nil
}

// resolveWith is Resolve with the caller's pathResolver, so a validation
// call that resolves further paths reads each parent collection once.
func (changeSetResolver) resolveWith(paths *pathResolver, dbPath string, changedFiles []ingitdb.ChangedFile) []AffectedRecord {
	var affected []AffectedRecord
	for _, cf := range changedFiles {
		if cf.Kind == ingitdb.ChangeKindDeleted {
			continue
		}
		absPath := filepath.Clean(filepath.Join(dbPath, cf.Path))
		levels := paths.recordFile(absPath)
		if levels == nil {
			continue
		}
		owner := levels[len(levels)-1]
		key := ""
		if owner.colDef.RecordFile.RecordType == ingitdb.SingleRecord {
			key = recordKeyFromCollectionFilePath(owner.colDef, absPath)
		}
		affected = append(affected, AffectedRecord{CollectionID: owner.id, FilePath: absPath, RecordKey: key, ChangeKind: cf.Kind, levels: levels})
	}
	return affected
}

// instanceLevel is one collection instance on the way from a root
// collection to the instance that owns a record file.
type instanceLevel struct {
	id     string                 // full id, e.g. "orders/order_details"
	colDef *ingitdb.CollectionDef // DirPath repointed at the instance's data directory
	// parentKey and nameErr are the decoded parent directory name of a
	// subcollection instance, and the error when the parent's key_encoding
	// never writes that name.
	parentKey string
	nameErr   error
}

// pathResolver maps paths to collection instances for one validation call.
// It remembers, per parent collection instance, which directory names of
// its records directory belong to parent records, which it reads only for a
// name the parent-directory rule rejects (see parentWalked). readDir and
// load are os.ReadDir and loadSingleRecordFile; tests count calls through
// them.
type pathResolver struct {
	def     *ingitdb.Definition
	parents map[string]*parentRecords // collection id + "\x00" + instance data directory → its parent records
	readDir func(string) ([]os.DirEntry, error)
	load    func(*ingitdb.CollectionDef, string) (loadedRecord, bool)
}

// parentRecords are the parent records of one collection instance, by the
// directory name that holds their subcollection instances. For a
// single-record layout with {key} the names come from a listing (files) and
// only the files of a name asked for are parsed; other layouts keep their
// records in one file, which is parsed whole.
type parentRecords struct {
	files  map[string][]string // directory name → record files named so, not yet parsed
	walked map[string]bool     // directory name → whether a parent record has it
}

func newPathResolver(def *ingitdb.Definition) *pathResolver {
	return &pathResolver{def: def, parents: map[string]*parentRecords{}, readDir: os.ReadDir, load: loadSingleRecordFile}
}

// recordFile returns the collection instances from a root collection down
// to the one that owns absPath as a record file, or nil.
func (r *pathResolver) recordFile(absPath string) []instanceLevel {
	levels, isRecord := r.resolve(absPath)
	if !isRecord {
		return nil
	}
	return levels
}

// resolve returns the collection instances from a root collection down to
// the deepest one whose records directory holds absPath, and whether absPath
// is a record file of that instance; nil when no records directory holds it.
// Below a collection's records directory, <parent>/<subcollection id>/...
// leads into that subcollection's instance for the parent only where the
// full pass walks one (parentWalked). Root collections are tried in id
// order; the owner of a record file wins over a collection whose records
// directory merely holds it.
func (r *pathResolver) resolve(absPath string) ([]instanceLevel, bool) {
	var holder []instanceLevel
	for _, id := range slices.Sorted(maps.Keys(r.def.Collections)) {
		levels, isRecord := r.below([]instanceLevel{{id: id, colDef: r.def.Collections[id]}}, absPath)
		if isRecord {
			return levels, true
		}
		if holder == nil {
			holder = levels
		}
	}
	return holder, false
}

func (r *pathResolver) below(levels []instanceLevel, absPath string) ([]instanceLevel, bool) {
	current := levels[len(levels)-1]
	if current.colDef == nil {
		return nil, false
	}
	if ownsRecordFile(current.colDef, absPath) {
		return levels, true
	}
	rel, err := filepath.Rel(recordsBaseDir(current.colDef), absPath)
	segments := strings.Split(rel, string(filepath.Separator))
	if err != nil || rel == "." || segments[0] == ".." {
		return nil, false
	}
	if sub := current.colDef.SubCollections[segmentAt(segments, 1)]; sub != nil && len(segments) >= 3 && r.parentWalked(current, segments[0]) {
		inst := *sub // shallow copy: repoint DirPath without mutating the shared definition
		inst.DirPath = subCollectionDataDir(current.colDef, segments[0], segments[1])
		parentKey, nameErr := decodeParentName(current.colDef, segments[0])
		deeper, isRecord := r.below(append(slices.Clone(levels), instanceLevel{
			id: current.id + "/" + segments[1], colDef: &inst, parentKey: parentKey, nameErr: nameErr,
		}), absPath)
		if deeper != nil {
			return deeper, isRecord
		}
	}
	return levels, false
}

func segmentAt(segments []string, i int) string {
	if i < len(segments) {
		return segments[i]
	}
	return ""
}

// parentWalked reports whether the full pass walks the subcollection
// instances below the directory name in the records directory of the
// collection instance at level: a name the parent-directory rule accepts
// (ingitdb.IsParentDirName), or the directory name of one of its parent
// records, read as the full pass's walk reads them (a record file that
// loads, parentDirName). The parents are looked up once per collection and
// instance directory: collections sharing a data directory each have their
// own.
func (r *pathResolver) parentWalked(level instanceLevel, name string) bool {
	colDef := level.colDef
	if ingitdb.IsParentDirName(name, colDef.RecordFile) {
		return true
	}
	cacheKey := level.id + "\x00" + colDef.DirPath
	parents := r.parents[cacheKey]
	if parents == nil {
		parents = r.readParents(colDef)
		r.parents[cacheKey] = parents
	}
	walked, known := parents.walked[name]
	if !known {
		for _, filePath := range parents.files[name] {
			if _, ok := r.load(colDef, filePath); ok {
				walked = true
				break
			}
		}
		parents.walked[name] = walked
	}
	return walked
}

// readParents lists the parent records of a collection instance: their
// record files by directory name for a single-record layout with {key},
// otherwise every record parsed from the shared file.
func (r *pathResolver) readParents(colDef *ingitdb.CollectionDef) *parentRecords {
	parents := &parentRecords{files: map[string][]string{}, walked: map[string]bool{}}
	rf := colDef.RecordFile
	if !shouldSkipRecordParsing(colDef) && rf.RecordType == ingitdb.SingleRecord && strings.Contains(rf.Name, "{key}") {
		files, _, _ := recordFilesIn(colDef, r.readDir)
		for _, filePath := range files {
			name := recordNameFromCollectionFilePath(colDef, filePath)
			parents.files[name] = append(parents.files[name], filePath)
		}
		return parents
	}
	records, _ := loadCollectionRecords(colDef)
	for _, pr := range records {
		if dirName, ok := parentDirName(colDef, pr); ok {
			parents.walked[dirName] = true
		}
	}
	return parents
}

// CollectionForRecordFile returns the root collection (and its ID) that owns
// absPath as a record file, or ("", nil) when no root collection's
// record-file layout matches. It mirrors how the full validator enumerates
// record files, so the incremental validator, change-set resolver, and `diff`
// all agree on which files are records.
func CollectionForRecordFile(def *ingitdb.Definition, absPath string) (string, *ingitdb.CollectionDef) {
	for id, colDef := range def.Collections {
		if ownsRecordFile(colDef, absPath) {
			return id, colDef
		}
	}
	return "", nil
}

// ownsRecordFile reports whether absPath is a record file of the collection
// instance colDef.
func ownsRecordFile(colDef *ingitdb.CollectionDef, absPath string) bool {
	if shouldSkipRecordParsing(colDef) {
		return false
	}
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		return isRecordFilePath(colDef, absPath)
	case ingitdb.MapOfRecords, ingitdb.ListOfRecords:
		return filepath.Clean(collectionRecordFilePath(colDef)) == absPath
	}
	return false
}

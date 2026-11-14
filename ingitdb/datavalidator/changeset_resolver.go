package datavalidator

// specscore: feature/cli/validate

import (
	"maps"
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
func (changeSetResolver) Resolve(dbPath string, def *ingitdb.Definition, changedFiles []ingitdb.ChangedFile) ([]AffectedRecord, error) {
	var affected []AffectedRecord
	paths := newPathResolver(def)
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
	return affected, nil
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
// It remembers, per parent collection instance, the directory names of its
// parent records, which it reads only for a name the parent-directory rule
// rejects (see parentWalked).
type pathResolver struct {
	def         *ingitdb.Definition
	parentNames map[string]map[string]bool // parent instance data directory → directory names of its records
}

func newPathResolver(def *ingitdb.Definition) *pathResolver {
	return &pathResolver{def: def, parentNames: map[string]map[string]bool{}}
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
	if sub := current.colDef.SubCollections[segmentAt(segments, 1)]; sub != nil && len(segments) >= 3 && r.parentWalked(current.colDef, segments[0]) {
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
// instances below the directory name in a collection's records directory:
// a name the parent-directory rule accepts (ingitdb.IsParentDirName), or the
// directory name of one of its parent records, read exactly as the full
// pass's walk reads them (loadCollectionRecords, parentDirName), once per
// parent instance.
func (r *pathResolver) parentWalked(colDef *ingitdb.CollectionDef, name string) bool {
	if ingitdb.IsParentDirName(name, colDef.RecordFile) {
		return true
	}
	names, ok := r.parentNames[colDef.DirPath]
	if !ok {
		names = map[string]bool{}
		records, _ := loadCollectionRecords(colDef)
		for _, pr := range records {
			if dirName, ok := parentDirName(colDef, pr); ok {
				names[dirName] = true
			}
		}
		r.parentNames[colDef.DirPath] = names
	}
	return names[name]
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

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
	for _, cf := range changedFiles {
		if cf.Kind == ingitdb.ChangeKindDeleted {
			continue
		}
		absPath := filepath.Clean(filepath.Join(dbPath, cf.Path))
		levels := resolveRecordFile(def, absPath)
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

// resolveRecordFile returns the collection instances from a root collection
// down to the one that owns absPath as a record file, or nil. Below a
// collection's records directory, <parent>/<subcollection id>/... leads into
// that subcollection's instance for the parent, as the full pass walks it.
// Root collections are tried in id order.
func resolveRecordFile(def *ingitdb.Definition, absPath string) []instanceLevel {
	for _, id := range slices.Sorted(maps.Keys(def.Collections)) {
		if levels := resolveBelow([]instanceLevel{{id: id, colDef: def.Collections[id]}}, absPath); levels != nil {
			return levels
		}
	}
	return nil
}

func resolveBelow(levels []instanceLevel, absPath string) []instanceLevel {
	current := levels[len(levels)-1]
	if current.colDef == nil {
		return nil
	}
	if ownsRecordFile(current.colDef, absPath) {
		return levels
	}
	rel, err := filepath.Rel(recordsBaseDir(current.colDef), absPath)
	segments := strings.Split(rel, string(filepath.Separator))
	if err != nil || len(segments) < 3 || segments[0] == ".." {
		return nil
	}
	sub := current.colDef.SubCollections[segments[1]]
	if sub == nil {
		return nil
	}
	inst := *sub // shallow copy: repoint DirPath without mutating the shared definition
	inst.DirPath = subCollectionDataDir(current.colDef, segments[0], segments[1])
	parentKey, nameErr := decodeParentName(current.colDef, segments[0])
	return resolveBelow(append(slices.Clone(levels), instanceLevel{
		id: current.id + "/" + segments[1], colDef: &inst, parentKey: parentKey, nameErr: nameErr,
	}), absPath)
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
		pattern := singleRecordGlobPattern(colDef)
		matched, matchErr := filepath.Match(filepath.Clean(pattern), absPath)
		return matchErr == nil && matched && !skipRecordPath(absPath, colDef.RecordFile)
	case ingitdb.MapOfRecords, ingitdb.ListOfRecords:
		return filepath.Clean(collectionRecordFilePath(colDef)) == absPath
	}
	return false
}

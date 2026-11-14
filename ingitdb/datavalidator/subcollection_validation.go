package datavalidator

// specscore: feature/subcollection-record-validation

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// subCollectionInstance is one on-disk materialization of a declared
// subcollection: the subcollection definition repointed at the concrete data
// directory that holds a single parent record's instance of it. A subcollection
// has no single data directory — its records live once per parent record — so
// its loaded DirPath (the schema directory, or the shared data root) never
// points at data. Every reader of subcollection records must resolve the data
// directory per parent record; this type carries that resolution.
type subCollectionInstance struct {
	fullID    string                 // schema full path, e.g. "orders/order_details"
	colDef    *ingitdb.CollectionDef // a shallow copy of the sub def, DirPath repointed at the instance data dir
	parentKey string                 // key of the parent record that owns this instance
	// ancestorKeys are the keys from the root record down to and including
	// parentKey: the instance at spaces/k3v9q/ext/calendarius/days has
	// ["k3v9q", "calendarius"]. A path foreign_key's $same segments read them.
	ancestorKeys []string
}

// subCollectionDataDir returns the effective data directory for one parent
// record's instance of a subcollection, per the storage convention:
//
//	<parent DirPath>/<parent records-base-path>/<parentKey>/<subID>/
//
// The parent's records-base-path is "$records" when its record_file.name
// contains "{key}" and empty otherwise (RecordFileDef.RecordsBasePath), so the
// per-record directory is a sibling of the parent's record files — the same
// place a per-key subdirectory naturally lives (e.g.
// orders/$records/ord001/order_details next to orders/$records/ord001.yaml).
func subCollectionDataDir(parentColDef *ingitdb.CollectionDef, parentKey, subID string) string {
	base := parentColDef.DirPath
	if parentColDef.RecordFile != nil {
		base = filepath.Join(base, parentColDef.RecordFile.RecordsBasePath())
	}
	return filepath.Join(base, parentKey, subID)
}

// walkSubCollectionInstances invokes fn for every subcollection instance
// reachable from colDef, recursively, to arbitrary depth. colDef is a collection
// whose DirPath already points at its data directory — a root collection, or a
// subcollection instance already repointed by an enclosing level. For each level
// the parent records are read once (with loadCollectionRecords, the same reader
// the schema and foreign-key passes use), and every declared subcollection is
// expanded into one instance per parent key. Subcollection ids and parent keys
// are visited in sorted order so findings are deterministic.
//
// A parent key is either the key of a parent record, or the name of a
// directory that holds data for the subcollection although no parent record
// file exists (parentKeysWithSubCollectionDir). The second case mirrors
// Firestore, where a document may own subcollections without existing itself
// (`ext/commitius/cmSessions/{id}` with no `ext/commitius` document); without it
// such records were silently never validated.
//
// A read/parse failure at a level is not reported here (the schema pass reports
// it for that collection); the level then yields only the directory-discovered
// instances.
func walkSubCollectionInstances(fullID string, colDef *ingitdb.CollectionDef, fn func(subCollectionInstance)) {
	walkSubCollectionInstancesBelow(fullID, colDef, nil, fn)
}

// walkSubCollectionInstancesBelow is walkSubCollectionInstances for a colDef
// whose own records sit below the given ancestor keys.
func walkSubCollectionInstancesBelow(fullID string, colDef *ingitdb.CollectionDef, ancestorKeys []string, fn func(subCollectionInstance)) {
	if colDef == nil || len(colDef.SubCollections) == 0 {
		return
	}
	parents, _ := loadCollectionRecords(colDef)
	recordKeys := make([]string, 0, len(parents))
	for _, pr := range parents {
		recordKeys = append(recordKeys, pr.Key)
	}

	for _, subID := range slices.Sorted(maps.Keys(colDef.SubCollections)) {
		sub := colDef.SubCollections[subID]
		subFullID := fullID + "/" + subID
		parentKeys := append(slices.Clone(recordKeys), parentKeysWithSubCollectionDir(colDef, subID)...)
		slices.Sort(parentKeys)
		parentKeys = slices.Compact(parentKeys)
		for _, parentKey := range parentKeys {
			inst := *sub // shallow copy: repoint DirPath without mutating the shared definition
			inst.DirPath = subCollectionDataDir(colDef, parentKey, subID)
			keys := append(slices.Clone(ancestorKeys), parentKey)
			fn(subCollectionInstance{fullID: subFullID, colDef: &inst, parentKey: parentKey, ancestorKeys: keys})
			walkSubCollectionInstancesBelow(subFullID, &inst, keys, fn)
		}
	}
}

// parentKeysWithSubCollectionDir returns the names of the per-record
// directories under parentColDef's records base path that contain a directory
// named subID — the keys of parents that hold data for that subcollection,
// whether or not a parent record file exists. Hidden entries, `$`-prefixed
// entries (inGitDB's own `$records`, `$views`, ...; no record key starts with
// `$`), and names matched by the parent's exclude_regex are skipped. An
// unreadable or missing base directory yields none.
func parentKeysWithSubCollectionDir(parentColDef *ingitdb.CollectionDef, subID string) []string {
	base := parentColDef.DirPath
	if parentColDef.RecordFile != nil {
		base = filepath.Join(base, parentColDef.RecordFile.RecordsBasePath())
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var keys []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
			continue
		}
		if parentColDef.RecordFile != nil && parentColDef.RecordFile.IsExcluded(name) {
			continue
		}
		info, statErr := os.Stat(filepath.Join(base, name, subID))
		if statErr != nil || !info.IsDir() {
			continue
		}
		keys = append(keys, name)
	}
	return keys
}

// validateSubCollections runs the per-record schema pass and per-parent-instance
// record-count enforcement over every subcollection instance in the definition,
// appending findings to result. It is purely additive to the root pass in
// simpleValidator.Validate: root collections are validated there, subcollection
// records here, with the same per-record machinery (validateCollectionRecords).
//
// Record-count bookkeeping for a subcollection aggregates across all parent
// instances under its full path, so SetRecordCounts is called once per
// subcollection id after the walk rather than per instance (which would let a
// later instance overwrite an earlier one).
func validateSubCollections(def *ingitdb.Definition, result *ingitdb.ValidationResult) {
	if def == nil {
		return
	}
	passedByID := make(map[string]int)
	totalByID := make(map[string]int)
	seen := make(map[string]struct{})

	for _, rootID := range slices.Sorted(maps.Keys(def.Collections)) {
		walkSubCollectionInstances(rootID, def.Collections[rootID], func(inst subCollectionInstance) {
			passed, total, errs := validateCollectionRecords(inst.fullID, inst.colDef)
			for _, validationErr := range errs {
				result.Append(validationErr)
			}
			// Record-count bounds apply per parent-record instance: each parent's
			// instance is an independent collection with its own count. The finding
			// is collection-level (no field/record), but its FilePath names the
			// instance data directory so the owning parent is identifiable.
			for _, validationErr := range checkRecordCountConstraints(inst.fullID, inst.colDef, total) {
				validationErr.FilePath = inst.colDef.DirPath
				result.Append(validationErr)
			}
			passedByID[inst.fullID] += passed
			totalByID[inst.fullID] += total
			seen[inst.fullID] = struct{}{}
		})
	}

	for id := range seen {
		result.SetRecordCounts(id, passedByID[id], totalByID[id])
		result.SetRecordCount(id, totalByID[id])
	}
}

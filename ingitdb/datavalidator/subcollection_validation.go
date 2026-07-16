package datavalidator

// specscore: feature/subcollection-record-validation

import (
	"maps"
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
// expanded into one instance per parent record. Subcollection ids and parent
// keys are visited in sorted order so findings are deterministic.
//
// A read/parse failure at a level is not reported here (the schema pass reports
// it for that collection); the level simply yields no instances.
func walkSubCollectionInstances(fullID string, colDef *ingitdb.CollectionDef, fn func(subCollectionInstance)) {
	if colDef == nil || len(colDef.SubCollections) == 0 {
		return
	}
	parents, err := loadCollectionRecords(colDef)
	if err != nil {
		return
	}
	slices.SortFunc(parents, func(a, b loadedRecord) int { return strings.Compare(a.Key, b.Key) })

	for _, subID := range slices.Sorted(maps.Keys(colDef.SubCollections)) {
		sub := colDef.SubCollections[subID]
		subFullID := fullID + "/" + subID
		for _, pr := range parents {
			inst := *sub // shallow copy: repoint DirPath without mutating the shared definition
			inst.DirPath = subCollectionDataDir(colDef, pr.Key, subID)
			fn(subCollectionInstance{fullID: subFullID, colDef: &inst, parentKey: pr.Key})
			walkSubCollectionInstances(subFullID, &inst, fn)
		}
	}
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

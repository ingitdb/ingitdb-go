package datavalidator

// specscore: feature/subcollection-record-validation

import (
	"fmt"
	"io/fs"
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
	return filepath.Join(recordsBaseDir(parentColDef), parentKey, subID)
}

// instanceWalk carries what a walk over subcollection instances needs beyond
// the definition: the database root (for the inside-root rule of
// ingitdb.ScanParentDirs; "" disables it) and an optional callback for a
// parent directory that resolves outside the database root, which is not
// read.
type instanceWalk struct {
	root      string
	onOutside func(subFullID, parentDir, key string)
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
// file exists. The second case mirrors Firestore, where a document may own
// subcollections without existing itself (`ext/commitius/cmSessions/{id}`
// with no `ext/commitius` document); without it such records were silently
// never validated. Which directories count is decided by
// ingitdb.ScanParentDirs, the one rule shared with the local storage driver's
// collection-group read; the parent's directory is read once per level.
//
// Directory discovery looks in <DirPath>/<records_dir>/<key>/<sub>/, the
// layout validation reads. With records_dir "." that is also where the
// storage drivers read and write; under the default records_dir ($records)
// the drivers use <DirPath>/<key>/<sub>/ instead, which is an open layout
// decision (see driverLayoutWarnings).
//
// A read/parse failure at a level is not reported here (the schema pass reports
// it for that collection); the level then yields only the directory-discovered
// instances.
func walkSubCollectionInstances(fullID string, colDef *ingitdb.CollectionDef, fn func(subCollectionInstance)) {
	instanceWalk{}.walk(fullID, colDef, fn)
}

func (w instanceWalk) walk(fullID string, colDef *ingitdb.CollectionDef, fn func(subCollectionInstance)) {
	w.walkBelow(fullID, colDef, nil, fn)
}

// walkBelow is walk for a colDef whose own records sit below the given
// ancestor keys.
func (w instanceWalk) walkBelow(fullID string, colDef *ingitdb.CollectionDef, ancestorKeys []string, fn func(subCollectionInstance)) {
	if colDef == nil || len(colDef.SubCollections) == 0 {
		return
	}
	parents, _ := loadCollectionRecords(colDef)
	recordKeys := make([]string, 0, len(parents))
	for _, pr := range parents {
		recordKeys = append(recordKeys, pr.Key)
	}
	base := recordsBaseDir(colDef)
	scan := ingitdb.ScanParentDirs(w.root, base, colDef.RecordFile)

	for _, subID := range slices.Sorted(maps.Keys(colDef.SubCollections)) {
		sub := colDef.SubCollections[subID]
		subFullID := fullID + "/" + subID
		dirKeys, outside := scan.KeysWith(subID)
		if w.onOutside != nil {
			for _, key := range outside {
				w.onOutside(subFullID, base, key)
			}
		}
		parentKeys := append(slices.Clone(recordKeys), dirKeys...)
		slices.Sort(parentKeys)
		parentKeys = slices.Compact(parentKeys)
		// A parent whose instance directory leads outside the database is not
		// read, whether or not the parent has a record file.
		parentKeys = slices.DeleteFunc(parentKeys, func(key string) bool { return slices.Contains(outside, key) })
		for _, parentKey := range parentKeys {
			inst := *sub // shallow copy: repoint DirPath without mutating the shared definition
			inst.DirPath = subCollectionDataDir(colDef, parentKey, subID)
			keys := append(slices.Clone(ancestorKeys), parentKey)
			fn(subCollectionInstance{fullID: subFullID, colDef: &inst, parentKey: parentKey, ancestorKeys: keys})
			w.walkBelow(subFullID, &inst, keys, fn)
		}
	}
}

// recordsBaseDir is <DirPath>/<records_dir>, where a collection's per-record
// directories live in the layout validation reads.
func recordsBaseDir(colDef *ingitdb.CollectionDef) string {
	if colDef.RecordFile == nil {
		return colDef.DirPath
	}
	return filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
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
func validateSubCollections(root string, def *ingitdb.Definition, result *ingitdb.ValidationResult) {
	if def == nil {
		return
	}
	w := instanceWalk{root: root, onOutside: func(subFullID, parentDir, key string) {
		result.AddWarning(ingitdb.ValidationError{
			CollectionID: subFullID,
			FilePath:     filepath.Join(parentDir, key),
			Message: fmt.Sprintf("%s: %s/%s resolves outside the database through a symbolic link; its records are not read or validated",
				subFullID, relToRoot(root, filepath.Join(parentDir, key)), lastSegment(subFullID)),
		})
	}}
	passedByID := make(map[string]int)
	totalByID := make(map[string]int)
	seen := make(map[string]struct{})

	for _, rootID := range slices.Sorted(maps.Keys(def.Collections)) {
		w.walk(rootID, def.Collections[rootID], func(inst subCollectionInstance) {
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

// relToRoot renders p relative to the database root for messages; p itself
// when root is "" or p is not below it.
func relToRoot(root, p string) string {
	if root == "" {
		return p
	}
	if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return p
}

// lastSegment is the last "/"-separated segment of a schema path.
func lastSegment(fullID string) string {
	return fullID[strings.LastIndex(fullID, "/")+1:]
}

// mislaidSubCollectionFindings reports subcollection data stored where the
// definition does not put it. The definition puts one parent record's
// instance of subcollection <sub> at <DirPath>/<records_dir>/<key>/<sub>/,
// next to the parent's record files (records_dir is the parent's
// record_file.records_dir; the default is $records when the record file
// name contains {key}; "." is the data directory itself). When the parent's
// records_dir is not ".", an instance found at <DirPath>/<key>/<sub>/ (the
// layout earlier storage-driver versions wrote) is mislaid: its records are
// not validated, and each such directory is one finding naming the
// collection, the found path, the expected path, the file count and the fix.
func mislaidSubCollectionFindings(root string, def *ingitdb.Definition) []ingitdb.ValidationError {
	if def == nil {
		return nil
	}
	var findings []ingitdb.ValidationError
	check := func(fullID string, colDef *ingitdb.CollectionDef) {
		if colDef.RecordFile == nil || colDef.RecordFile.RecordsBasePath() == "" || len(colDef.SubCollections) == 0 {
			return
		}
		base := colDef.RecordFile.RecordsBasePath()
		scan := ingitdb.ScanParentDirs(root, colDef.DirPath, colDef.RecordFile)
		for _, subID := range slices.Sorted(maps.Keys(colDef.SubCollections)) {
			keys, _ := scan.KeysWith(subID)
			for _, key := range keys {
				found := filepath.Join(colDef.DirPath, key, subID)
				files := countFiles(found)
				if files == 0 {
					continue
				}
				subFullID := fullID + "/" + subID
				message := fmt.Sprintf("%s: %d file(s) at %s/ are where the definition does not put them, and are not validated: %s keeps its records in record_file.records_dir %q, so the %s of parent record %q belong at %s/. Move them there, or, if %s should use the Firestore-mirror layout, set record_file.records_dir: \".\" on %s and move its own records out of %s/",
					subFullID, files, relToRoot(root, found), fullID, base, subID, key, relToRoot(root, filepath.Join(colDef.DirPath, base, key, subID)),
					fullID, fullID, relToRoot(root, filepath.Join(colDef.DirPath, base)))
				findings = append(findings, ingitdb.ValidationError{
					Severity:     ingitdb.SeverityError,
					CollectionID: subFullID,
					FilePath:     found,
					RecordKey:    key,
					Message:      message,
				})
			}
		}
	}
	for _, rootID := range slices.Sorted(maps.Keys(def.Collections)) {
		col := def.Collections[rootID]
		check(rootID, col)
		instanceWalk{root: root}.walk(rootID, col, func(inst subCollectionInstance) {
			check(inst.fullID, inst.colDef)
		})
	}
	return findings
}

// countFiles counts the non-hidden files below dir, at any depth.
func countFiles(dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
			n++
		}
		return nil
	})
	return n
}

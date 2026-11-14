package datavalidator

// specscore: feature/record-key-encoding

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// collisionFileSystems names, without promising more than the comparison
// establishes, where names that ingitdb.CaseOnlyDifferences groups can be
// one entry.
const collisionFileSystems = "case-insensitive or normalisation-insensitive file systems (for example macOS; Windows for letter case)"

// nameIndex finds name collisions (record-key-encoding#req:file-name-collisions-reported)
// for one validation call. Passes register the directory entries they use;
// findings then lists each registered directory once, groups its entries
// with ingitdb.CaseOnlyDifferences, and reports every group of two or more
// entries that holds a used name. Full and incremental validation register
// the same entries for the same records, so they report the same groups.
type nameIndex struct {
	root    string
	readDir func(string) ([]os.DirEntry, error)
	// used holds, per directory, the names a pass used and the smallest
	// collection id that used each. A used name not on disk (a deleted
	// record) joins its group but is not reported as a member.
	used map[string]map[string]string
	// allUsed marks records directories: every entry in them is used.
	allUsed map[string]bool
	// declared holds names the definition declares (collection directories),
	// members of their groups whether or not they are on disk.
	declared map[string]map[string]bool
	// owner is the collection a finding in a directory belongs to: the
	// collection instance whose records directory it is, or the parent
	// collection of a parent record directory.
	owner map[string]string
}

func newNameIndex(root string, readDir func(string) ([]os.DirEntry, error)) *nameIndex {
	if readDir == nil {
		readDir = os.ReadDir
	}
	return &nameIndex{root: root, readDir: readDir, used: map[string]map[string]string{},
		allUsed: map[string]bool{}, declared: map[string]map[string]bool{}, owner: map[string]string{}}
}

func minID(current, id string) string {
	if current == "" || id < current {
		return id
	}
	return current
}

func (x *nameIndex) use(id, dir, name string, declared bool) {
	if x.used[dir] == nil {
		x.used[dir] = map[string]string{}
	}
	x.used[dir][name] = minID(x.used[dir][name], id)
	if declared {
		if x.declared[dir] == nil {
			x.declared[dir] = map[string]bool{}
		}
		x.declared[dir][name] = true
	}
}

// usePath registers every segment of path below the root, each in its
// parent directory. A path outside the root (or with no root) registers its
// last segment only; the root itself has none.
func (x *nameIndex) usePath(id, path string) {
	x.walkPath(id, path, false)
}

func (x *nameIndex) walkPath(id, path string, declared bool) {
	path = filepath.Clean(path)
	rel, err := filepath.Rel(x.root, path)
	if x.root == "" || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		x.use(id, filepath.Dir(path), filepath.Base(path), declared)
		return
	}
	if rel == "." {
		return
	}
	dir := filepath.Clean(x.root)
	for _, segment := range strings.Split(rel, string(filepath.Separator)) {
		x.use(id, dir, segment, declared)
		dir = filepath.Join(dir, segment)
	}
}

// useRecordsDir registers a collection instance's records directory: its
// path, and every entry in it.
func (x *nameIndex) useRecordsDir(id, dir string) {
	x.usePath(id, dir)
	dir = filepath.Clean(dir)
	x.allUsed[dir] = true
	x.own(id, dir)
	if x.used[dir] == nil {
		x.used[dir] = map[string]string{}
	}
}

func (x *nameIndex) own(id, dir string) {
	dir = filepath.Clean(dir)
	x.owner[dir] = minID(x.owner[dir], id)
}

// useRecord registers a record file (or a deleted one) of a collection
// instance: the path to the instance's records directory, and the first
// segment below it (the record file, or the directory of a {key}/index.yaml
// record).
func (x *nameIndex) useRecord(levels []instanceLevel, filePath string) {
	for i, level := range levels {
		x.own(level.id, recordsBaseDir(level.colDef))
		if i > 0 {
			x.own(levels[i-1].id, filepath.Dir(level.colDef.DirPath))
		}
	}
	last := levels[len(levels)-1]
	base := recordsBaseDir(last.colDef)
	rel, _ := filepath.Rel(base, filePath)
	x.usePath(last.id, filepath.Join(base, strings.Split(rel, string(filepath.Separator))[0]))
}

// findings lists each registered directory once and reports its collisions,
// sorted by directory.
func (x *nameIndex) findings() []ingitdb.ValidationError {
	var findings []ingitdb.ValidationError
	for _, dir := range slices.Sorted(maps.Keys(x.used)) {
		entries, _ := x.readDir(dir)
		present := map[string]bool{}
		for _, e := range entries {
			present[e.Name()] = true
		}
		for name := range x.declared[dir] {
			present[name] = true
		}
		names := slices.Sorted(maps.Keys(present))
		for name := range x.used[dir] {
			if !present[name] {
				names = append(names, name)
			}
		}
		for _, group := range ingitdb.CaseOnlyDifferences(names) {
			members := slices.DeleteFunc(slices.Clone(group), func(name string) bool { return !present[name] })
			id := x.owner[dir]
			usedHere := x.allUsed[dir]
			for _, name := range group {
				if userID, ok := x.used[dir][name]; ok {
					usedHere = true
					if x.owner[dir] == "" {
						id = minID(id, userID)
					}
				}
			}
			if len(members) < 2 || !usedHere {
				continue
			}
			slices.Sort(members)
			shown, reason := collisionReason(members)
			findings = append(findings, newValidationError(id, filepath.Join(dir, members[0]), "", "",
				fmt.Sprintf("names %s in %s %s, so they may be the same file on %s; a checkout there keeps only one of them and loses the others' data: rename all but one",
					strings.Join(shown, ", "), dir, reason, collisionFileSystems), nil))
		}
	}
	return findings
}

// collisionReason describes a group of names that ingitdb.CaseOnlyDifferences
// found to be one entry: the names as a message shows them (escaped when they
// differ in Unicode normalisation, which prints the same, or when one is not
// printable as it is), and how they differ.
func collisionReason(names []string) ([]string, string) {
	letterCase, normalisation := ingitdb.NameCollisionKind(names)
	shown := slices.Clone(names)
	if normalisation || slices.ContainsFunc(names, unsafeName) {
		for i, name := range shown {
			shown[i] = strconv.QuoteToASCII(name)
		}
	}
	switch {
	case letterCase && normalisation:
		return shown, "differ only in letter case and Unicode normalisation"
	case normalisation:
		return shown, "differ only in Unicode normalisation"
	default:
		return shown, "differ only in letter case"
	}
}

// unsafeName reports a name a message cannot show as it is: not valid UTF-8,
// or holding a character that does not print.
func unsafeName(name string) bool {
	return !utf8.ValidString(name) || strings.ContainsFunc(name, func(r rune) bool { return !strconv.IsPrint(r) })
}

// definitionNameFindings declares every root collection's data directory in
// the index (so two collections whose directories collide at any level of
// their path are one group in that level's parent directory, on disk or
// not), and returns the subcollection ids of one collection that collide:
// their directories sit side by side under every parent record. Both full and
// incremental validation call it; it reads no file.
func definitionNameFindings(idx *nameIndex, def *ingitdb.Definition) []ingitdb.ValidationError {
	var findings []ingitdb.ValidationError
	for _, id := range slices.Sorted(maps.Keys(def.Collections)) {
		colDef := def.Collections[id]
		if colDef != nil && colDef.DirPath != "" {
			idx.walkPath(id, colDef.DirPath, true)
			idx.own(id, recordsBaseDir(colDef))
		}
		findings = append(findings, subCollectionIDFindings(id, colDef)...)
	}
	return findings
}

// subCollectionIDFindings reports, recursively, subcollection ids of one
// collection that name one directory.
func subCollectionIDFindings(fullID string, colDef *ingitdb.CollectionDef) []ingitdb.ValidationError {
	if colDef == nil {
		return nil
	}
	ids := slices.Sorted(maps.Keys(colDef.SubCollections))
	var findings []ingitdb.ValidationError
	for _, group := range ingitdb.CaseOnlyDifferences(ids) {
		shown, reason := collisionReason(group)
		findings = append(findings, newValidationError(fullID+"/"+group[0], "", "", "",
			fmt.Sprintf("subcollections %s of %s %s, so under every parent record their directories may be one directory on %s, and a checkout there mixes their records: rename all but one",
				strings.Join(shown, ", "), fullID, reason, collisionFileSystems), nil))
	}
	for _, id := range ids {
		findings = append(findings, subCollectionIDFindings(fullID+"/"+id, colDef.SubCollections[id])...)
	}
	return findings
}

// parentNameFinding reports a subcollection instance directory whose parent
// directory name is not one the parent collection's key_encoding writes.
func parentNameFinding(fullID, dirPath, parentKey string, err error) ingitdb.ValidationError {
	return newValidationError(fullID, dirPath, parentKey, "",
		"the parent directory name is not one the parent collection's key_encoding writes, so it names no parent record; rename the directory: "+err.Error(), nil)
}

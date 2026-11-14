package ingitdb

// specscore: feature/subcollection-record-validation

import (
	"os"
	"path/filepath"
	"strings"
)

// ParentDirs is one scan of a directory that holds per-parent-record
// directories, the place a subcollection's instances live:
// <base>/<parentKey>/<subcollection>/. It is the single rule, shared by
// `ingitdb validate` and the local storage driver's collection-group read,
// for which directories are parent instances, so a reader can never return a
// record that validation did not consider.
//
// The rule:
//   - hidden entries (".") are not parents;
//   - the names inGitDB itself writes in a collection directory ($records,
//     $views, $ingitdb, and the first segment of the parent's records
//     directory, "data" for records_dir data/items) are not parents, even
//     under records_dir "." where a Firestore document could carry such an
//     id; every other name is a parent key, including other "$"-prefixed
//     names, since Firestore allows document ids such as "$x";
//   - names matched by the parent's exclude_regex are not parents;
//   - an instance directory reached through a symbolic link is used only when
//     it resolves inside the database root; otherwise it is not read and is
//     reported (Outside), so nothing outside the database is read.
//
// A parent key qualifies for a subcollection when <base>/<key>/<subID>
// resolves to a directory.
type ParentDirs struct {
	base       string
	realRoot   string // the database root with symlinks resolved; "" disables the check
	candidates []string
}

// reservedDirNames are the directory names inGitDB writes inside a
// collection's data directory; they are never parent keys.
var reservedDirNames = map[string]bool{"$records": true, "$views": true, IngitdbDir: true}

// ScanParentDirs reads base once. root is the database root; when it is ""
// or cannot be resolved, symbolic links are followed without the inside-root
// check. A missing or unreadable base yields no candidates.
func ScanParentDirs(root, base string, recordFile *RecordFileDef) ParentDirs {
	scan := ParentDirs{base: base}
	if root != "" {
		abs, _ := filepath.Abs(root)
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			scan.realRoot = real
		}
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return scan
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || reservedDirNames[name] {
			continue
		}
		if recordFile != nil && (recordFile.IsExcluded(name) || name == recordsDirHead(recordFile)) {
			continue
		}
		scan.candidates = append(scan.candidates, name)
	}
	return scan
}

// KeysWith returns, in name order, the parent keys whose <key>/<subID>
// resolves to a directory inside the database root, and the keys whose
// <key>/<subID> resolves outside it through a symbolic link (not to be read).
func (s ParentDirs) KeysWith(subID string) (keys, outside []string) {
	for _, name := range s.candidates {
		real, err := filepath.EvalSymlinks(filepath.Join(s.base, name, subID))
		if err != nil {
			continue
		}
		real, _ = filepath.Abs(real)
		if info, statErr := os.Stat(real); statErr != nil || !info.IsDir() {
			continue
		}
		if s.realRoot != "" && !isWithin(s.realRoot, real) {
			outside = append(outside, name)
			continue
		}
		keys = append(keys, name)
	}
	return keys, outside
}

// recordsDirHead is the first path segment of the records directory, the
// entry of the collection directory inGitDB keeps record files under; ""
// when records sit in the collection directory itself.
func recordsDirHead(recordFile *RecordFileDef) string {
	head, _, _ := strings.Cut(recordFile.RecordsBasePath(), "/")
	return head
}

// isWithin reports whether path p is root or below it.
func isWithin(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

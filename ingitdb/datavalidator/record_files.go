package datavalidator

// specscore: feature/record-key-encoding

import (
	"os"
	"path/filepath"
	"strings"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// The record files of a single-record collection instance are the paths
// below its records directory that record_file.name names, {key} matching
// any name. The records directory is a literal path: it is never turned into
// a glob, so its names (a checkout under "proj[1]", a parent key "[p]" or
// `p\q`, bytes that are not UTF-8, a Windows volume) are never read as a
// pattern. Only the segments of record_file.name are patterns, matched one
// path segment at a time with filepath.Match. recordFiles (full validation,
// the foreign-key and subcollection passes) and isRecordFilePath
// (incremental validation and change resolution) share recordNameSegments
// and filepath.Match, so they find the same files.

// recordNameSegments is record_file.name with {key} as *, split into its
// "/"-separated segments.
func recordNameSegments(rf *ingitdb.RecordFileDef) []string {
	return strings.Split(strings.ReplaceAll(rf.Name, "{key}", "*"), "/")
}

// globMeta are the characters that make a segment a pattern for
// filepath.Match; a backslash escapes on every platform but Windows.
func globMeta() string {
	meta := "*?["
	if filepath.Separator == '/' {
		meta += `\`
	}
	return meta
}

// recordFiles returns, in name order, the record files of colDef's records
// directory, as filepath.Glob would for the pattern of record_file.name
// joined onto a literal directory: a segment without pattern characters is
// checked with Lstat, any other segment is matched against the directory's
// entries. Unreadable directories hold nothing; a malformed pattern fails
// with filepath.ErrBadPattern.
func recordFiles(colDef *ingitdb.CollectionDef) ([]string, error) {
	segments := recordNameSegments(colDef.RecordFile)
	for _, segment := range segments {
		if _, err := filepath.Match(segment, ""); err != nil {
			return nil, err
		}
	}
	paths := []string{recordsBaseDir(colDef)}
	for _, segment := range segments {
		var next []string
		for _, dir := range paths {
			if !strings.ContainsAny(segment, globMeta()) {
				if _, err := os.Lstat(filepath.Join(dir, segment)); err == nil {
					next = append(next, filepath.Join(dir, segment))
				}
				continue
			}
			entries, _ := os.ReadDir(dir)
			for _, entry := range entries {
				if matched, _ := filepath.Match(segment, entry.Name()); matched {
					next = append(next, filepath.Join(dir, entry.Name()))
				}
			}
		}
		paths = next
	}
	return paths, nil
}

// isRecordFilePath reports whether absPath is a record file of the
// single-record collection instance colDef: below its records directory,
// matched segment by segment, and not hidden or excluded.
func isRecordFilePath(colDef *ingitdb.CollectionDef, absPath string) bool {
	rel, err := filepath.Rel(recordsBaseDir(colDef), absPath)
	return err == nil && matchRecordPath(recordNameSegments(colDef.RecordFile), rel, string(filepath.Separator)) &&
		!skipRecordPath(absPath, colDef.RecordFile)
}

// matchRecordPath reports whether rel, a path relative to a records
// directory with segments separated by separator, is named by the record
// name segments: one path segment per name segment, each matched with
// filepath.Match. A path that leaves the directory matches nothing, and so
// does a malformed pattern.
func matchRecordPath(segments []string, rel, separator string) bool {
	parts := strings.Split(rel, separator)
	if len(parts) != len(segments) || parts[0] == ".." || parts[0] == "." {
		return false
	}
	for i, segment := range segments {
		if matched, err := filepath.Match(segment, parts[i]); err != nil || !matched {
			return false
		}
	}
	return true
}

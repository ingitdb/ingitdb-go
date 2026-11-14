package datavalidator

// specscore: feature/record-key-encoding

import (
	"errors"
	"io/fs"
	"os"
	"path"
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
// path segment at a time with filepath.Match, exactly as filepath.Glob
// matches them. recordFiles (full validation, the foreign-key and
// subcollection passes) and isRecordFilePath (incremental validation and
// change resolution) share recordPattern and filepath.Match, so they find
// the same files.

// recordPattern returns the directory record_file.name is relative to and
// the name's segments, {key} as *. The name is cleaned as filepath.Join
// cleaned it when it was joined onto the records directory: "." and empty
// segments drop out, "a/../" folds away, and leading ".." segments move the
// directory up.
func recordPattern(colDef *ingitdb.CollectionDef) (string, []string) {
	dir := recordsBaseDir(colDef)
	name := path.Clean(strings.TrimLeft(filepath.ToSlash(strings.ReplaceAll(colDef.RecordFile.Name, "{key}", "*")), "/"))
	segments := strings.Split(name, "/")
	for len(segments) > 0 && segments[0] == ".." {
		dir = filepath.Dir(dir)
		segments = segments[1:]
	}
	if len(segments) == 1 && segments[0] == "." {
		return dir, nil // a name that is the directory itself names no file
	}
	return dir, segments
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

// unreadableDir is a directory recordFiles could not list, so the records
// below it were not read.
type unreadableDir struct {
	dir string
	err error
}

// recordFiles returns, in name order, the record files of colDef's records
// directory, as filepath.Glob returns them for the pattern of
// record_file.name joined onto a literal directory: a name without pattern
// characters is checked with Lstat; otherwise the segments before the first
// pattern segment are joined literally, and from there every segment, a
// literal one included, is matched with filepath.Match against the entries
// the directory lists, so a match is exact and case-sensitive on every file
// system. A malformed pattern fails with filepath.ErrBadPattern. A missing
// directory holds nothing; unreadable lists the records directory when it
// exists but cannot be read, and a directory below it that cannot be read
// for lack of permission.
func recordFiles(colDef *ingitdb.CollectionDef) ([]string, []unreadableDir, error) {
	return recordFilesIn(colDef, os.ReadDir)
}

func recordFilesIn(colDef *ingitdb.CollectionDef, readDir func(string) ([]os.DirEntry, error)) (paths []string, unreadable []unreadableDir, err error) {
	dir, segments := recordPattern(colDef)
	first := -1
	for i, segment := range segments {
		if _, err := filepath.Match(segment, ""); err != nil {
			return nil, nil, err
		}
		if first < 0 && strings.ContainsAny(segment, globMeta()) {
			first = i
		}
	}
	if len(segments) == 0 {
		return nil, nil, nil
	}
	if first < 0 {
		p := filepath.Join(append([]string{dir}, segments...)...)
		if _, err := os.Lstat(p); err == nil {
			return []string{p}, nil, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, []unreadableDir{{dir: filepath.Dir(p), err: err}}, nil
		}
		return nil, nil, nil
	}
	start := filepath.Join(append([]string{dir}, segments[:first]...)...)
	paths = []string{start}
	for _, segment := range segments[first:] {
		var next []string
		for _, d := range paths {
			entries, err := readDir(d)
			if err != nil {
				if d == start && !errors.Is(err, fs.ErrNotExist) || d != start && errors.Is(err, fs.ErrPermission) {
					unreadable = append(unreadable, unreadableDir{dir: d, err: err})
				}
				continue
			}
			for _, entry := range entries {
				if matched, _ := filepath.Match(segment, entry.Name()); matched {
					next = append(next, filepath.Join(d, entry.Name()))
				}
			}
		}
		paths = next
	}
	return paths, unreadable, nil
}

// isRecordFilePath reports whether absPath is a record file of the
// single-record collection instance colDef: below its records directory,
// matched segment by segment, and not hidden or excluded.
func isRecordFilePath(colDef *ingitdb.CollectionDef, absPath string) bool {
	dir, segments := recordPattern(colDef)
	rel, err := filepath.Rel(dir, absPath)
	return err == nil && matchRecordPath(segments, rel, string(filepath.Separator)) &&
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

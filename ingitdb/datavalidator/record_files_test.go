package datavalidator

// specscore: feature/record-key-encoding

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// recordFilesLayout is a records directory with the oddities record
// enumeration must treat exactly as filepath.Glob did: letter-case variants,
// hidden names, directories and symbolic links named like records, deeper
// files, glob characters and a backslash in file names.
func recordFilesLayout(t *testing.T, base string) {
	t.Helper()
	files := map[string]string{
		"a.yaml": "", "b.YAML": "", ".hidden.yaml": "", "README.md": "", "index.yaml": "",
		"dir.yaml/x": "", "k1/index.yaml": "", "k2/index.yaml": "", "k3/INDEX.yaml": "", ".k4/index.yaml": "",
		"k5/deeper/index.yaml": "", "k6/index.yaml/x": "", "sub/s.json": "", "sub/S.JSON": "",
		"q[1].json": "", "z.JSON": "", "static.yaml": "",
	}
	if filepath.Separator == '/' {
		files[`b\x.json`] = ""
	}
	writeTree(t, base, files)
	for link, target := range map[string]string{"link.yaml": "a.yaml", "linkdir.yaml": "dir.yaml", "dangling.yaml": "missing", "k7": "k1"} {
		if err := os.Symlink(target, filepath.Join(base, link)); err != nil {
			t.Fatal(err)
		}
	}
}

// Keeps the differential test of the round-4 review: over one layout and
// every name pattern, recordFiles finds exactly the files filepath.Glob finds
// for the name joined onto the records directory (the enumeration before
// directory names became literal), and incremental validation's matcher
// accepts exactly those of them that are not hidden or excluded. Names that
// are not clean ("./{key}.yaml", "{key}//index.yaml", "a/../") are cleaned
// as filepath.Join cleaned them.
func TestRecordFiles_SameFilesAsGlob(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, "c", "$records")
	recordFilesLayout(t, base)
	var all []string
	if err := filepath.WalkDir(root, func(p string, _ fs.DirEntry, err error) error {
		all = append(all, p)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"{key}.yaml", "{key}/index.yaml", "{key}/[i]ndex.yaml", "./{key}.yaml", "{key}//index.yaml", "{key}/./index.yaml",
		"{key}/../{key}.yaml", "/{key}.yaml", "sub/{key}.json", "{key}.json", "[ab]*.yaml", "static.yaml", "missing.yaml",
		"../$records/{key}.yaml", "{key}/deeper/index.yaml", "$records/*/*/index.yaml", "$records/static.yaml", "{key}", "./", "..",
	} {
		colDef := &ingitdb.CollectionDef{ID: "c", DirPath: filepath.Join(root, "c"),
			RecordFile: &ingitdb.RecordFileDef{Name: name, Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord}}
		got, unreadable, err := recordFiles(colDef)
		if err != nil || unreadable != nil {
			t.Fatalf("%s: %v %v", name, err, unreadable)
		}
		// a name without {key} is relative to the collection directory
		want, _ := filepath.Glob(filepath.Join(recordsBaseDir(colDef), strings.ReplaceAll(name, "{key}", "*")))
		if name == "./" || name == ".." {
			want = nil // the directory itself, or its parent, is no record file
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s:\nrecordFiles %q\nGlob        %q", name, got, want)
		}
		for _, p := range all {
			if isRecord, wantRecord := isRecordFilePath(colDef, p), slices.Contains(want, p) && !skipRecordPath(p, colDef.RecordFile); isRecord != wantRecord {
				t.Errorf("%s: isRecordFilePath(%s) = %v", name, p, isRecord)
			}
		}
	}
}

// renamedEntry is a directory entry listed under another name, as a
// case-insensitive file system lists a file stored as INDEX.yaml that a
// lookup of index.yaml finds.
type renamedEntry struct {
	os.DirEntry
	name string
}

func (e renamedEntry) Name() string { return e.name }

// A literal segment after a pattern segment matches the names the directory
// lists, exactly and case-sensitively, as filepath.Glob matches them: on a
// file system that finds k1/index.yaml under the stored name k1/INDEX.yaml,
// "{key}/index.yaml" names no record, as the incremental matcher says.
func TestRecordFiles_LiteralSegmentsMatchListedNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"c/$records/k1/index.yaml": "title: a\n", "c/$records/k2/index.yaml": "title: b\n"})
	caseInsensitive := func(dir string) ([]os.DirEntry, error) {
		entries, err := os.ReadDir(dir)
		for i, e := range entries {
			if filepath.Base(dir) == "k1" {
				entries[i] = renamedEntry{e, strings.ToUpper(strings.TrimSuffix(e.Name(), ".yaml")) + ".yaml"}
			}
		}
		return entries, err
	}
	colDef := &ingitdb.CollectionDef{ID: "c", DirPath: filepath.Join(root, "c"),
		RecordFile: &ingitdb.RecordFileDef{Name: "{key}/index.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord}}
	got, _, _ := recordFilesIn(colDef, caseInsensitive)
	if want := []string{filepath.Join(root, "c", "$records", "k2", "index.yaml")}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if isRecordFilePath(colDef, filepath.Join(root, "c", "$records", "k1", "INDEX.yaml")) {
		t.Error("the incremental matcher accepts INDEX.yaml")
	}
}

// A record_file.name that is not clean ("./{key}.yaml", "{key}//index.yaml")
// names the same records in full and incremental validation: a changed
// invalid record is reported by both.
func TestIncrementalValidator_UncleanRecordNames(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, file string }{
		{"./{key}.yaml", "c/$records/bad.yaml"},
		{"{key}//index.yaml", "c/$records/bad/index.yaml"},
		{"{key}/../{key}.yaml", "c/$records/bad.yaml"},
	} {
		root := t.TempDir()
		writeTree(t, root, map[string]string{tc.file: "title: [\n"})
		def := plainDB(root)
		def.Collections["c"].RecordFile.Name = tc.name
		full := fullFindings(t, root, def)
		inc := incrementalFindings(t, root, def, added(tc.file))
		if len(full) != 1 || findingTexts(inc) != findingTexts(full) {
			t.Errorf("%s:\nfull %s\nincremental %s", tc.name, findingTexts(full), findingTexts(inc))
		}
	}
}

// The incremental matcher takes a path with one segment per name segment
// only, and never a hidden or excluded file.
func TestIsRecordFilePath_SegmentsHiddenAndExcluded(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "c")
	rf := plainRecordFile()
	rf.ExcludeRegex = `^skip`
	colDef := &ingitdb.CollectionDef{ID: "c", DirPath: base, RecordFile: rf}
	for rel, want := range map[string]bool{
		"$records/a.yaml":          true,
		"$records/dir.yaml/x.yaml": false, // deeper than the name
		"$records/dir.yaml/x":      false,
		"$records/.h.yaml":         false, // hidden
		"$records/skip.yaml":       false, // excluded
	} {
		if got := isRecordFilePath(colDef, filepath.Join(base, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s: %v, want %v", rel, got, want)
		}
	}
}

// A records directory that exists but cannot be read is an error, not a
// collection without records; a missing one holds no records. Below it, a
// directory that cannot be read for lack of permission is an error too.
func TestValidate_UnreadableRecordsDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"c/$records": "not a directory\n", "f/data": "not a directory\n"})
	def := plainDB(root)
	def.Collections["f"] = &ingitdb.CollectionDef{ID: "f", DirPath: filepath.Join(root, "f", "data"), Columns: titleColumns(),
		RecordFile: &ingitdb.RecordFileDef{Name: "static.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord}}
	def.Collections["m"] = &ingitdb.CollectionDef{ID: "m", DirPath: filepath.Join(root, "missing"), Columns: titleColumns(), RecordFile: plainRecordFile()}
	errs := fullFindings(t, root, def)
	if len(errs) != 2 || errs[0].CollectionID != "c" || errs[1].CollectionID != "f" ||
		!strings.Contains(findingTexts(errs), "failed to read the records directory; its records are not validated") ||
		errs[0].FilePath != filepath.Join(root, "c", "$records") || errs[1].FilePath != filepath.Join(root, "f", "data") {
		t.Fatalf("got %s", findingTexts(errs))
	}

	nested := t.TempDir()
	writeTree(t, nested, map[string]string{"c/$records/k1/index.yaml": "", "c/$records/k2/index.yaml": "", "c/$records/README.md": ""})
	denied := func(dir string) ([]os.DirEntry, error) {
		if filepath.Base(dir) == "k1" {
			return nil, fs.ErrPermission
		}
		return os.ReadDir(dir)
	}
	colDef := &ingitdb.CollectionDef{ID: "c", DirPath: filepath.Join(nested, "c"),
		RecordFile: &ingitdb.RecordFileDef{Name: "{key}/index.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord}}
	paths, unreadable, err := recordFilesIn(colDef, denied)
	if err != nil || len(paths) != 1 || len(unreadable) != 1 || unreadable[0].dir != filepath.Join(nested, "c", "$records", "k1") {
		t.Errorf("paths %q, unreadable %v, err %v", paths, unreadable, err) // README.md is no directory: not unreadable
	}
}

// A records directory without read permission fails validation.
func TestValidate_RecordsDirectoryWithoutPermission(t *testing.T) {
	if os.Geteuid() == 0 || filepath.Separator != '/' {
		t.Skip("permissions do not stop root, and are not POSIX on Windows")
	}
	root := t.TempDir()
	writeTree(t, root, map[string]string{"c/$records/bad.yaml": "title: [\n"})
	dir := filepath.Join(root, "c", "$records")
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if errs := fullFindings(t, root, plainDB(root)); len(errs) != 1 || errs[0].FilePath != dir {
		t.Errorf("got %s", findingTexts(errs))
	}
}

// Collections sharing a data directory have parent records of their own:
// a record $views of b owns the instance below $views, though a holds no
// such record and is tried first.
func TestIncrementalValidator_ParentsPerCollection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"data/a.yaml":                         "a1:\n  title: a\n",
		"data/b.yaml":                         "$views:\n  title: b\n",
		"data/$views/notes/$records/bad.yaml": "title: [\n",
	})
	notes := func() map[string]*ingitdb.CollectionDef {
		return map[string]*ingitdb.CollectionDef{"notes": {ID: "notes", RecordFile: plainRecordFile(), Columns: titleColumns()}}
	}
	shared := func(id string) *ingitdb.CollectionDef {
		return &ingitdb.CollectionDef{ID: id, DirPath: filepath.Join(root, "data"), Columns: titleColumns(), SubCollections: notes(),
			RecordFile: &ingitdb.RecordFileDef{Name: id + ".yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.MapOfRecords}}
	}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"a": shared("a"), "b": shared("b")}}
	full := fullFindings(t, root, def)
	inc := incrementalFindings(t, root, def, added("data/$views/notes/$records/bad.yaml"))
	if len(full) != 1 || full[0].CollectionID != "b/notes" || findingTexts(inc) != findingTexts(full) {
		t.Errorf("full %s\nincremental %s", findingTexts(full), findingTexts(inc))
	}
}

// One incremental validation call looks up a collection's parent records
// once, from a listing, and parses only the record files of the directory
// names it asks about, never the whole parent collection.
func TestIncrementalValidator_ParentLookupListsOnceParsesOnlyMatches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		".hidden/index.yaml":              "title: h\n",
		".hidden/notes/$records/n1.yaml":  "title: [\n",
		".hidden/notes/$records/n2.yaml":  "title: [\n",
		".missing/notes/$records/n3.yaml": "title: [\n",
		"$views/notes/$records/n4.yaml":   "title: [\n",
		"p1/index.yaml":                   "title: p\n",
		"p2/index.yaml":                   "title: p\n",
		"p3/index.yaml":                   "title: p\n",
		"p1/notes/$records/ok.yaml":       "title: n\n",
	}
	tree := map[string]string{}
	var changed []ingitdb.ChangedFile
	for p, content := range files {
		tree["c/$records/"+p] = content
		if strings.Contains(p, "notes") {
			changed = append(changed, added("c/$records/"+p))
		}
	}
	changed = append(changed, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindDeleted, Path: "c/$records/.hidden/notes/$records/gone.yaml"})
	writeTree(t, root, tree)
	def := plainDB(root)
	def.Collections["c"].RecordFile.Name = "{key}/index.yaml"

	listed := map[string]int{}
	var loaded []string
	iv := &incrementalValidator{differ: fakeDiffer{files: changed}, resolver: NewChangeSetResolver(), full: NewValidator(),
		paths: func(def *ingitdb.Definition) *pathResolver {
			r := newPathResolver(def)
			r.readDir = countingReadDir(listed)
			r.load = func(colDef *ingitdb.CollectionDef, filePath string) (loadedRecord, bool) {
				loaded = append(loaded, filePath)
				return loadSingleRecordFile(colDef, filePath)
			}
			return r
		}}
	res, err := iv.ValidateChanges(context.Background(), root, def, "A", "B")
	if err != nil {
		t.Fatal(err)
	}
	if n := listed[filepath.Join(root, "c", "$records")]; n != 1 {
		t.Errorf("parent records listed %d times", n)
	}
	if want := []string{filepath.Join(root, "c", "$records", ".hidden", "index.yaml")}; !slices.Equal(loaded, want) {
		t.Errorf("parsed %q, want %q", loaded, want)
	}
	if full := fullFindings(t, root, def); findingTexts(res.Errors()) != findingTexts(full) || len(full) != 2 {
		t.Errorf("incremental %s\nfull %s", findingTexts(res.Errors()), findingTexts(full))
	}
}

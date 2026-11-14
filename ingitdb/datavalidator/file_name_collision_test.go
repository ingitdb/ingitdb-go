package datavalidator

// specscore: feature/record-key-encoding

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

// plainDB is a database whose collections declare no key_encoding: c keeps
// one record per {key}.yaml file and has a subcollection notes.
func plainDB(root string) *ingitdb.Definition {
	str := map[string]*ingitdb.ColumnDef{"title": {Type: ingitdb.ColumnTypeString}}
	rf := func() *ingitdb.RecordFileDef {
		return &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord}
	}
	notes := &ingitdb.CollectionDef{ID: "notes", RecordFile: rf(), Columns: str}
	c := &ingitdb.CollectionDef{ID: "c", DirPath: filepath.Join(root, "c"), RecordFile: rf(), Columns: str,
		SubCollections: map[string]*ingitdb.CollectionDef{"notes": notes}}
	return &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"c": c}}
}

func validateTree(t *testing.T, files map[string]string, def func(root string) *ingitdb.Definition) []ingitdb.ValidationError {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)
	res, err := NewValidator().Validate(context.Background(), root, def(root))
	if err != nil {
		t.Fatal(err)
	}
	return res.Errors()
}

// Verifies record-key-encoding#ac:file-name-collisions-reported: record files
// of a collection without key_encoding whose names differ only in letter case
// are an error naming both files and the remedy; names that differ otherwise
// are not.
func TestValidate_FileNameCollision_TopLevelWithoutEncoding(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{
		"c/$records/Bob.yaml":   "title: a\n",
		"c/$records/bob.yaml":   "title: b\n",
		"c/$records/bobby.yaml": "title: c\n",
		"c/$records/b0b.yaml":   "title: d\n",
		"c/$records/bob.yml":    "title: e\n", // another extension is another name
	}, plainDB)
	if len(errs) != 1 {
		t.Fatalf("want 1 finding, got %s", findingTexts(errs))
	}
	msg := errs[0].Message
	for _, want := range []string{"record files Bob.yaml, bob.yaml in ", "differ only in letter case",
		"the same file on case-insensitive file systems such as macOS and Windows", "rename all but one"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q: %s", want, msg)
		}
	}
	if errs[0].CollectionID != "c" || filepath.Base(errs[0].FilePath) != "Bob.yaml" {
		t.Errorf("finding at %s %s", errs[0].CollectionID, errs[0].FilePath)
	}
}

// Sub-collection record files and sub-collection parent directories collide
// without a key_encoding too.
func TestValidate_FileNameCollision_SubCollectionWithoutEncoding(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{
		"c/$records/p1.yaml":                  "title: p\n",
		"c/$records/p1/notes/$records/N.yaml": "title: a\n",
		"c/$records/p1/notes/$records/n.yaml": "title: b\n",
		"c/$records/Pq/notes/$records/a.yaml": "title: a\n",
		"c/$records/pq/notes/$records/b.yaml": "title: b\n",
	}, plainDB)
	all := findingTexts(errs)
	if len(errs) != 2 ||
		!strings.Contains(all, "record files N.yaml, n.yaml in ") ||
		!strings.Contains(all, "parent directories Pq, pq in ") ||
		!strings.Contains(all, "the notes records of all of them are mixed") {
		t.Fatalf("got %s", all)
	}
	for _, e := range errs {
		if e.CollectionID != "c/notes" {
			t.Errorf("finding for %q, want c/notes", e.CollectionID)
		}
	}
}

// Records kept as directories ({key}/index.yaml) collide by directory name.
func TestValidate_FileNameCollision_RecordDirectoriesWithoutEncoding(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{
		"c/$records/Ab/index.yaml": "title: one\n",
		"c/$records/ab/index.yaml": "title: two\n",
	}, func(root string) *ingitdb.Definition {
		def := plainDB(root)
		def.Collections["c"].RecordFile.Name = "{key}/index.yaml"
		return def
	})
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "record files Ab/index.yaml, ab/index.yaml in ") {
		t.Fatalf("got %s", findingTexts(errs))
	}
}

// Names that differ only in Unicode normalisation (NFC "é" against NFD
// "e" + U+0301) are one file on macOS; the finding escapes the names, which
// look the same when printed.
func TestValidate_FileNameCollision_NFCAndNFD(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{
		"c/$records/caf\u00e9.yaml":  "title: nfc\n",
		"c/$records/cafe\u0301.yaml": "title: nfd\n",
		"c/$records/cafe.yaml":       "title: plain\n",
	}, plainDB)
	if len(errs) != 1 {
		t.Fatalf("want 1 finding, got %s", findingTexts(errs))
	}
	msg := errs[0].Message
	for _, want := range []string{`"cafe\u0301.yaml"`, `"caf\u00e9.yaml"`, "differ only in Unicode normalisation (NFC/NFD)", "the same file on macOS"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q: %s", want, msg)
		}
	}
}

// Three names of one file are one finding naming all three: they share one
// remedy (keep one name), and a finding per pair would repeat each file.
func TestValidate_FileNameCollision_ThreeWayIsOneGroup(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{
		"c/$records/BOB.yaml":            "title: a\n",
		"c/$records/Bob.yaml":            "title: b\n",
		"c/$records/bob.yaml":            "title: c\n",
		"c/$records/Caf\u00e9.yaml":      "title: d\n",
		"c/$records/cafe\u0301.yaml":     "title: e\n",
		"c/$records/unrelated-name.yaml": "title: f\n",
	}, plainDB)
	all := findingTexts(errs)
	if len(errs) != 2 ||
		!strings.Contains(all, "record files BOB.yaml, Bob.yaml, bob.yaml in ") ||
		!strings.Contains(all, "differ only in letter case and Unicode normalisation") {
		t.Fatalf("got %s", all)
	}
}

// Declared collection directories at one level, and sub-collection
// directories of one parent, collide like record names.
func TestValidate_FileNameCollision_CollectionDirectories(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{
		"data/Users/$records/u1.yaml": "title: a\n",
		"data/users/$records/u2.yaml": "title: b\n",
		"data/teams/$records/t1.yaml": "title: c\n",
	}, func(root string) *ingitdb.Definition {
		def := plainDB(root)
		rf := func() *ingitdb.RecordFileDef {
			return &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord}
		}
		str := map[string]*ingitdb.ColumnDef{"title": {Type: ingitdb.ColumnTypeString}}
		for _, id := range []string{"Users", "users", "teams"} {
			def.Collections[id] = &ingitdb.CollectionDef{ID: id, DirPath: filepath.Join(root, "data", id), RecordFile: rf(), Columns: str}
		}
		c := def.Collections["c"]
		c.SubCollections["Notes"] = &ingitdb.CollectionDef{ID: "Notes", RecordFile: rf(), Columns: str,
			SubCollections: map[string]*ingitdb.CollectionDef{"x": {ID: "x"}, "X": {ID: "X"}}}
		return def
	})
	all := findingTexts(errs)
	for _, want := range []string{
		"collection directories Users, users in ",
		"subcollections Notes, notes of c: their directories differ only in letter case",
		"subcollections X, x of c/Notes: their directories differ only in letter case",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("findings lack %q:\n%s", want, all)
		}
	}
	if len(errs) != 3 {
		t.Errorf("want 3 findings, got %s", all)
	}
}

// Verifies record-key-encoding#ac:file-name-collisions-reported for
// incremental validation: a file added next to one whose name differs only in
// letter case fails a collection without key_encoding, as the full pass does.
func TestIncrementalValidator_AddedCollidingFileWithoutEncoding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"c/$records/Bob.yaml": "title: a\n", "c/$records/bob.yaml": "title: b\n"})
	differ := fakeDiffer{files: []ingitdb.ChangedFile{{Kind: ingitdb.ChangeKindAdded, Path: "c/$records/bob.yaml"}}}
	res, err := NewIncrementalValidator(differ, NewChangeSetResolver(), NewValidator()).ValidateChanges(context.Background(), root, plainDB(root), "A", "B")
	if err != nil {
		t.Fatal(err)
	}
	errs := res.Errors()
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "Bob.yaml differ only in letter case") ||
		!strings.Contains(errs[0].Message, "rename one") {
		t.Fatalf("got %s", findingTexts(errs))
	}
	full, err := NewValidator().Validate(context.Background(), root, plainDB(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Errors()) != 1 {
		t.Errorf("full and incremental disagree: %s", findingTexts(full.Errors()))
	}
}

func TestCollisionReason(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		names  []string
		reason string
		shown  string
	}{
		{[]string{"Ab", "ab"}, "differ only in letter case", "Ab, ab"},
		{[]string{"caf\u00e9", "cafe\u0301"}, "differ only in Unicode normalisation (NFC/NFD)", `"caf\u00e9", "cafe\u0301"`},
		{[]string{"Caf\u00e9", "caf\u00e9", "cafe\u0301"}, "differ only in letter case and Unicode normalisation", `"Caf\u00e9", "caf\u00e9", "cafe\u0301"`},
	} {
		shown, reason := collisionReason(tc.names, "file")
		if !strings.HasPrefix(reason, tc.reason) || strings.Join(shown, ", ") != tc.shown {
			t.Errorf("%q: %q / %q", tc.names, shown, reason)
		}
	}
}

// A collection without a definition, or without a data directory, names no
// directory.
func TestCollectionDirCollisionFindings_NoDirectory(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
		"a": nil, "A": {ID: "A"}, "b": {ID: "b", SubCollections: map[string]*ingitdb.CollectionDef{"s": nil, "S": nil}},
	}}
	got := collectionDirCollisionFindings(def)
	if len(got) != 1 || got[0].CollectionID != "b/S" {
		t.Errorf("got %s", findingTexts(got))
	}
}

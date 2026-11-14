package datavalidator

// specscore: feature/record-key-encoding

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
)

func tildeDB(root string) *ingitdb.Definition {
	str := &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString}
	rf := func(enc string) *ingitdb.RecordFileDef {
		return &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord, KeyEncoding: enc}
	}
	notes := &ingitdb.CollectionDef{ID: "notes", RecordFile: rf(ingitdb.KeyEncodingTildeV1), Columns: map[string]*ingitdb.ColumnDef{"title": str}}
	spaces := &ingitdb.CollectionDef{ID: "spaces", DirPath: filepath.Join(root, "spaces"), RecordFile: rf(ingitdb.KeyEncodingTildeV1),
		Columns: map[string]*ingitdb.ColumnDef{"title": str}, SubCollections: map[string]*ingitdb.CollectionDef{"notes": notes}}
	refs := &ingitdb.CollectionDef{ID: "refs", DirPath: filepath.Join(root, "refs"), RecordFile: rf(""),
		Columns: map[string]*ingitdb.ColumnDef{"space": {Type: ingitdb.ColumnTypeString, ForeignKey: "spaces"}}}
	return &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"spaces": spaces, "refs": refs}}
}

// Verifies record-key-encoding#ac:readers-use-decoded-keys and
// record-key-encoding#ac:invalid-names-and-collisions-reported: validation
// reads tilde-v1 file and directory names as their decoded keys (foreign
// keys hold the real ids), and reports names the encoding never writes and
// names that differ only in letter case.
func TestValidate_KeyEncodingTildeV1(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"spaces/$records/.hidden.yaml":                 "title: 1\n", // hidden: not a record
		"spaces/$records/~43ON.yaml":                   "title: CON\n",
		"spaces/$records/a~2Fb.yaml":                   "title: a/b\n",
		"spaces/$records/bad~2f.yaml":                  "title: bad\n", // not canonical
		"spaces/$records/Ab.yaml":                      "title: Ab\n",
		"spaces/$records/ab.yaml":                      "title: ab\n", // case-only difference
		"spaces/$records/a~2Fb/notes/$records/n1.yaml": "title: n1\n",
		"spaces/$records/x~zz/notes/$records/n2.yaml":  "title: n2\n",     // parent name not canonical
		"refs/$records/r1.yaml":                        "space: a/b\n",    // the real id
		"refs/$records/r2.yaml":                        "space: a~2Fb\n",  // a file name is not a key
		"spaces/$records/Pq/notes/$records/a.yaml":     "title: a\n",      // parent directories that
		"spaces/$records/pq/notes/$records/b.yaml":     "title: b\n",      // differ only in case
		"refs/$records/r4.yaml":                        "space: bad~2f\n", // an invalid name is no record
		"refs/$records/r3.yaml":                        "space: CON\n",
	})
	res, err := NewValidator().Validate(context.Background(), root, tildeDB(root))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range res.Errors() {
		got = append(got, filepath.Base(e.FilePath)+": "+e.Message)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{
		`bad~2f.yaml: the file name is not one the collection's key_encoding writes`,
		`"bad~2f" is not a tilde-v1 name; the name of key "bad/" is "bad~2F"`,
		`Ab.yaml: record files Ab.yaml, ab.yaml in `,
		`notes: the parent directory name is not one the parent collection's key_encoding writes`,
		`foreign key "space" = "a~2Fb" has no matching record in collection "spaces"`,
		`foreign key "space" = "bad~2f" has no matching record in collection "spaces"`,
		`Pq: parent directories Pq, pq in `,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("findings lack %q:\n%s", want, all)
		}
	}
	if len(got) != 6 {
		t.Errorf("want 6 findings, got %d:\n%s", len(got), all)
	}
	if passed, total := res.GetRecordCounts("spaces/notes"); passed != 4 || total != 4 {
		t.Errorf("spaces/notes counts = %d/%d, want 4/4", passed, total)
	}
	var keys []string
	w := instanceWalk{root: root}
	w.walk("spaces", tildeDB(root).Collections["spaces"], func(inst subCollectionInstance) {
		keys = append(keys, inst.parentKey)
	})
	if strings.Join(keys, " ") != "Ab Pq ab a/b pq x~zz CON" {
		t.Errorf("parent keys = %q", keys)
	}
}

// Verifies record-key-encoding#ac:over-long-key-fails for validation.
func TestDecodedRecordKey_OverLongName(t *testing.T) {
	t.Parallel()
	def := tildeDB("/db").Collections["spaces"]
	name := strings.Repeat("x", 300)
	key, err := decodedRecordKey(def, "/db/spaces/$records/"+name+".yaml")
	if !errors.Is(err, ingitdb.ErrKeyNotStorable) || key != name {
		t.Errorf("got %q, %v", key[:10], err)
	}
	if key, err := decodedRecordKey(nil, "/x/a~2Fb.yaml"); err != nil || key != "a~2Fb" {
		t.Errorf("no definition: %q, %v", key, err)
	}
}

// A shared-file parent's instance directory is its encoded key, and a key
// too long for a directory name owns no instance.
func TestParentDirName(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{ID: "m", DirPath: "/db/m", RecordFile: &ingitdb.RecordFileDef{Name: "all.yaml", RecordType: ingitdb.MapOfRecords, KeyEncoding: ingitdb.KeyEncodingTildeV1}}
	if name, ok := parentDirName(colDef, loadedRecord{Key: "a/b"}); !ok || name != "a~2Fb" {
		t.Errorf("got %q, %v", name, ok)
	}
	if _, ok := parentDirName(colDef, loadedRecord{Key: strings.Repeat("/", 90)}); ok {
		t.Error("over-long key accepted")
	}
	if key, err := decodeParentName(&ingitdb.CollectionDef{}, "x~zz"); err != nil || key != "x~zz" {
		t.Errorf("no record file: %q, %v", key, err)
	}
}

// Records whose key names a directory ({key}/index.yaml) and differ only in
// letter case are reported too.
func TestValidate_KeyEncodingCaseCollisionInDirectoryNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"c/$records/Ab/index.yaml": "title: one\n",
		"c/$records/ab/index.yaml": "title: two\n",
	})
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"c": {ID: "c", DirPath: filepath.Join(root, "c"),
		RecordFile: &ingitdb.RecordFileDef{Name: "{key}/index.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord, KeyEncoding: ingitdb.KeyEncodingTildeV1},
		Columns:    map[string]*ingitdb.ColumnDef{"title": {Type: ingitdb.ColumnTypeString}}}}}
	res, err := NewValidator().Validate(context.Background(), root, def)
	if err != nil {
		t.Fatal(err)
	}
	if errs := res.Errors(); len(errs) != 1 || !strings.Contains(errs[0].Message, "record files Ab/index.yaml, ab/index.yaml in ") {
		t.Fatalf("got %s", findingTexts(errs))
	}
}

// Incremental validation reports a changed record file whose name, or a
// directory on its path, differs from an existing one only in letter case.
func TestChangedFileCaseFindings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"spaces/$records/Ab.yaml": "title: x\n", "spaces/$records/ab.yaml": "title: y\n"})
	def := tildeDB(root).Collections["spaces"]
	got := changedFileCaseFindings(root, "spaces", def, filepath.Join(root, "spaces", "$records", "ab.yaml"))
	if len(got) != 1 || !strings.Contains(got[0].Message, "Ab.yaml differ only in letter case") {
		t.Fatalf("got %+v", got)
	}
	if got := changedFileCaseFindings(root, "spaces", def, filepath.Join(root, "spaces", "$records", "zz.yaml")); got != nil {
		t.Errorf("no collision: %+v", got)
	}
	if got := changedFileCaseFindings(root, "refs", tildeDB(root).Collections["refs"], filepath.Join(root, "spaces", "$records", "ab.yaml")); got != nil {
		t.Errorf("no encoding: %+v", got)
	}
}

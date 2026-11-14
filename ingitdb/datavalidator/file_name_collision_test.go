package datavalidator

// specscore: feature/record-key-encoding

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	ingitdb "github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/gitdiff"
)

const fsClause = "may be the same file on case-insensitive or normalisation-insensitive file systems (for example macOS; Windows for letter case)"

func plainRecordFile() *ingitdb.RecordFileDef {
	return &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord}
}

func titleColumns() map[string]*ingitdb.ColumnDef {
	return map[string]*ingitdb.ColumnDef{"title": {Type: ingitdb.ColumnTypeString}}
}

// plainDB is a database whose collections declare no key_encoding: c keeps
// one record per {key}.yaml file and has the subcollections notes and tasks.
func plainDB(root string) *ingitdb.Definition {
	sub := func(id string) *ingitdb.CollectionDef {
		return &ingitdb.CollectionDef{ID: id, RecordFile: plainRecordFile(), Columns: titleColumns()}
	}
	c := &ingitdb.CollectionDef{ID: "c", DirPath: filepath.Join(root, "c"), RecordFile: plainRecordFile(), Columns: titleColumns(),
		SubCollections: map[string]*ingitdb.CollectionDef{"notes": sub("notes"), "tasks": sub("tasks")}}
	return &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"c": c}}
}

func fullFindings(t *testing.T, root string, def *ingitdb.Definition) []ingitdb.ValidationError {
	t.Helper()
	res, err := NewValidator().Validate(context.Background(), root, def)
	if err != nil {
		t.Fatal(err)
	}
	return res.Errors()
}

func validateTree(t *testing.T, files map[string]string, def func(root string) *ingitdb.Definition) []ingitdb.ValidationError {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)
	return fullFindings(t, root, def(root))
}

// incrementalFindings validates changed against the tree under root.
func incrementalFindings(t *testing.T, root string, def *ingitdb.Definition, changed ...ingitdb.ChangedFile) []ingitdb.ValidationError {
	t.Helper()
	res, err := NewIncrementalValidator(fakeDiffer{files: changed}, NewChangeSetResolver(), NewValidator()).ValidateChanges(context.Background(), root, def, "A", "B")
	if err != nil {
		t.Fatal(err)
	}
	return res.Errors()
}

func collisionsOnly(errs []ingitdb.ValidationError) []ingitdb.ValidationError {
	var out []ingitdb.ValidationError
	for _, e := range errs {
		if strings.HasPrefix(e.Message, "names ") || strings.HasPrefix(e.Message, "subcollections ") {
			out = append(out, e)
		}
	}
	return out
}

func added(p string) ingitdb.ChangedFile {
	return ingitdb.ChangedFile{Kind: ingitdb.ChangeKindAdded, Path: p}
}

// sameFindings fails unless the incremental collision findings are exactly
// the full pass's.
func sameFindings(t *testing.T, full, incremental []ingitdb.ValidationError) {
	t.Helper()
	f, i := collisionsOnly(full), collisionsOnly(incremental)
	if findingTexts(f) != findingTexts(i) || len(f) != len(i) {
		t.Fatalf("full and incremental disagree:\nfull: %s\nincremental: %s", findingTexts(f), findingTexts(i))
	}
	for n := range f {
		if f[n].CollectionID != i[n].CollectionID || f[n].FilePath != i[n].FilePath {
			t.Errorf("finding %d: full %s %s, incremental %s %s", n, f[n].CollectionID, f[n].FilePath, i[n].CollectionID, i[n].FilePath)
		}
	}
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
	for _, want := range []string{"names Bob.yaml, bob.yaml in ", "differ only in letter case, so they " + fsClause, "rename all but one"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q: %s", want, msg)
		}
	}
	if errs[0].CollectionID != "c" || filepath.Base(errs[0].FilePath) != "Bob.yaml" {
		t.Errorf("finding at %s %s", errs[0].CollectionID, errs[0].FilePath)
	}
}

// Every entry of a records directory counts, not only names the record
// pattern matches: bob.YAML is one file with Bob.yaml where case is ignored.
func TestValidate_FileNameCollision_AnyEntryOfARecordsDirectory(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{"c/$records/Bob.yaml": "title: a\n", "c/$records/bob.YAML": "x\n"}, plainDB)
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "names Bob.yaml, bob.YAML in ") {
		t.Fatalf("got %s", findingTexts(errs))
	}
}

// Sub-collection record files and sub-collection parent directories collide
// without a key_encoding too.
func TestValidate_FileNameCollision_SubCollectionWithoutEncoding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"c/$records/p1.yaml":                  "title: p\n",
		"c/$records/p1/notes/$records/N.yaml": "title: a\n",
		"c/$records/p1/notes/$records/n.yaml": "title: b\n",
		"c/$records/Pq/notes/$records/a.yaml": "title: a\n",
		"c/$records/pq/notes/$records/b.yaml": "title: b\n",
	})
	errs := fullFindings(t, root, plainDB(root))
	if len(errs) != 2 ||
		errs[1].Message != "names N.yaml, n.yaml in "+filepath.Join(root, "c/$records/p1/notes/$records")+" differ only in letter case, so they "+fsClause+"; a checkout there keeps only one of them and loses the others' data: rename all but one" ||
		errs[1].CollectionID != "c/notes" ||
		!strings.HasPrefix(errs[0].Message, "names Pq, pq in "+filepath.Join(root, "c/$records")+" ") || errs[0].CollectionID != "c" {
		t.Fatalf("got %s", findingTexts(errs))
	}
}

// Parent directories are grouped per directory, whichever subcollections
// they hold: Pq/notes and pq/tasks collide, and Rs and rs are one finding
// although both hold notes.
func TestValidate_FileNameCollision_ParentsGroupedByDirectory(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{
		"c/$records/Pq/notes/$records/a.yaml": "title: a\n",
		"c/$records/pq/tasks/$records/b.yaml": "title: b\n",
		"c/$records/Rs/notes/$records/a.yaml": "title: a\n",
		"c/$records/Rs/tasks/$records/a.yaml": "title: a\n",
		"c/$records/rs/notes/$records/b.yaml": "title: b\n",
	}, plainDB)
	all := findingTexts(errs)
	if len(errs) != 2 || !strings.Contains(all, "names Pq, pq in ") || !strings.Contains(all, "names Rs, rs in ") {
		t.Fatalf("got %s", all)
	}
}

// Records kept as directories ({key}/index.yaml) collide by directory name.
func TestValidate_FileNameCollision_RecordDirectoriesWithoutEncoding(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{
		"c/$records/Ab/index.yaml": "title: one\n",
		"c/$records/ab/index.yaml": "title: two\n",
	}, indexDB)
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "names Ab, ab in ") || !strings.Contains(errs[0].Message, "differ only in letter case, so") {
		t.Fatalf("got %s", findingTexts(errs))
	}
}

func indexDB(root string) *ingitdb.Definition {
	def := plainDB(root)
	def.Collections["c"].RecordFile.Name = "{key}/index.yaml"
	return def
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
	if msg := errs[0].Message; !strings.Contains(msg, `names "cafe\u0301.yaml", "caf\u00e9.yaml" in `) || !strings.Contains(msg, "differ only in Unicode normalisation, so they "+fsClause) {
		t.Errorf("got %s", msg)
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
		!strings.Contains(all, "names BOB.yaml, Bob.yaml, bob.yaml in ") ||
		!strings.Contains(all, "differ only in letter case and Unicode normalisation, so") {
		t.Fatalf("got %s", all)
	}
}

// A name with a control character is shown escaped.
func TestValidate_FileNameCollision_UnsafeNamesEscaped(t *testing.T) {
	t.Parallel()
	errs := validateTree(t, map[string]string{"c/$records/a\tb.yaml": "title: a\n", "c/$records/A\tb.yaml": "title: b\n"}, plainDB)
	if len(errs) != 1 || !strings.Contains(errs[0].Message, `names "A\tb.yaml", "a\tb.yaml" in `) {
		t.Fatalf("got %s", findingTexts(errs))
	}
}

// collectionDirsDB declares root collections at data/Users, data/users,
// Data/teams, solo/A (not on disk) and solo/a, and subcollections Notes and
// notes of c, and X and x of c/Notes.
func collectionDirsDB(root string) *ingitdb.Definition {
	def := plainDB(root)
	for id, dir := range map[string]string{"Users": "data/Users", "users": "data/users", "teams": "Data/teams", "soloA": "solo/A", "soloa": "solo/a"} {
		def.Collections[id] = &ingitdb.CollectionDef{ID: id, DirPath: filepath.Join(root, dir), RecordFile: plainRecordFile(), Columns: titleColumns()}
	}
	c := def.Collections["c"]
	c.SubCollections["Notes"] = &ingitdb.CollectionDef{ID: "Notes", RecordFile: plainRecordFile(), Columns: titleColumns(),
		SubCollections: map[string]*ingitdb.CollectionDef{"x": {ID: "x"}, "X": {ID: "X"}}}
	return def
}

// Collection directories collide at any level of their path, compared under
// the same parent directory (Data against data, Users against users), and
// whether both are on disk or only declared; subcollection ids of one
// collection collide in the definition.
func TestValidate_FileNameCollision_CollectionDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"data/Users/$records/u1.yaml": "title: a\n",
		"data/users/$records/u2.yaml": "title: b\n",
		"Data/teams/$records/t1.yaml": "title: c\n",
		"solo/a/$records/s1.yaml":     "title: d\n",
	})
	errs := fullFindings(t, root, collectionDirsDB(root))
	all := findingTexts(errs)
	for _, want := range []string{
		"names Data, data in " + root + " differ only in letter case",
		"names Users, users in " + filepath.Join(root, "data") + " ",
		"names A, a in " + filepath.Join(root, "solo") + " ",
		"subcollections Notes, notes of c differ only in letter case, so under every parent record their directories may be one directory",
		"subcollections X, x of c/Notes differ only in letter case",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("findings lack %q:\n%s", want, all)
		}
	}
	if len(errs) != 5 {
		t.Errorf("want 5 findings, got %s", all)
	}
}

// Findings come in one order on every run: collisions sorted by directory,
// after the per-collection findings in collection order.
func TestValidate_FileNameCollision_DeterministicOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	for _, id := range []string{"e", "d", "c", "b", "a"} {
		files[id+"/$records/X.yaml"] = "title: x\n"
		files[id+"/$records/x.yaml"] = "title: x\n"
		files[id+"/$records/bad.yaml"] = "title: [\n"
		def.Collections[id] = &ingitdb.CollectionDef{ID: id, DirPath: filepath.Join(root, id), RecordFile: plainRecordFile(), Columns: titleColumns()}
	}
	writeTree(t, root, files)
	first := findingTexts(fullFindings(t, root, def))
	for range 5 {
		if got := findingTexts(fullFindings(t, root, def)); got != first {
			t.Fatalf("order changed:\n%s\n--\n%s", first, got)
		}
	}
	errs := fullFindings(t, root, def)
	if errs[0].CollectionID != "a" || !strings.HasPrefix(errs[5].Message, "names X.yaml, x.yaml in "+filepath.Join(root, "a")) {
		t.Errorf("not sorted: %s", first)
	}
}

// countingReadDir lists directories through os.ReadDir and counts each.
func countingReadDir(counts map[string]int) func(string) ([]os.DirEntry, error) {
	return func(dir string) ([]os.DirEntry, error) {
		counts[dir]++
		return os.ReadDir(dir)
	}
}

// One validation call lists each directory at most once for name
// collisions, however many records or changed files it holds.
func TestNameIndex_ListsEachDirectoryOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{"c/$records/Pq/notes/$records/a.yaml": "title: a\n", "c/$records/pq/tasks/$records/b.yaml": "title: b\n"}
	var changed []ingitdb.ChangedFile
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		files["c/$records/"+k+".yaml"] = "title: x\n"
		files["c/$records/"+k+"/notes/$records/n.yaml"] = "title: x\n"
		changed = append(changed, added("c/$records/"+k+".yaml"), added("c/$records/"+k+"/notes/$records/n.yaml"))
	}
	files["c/$records/A.yaml"] = "title: x\n"
	writeTree(t, root, files)
	def := plainDB(root)

	fullCounts := map[string]int{}
	res, err := (&simpleValidator{readDir: countingReadDir(fullCounts)}).Validate(context.Background(), root, def)
	if err != nil {
		t.Fatal(err)
	}
	incCounts := map[string]int{}
	iv := &incrementalValidator{differ: fakeDiffer{files: changed}, resolver: NewChangeSetResolver(), full: NewValidator(), readDir: countingReadDir(incCounts)}
	inc, err := iv.ValidateChanges(context.Background(), root, def, "A", "B")
	if err != nil {
		t.Fatal(err)
	}
	for name, counts := range map[string]map[string]int{"full": fullCounts, "incremental": incCounts} {
		if counts[filepath.Join(root, "c", "$records")] != 1 {
			t.Errorf("%s: records directory listed %d times", name, counts[filepath.Join(root, "c", "$records")])
		}
		for dir, n := range counts {
			if n > 1 {
				t.Errorf("%s: %s listed %d times", name, dir, n)
			}
		}
	}
	if len(collisionsOnly(res.Errors())) != 2 || len(collisionsOnly(inc.Errors())) != 1 {
		t.Errorf("full %s\nincremental %s", findingTexts(res.Errors()), findingTexts(inc.Errors()))
	}
}

// Verifies record-key-encoding#ac:file-name-collisions-reported for
// incremental validation: a file added next to names it collides with is
// reported with the whole group, exactly as the full pass reports it, once
// however many of the group changed.
func TestIncrementalValidator_FileNameCollision_SameGroupAsFull(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"c/$records/BOB.yaml": "title: a\n", "c/$records/Bob.yaml": "title: b\n", "c/$records/bob.yaml": "title: c\n"})
	def := plainDB(root)
	full := fullFindings(t, root, def)
	sameFindings(t, full, incrementalFindings(t, root, def, added("c/$records/bob.yaml")))
	sameFindings(t, full, incrementalFindings(t, root, def, added("c/$records/bob.yaml"), added("c/$records/Bob.yaml"), added("c/$records/BOB.yaml")))
	if len(full) != 1 || !strings.Contains(full[0].Message, "names BOB.yaml, Bob.yaml, bob.yaml in ") {
		t.Errorf("full: %s", findingTexts(full))
	}
}

// A record added to a sub-collection instance is resolved to that instance:
// it is validated, and its name collisions are the full pass's.
func TestIncrementalValidator_SubCollectionRecords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"c/$records/p1.yaml":                  "title: p\n",
		"c/$records/p1/notes/$records/N.yaml": "title: a\n",
		"c/$records/p1/notes/$records/n.yaml": "title: [\n",
		"c/$records/Pq/tasks/$records/a.yaml": "title: a\n",
		"c/$records/pq/notes/$records/b.yaml": "title: b\n",
	})
	def := plainDB(root)
	inc := incrementalFindings(t, root, def, added("c/$records/p1/notes/$records/n.yaml"), added("c/$records/pq/notes/$records/b.yaml"))
	sameFindings(t, fullFindings(t, root, def), inc)
	all := findingTexts(inc)
	if !strings.Contains(all, "failed to parse record file") || len(collisionsOnly(inc)) != 2 {
		t.Errorf("got %s", all)
	}
}

// A collision of a record directory is reported for the directory segment
// only, with the reason the full pass gives.
func TestIncrementalValidator_RecordDirectoryCollision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"c/$records/Ab/index.yaml": "title: one\n", "c/$records/ab/index.yaml": "title: two\n"})
	def := indexDB(root)
	inc := incrementalFindings(t, root, def, added("c/$records/ab/index.yaml"))
	sameFindings(t, fullFindings(t, root, def), inc)
	if len(inc) != 1 || !strings.Contains(inc[0].Message, "names Ab, ab in ") || strings.Contains(inc[0].Message, "normalisation, so") {
		t.Errorf("got %s", findingTexts(inc))
	}
}

// A deletion that leaves two names of one file is still reported; a rename
// that leaves one is not.
func TestIncrementalValidator_DeletionsAndRenames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"c/$records/BOB.yaml": "title: a\n", "c/$records/bob.yaml": "title: c\n", "d/$records/new.yaml": "title: n\n"})
	def := plainDB(root)
	def.Collections["d"] = &ingitdb.CollectionDef{ID: "d", DirPath: filepath.Join(root, "d"), RecordFile: plainRecordFile(), Columns: titleColumns()}
	inc := incrementalFindings(t, root, def, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindDeleted, Path: "c/$records/Bob.yaml"})
	sameFindings(t, fullFindings(t, root, def), inc)
	if len(inc) != 1 {
		t.Errorf("deletion: %s", findingTexts(inc))
	}
	renamed := incrementalFindings(t, root, def, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindRenamed, OldPath: "d/$records/New.yaml", Path: "d/$records/new.yaml"},
		ingitdb.ChangedFile{Kind: ingitdb.ChangeKindDeleted, Path: "elsewhere/x.yaml"})
	if len(renamed) != 0 {
		t.Errorf("rename: %s", findingTexts(renamed))
	}
}

// A changed record below a parent directory that names no parent record is
// reported once, as the full pass reports it.
func TestIncrementalValidator_ParentNameNotEncoded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"spaces/$records/x~zz/notes/$records/n1.yaml": "title: a\n",
		"spaces/$records/x~zz/notes/$records/n2.yaml": "title: b\n",
	})
	def := tildeDB(root)
	inc := incrementalFindings(t, root, def, added("spaces/$records/x~zz/notes/$records/n1.yaml"), added("spaces/$records/x~zz/notes/$records/n2.yaml"))
	full := fullFindings(t, root, def)
	if len(inc) != 1 || len(full) != 1 || inc[0].Message != full[0].Message || inc[0].CollectionID != "spaces/notes" {
		t.Errorf("incremental %s\nfull %s", findingTexts(inc), findingTexts(full))
	}
}

// A shared records file of a sub-collection instance is validated once per
// instance however many of its changes are listed.
func TestIncrementalValidator_SharedFileSubCollection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"c/$records/p1/tags/all.yaml": "t1:\n  title: a\n",
		"c/$records/p2/tags/all.yaml": "t2:\n  title: b\nt3:\n  title: c\n",
	})
	def := plainDB(root)
	def.Collections["c"].SubCollections["tags"] = &ingitdb.CollectionDef{ID: "tags", Columns: titleColumns(),
		RecordFile: &ingitdb.RecordFileDef{Name: "all.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.MapOfRecords}}
	res, err := NewIncrementalValidator(fakeDiffer{files: []ingitdb.ChangedFile{
		added("c/$records/p1/tags/all.yaml"), added("c/$records/p2/tags/all.yaml"), added("c/$records/p2/tags/all.yaml"),
	}}, NewChangeSetResolver(), NewValidator()).ValidateChanges(context.Background(), root, def, "A", "B")
	if err != nil {
		t.Fatal(err)
	}
	if passed, total := res.GetRecordCounts("c/tags"); passed != 3 || total != 3 || res.HasErrors() {
		t.Errorf("c/tags = %d/%d: %s", passed, total, findingTexts(res.Errors()))
	}
}

// End to end with git: an NFD name added next to an NFC one arrives
// unescaped from git diff and fails incremental validation as it fails the
// full pass.
func TestIncrementalValidator_RealGitNormalisationCollision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "T")
	writeTree(t, root, map[string]string{"c/$records/caf\u00e9.yaml": "title: nfc\n"})
	git("add", ".")
	git("commit", "-m", "base")
	base := git("rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"c/$records/cafe\u0301.yaml": "title: nfd\n"})
	git("add", ".")
	git("commit", "-m", "nfd")
	def := plainDB(root)
	res, err := NewIncrementalValidator(gitdiff.NewGitDiffer(), NewChangeSetResolver(), NewValidator()).ValidateChanges(context.Background(), root, def, base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	sameFindings(t, fullFindings(t, root, def), res.Errors())
	if len(res.Errors()) != 1 || !strings.Contains(res.Errors()[0].Message, "Unicode normalisation") {
		t.Errorf("got %s", findingTexts(res.Errors()))
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
		{[]string{"caf\u00e9", "cafe\u0301"}, "differ only in Unicode normalisation", `"caf\u00e9", "cafe\u0301"`},
		{[]string{"Caf\u00e9", "caf\u00e9", "cafe\u0301"}, "differ only in letter case and Unicode normalisation", `"Caf\u00e9", "caf\u00e9", "cafe\u0301"`},
		{[]string{"a\xff", "A\xff"}, "differ only in letter case", `"a\xff", "A\xff"`},
		{[]string{"K.yaml", "\u212a.yaml"}, "differ only in Unicode normalisation", `"K.yaml", "\u212a.yaml"`},
		{[]string{"\u03a9", "\u2126"}, "differ only in Unicode normalisation", `"\u03a9", "\u2126"`},
		{[]string{"\u00c5", "\u212b"}, "differ only in Unicode normalisation", `"\u00c5", "\u212b"`},
	} {
		shown, reason := collisionReason(tc.names)
		if reason != tc.reason || strings.Join(shown, ", ") != tc.shown {
			t.Errorf("%q: %q / %q", tc.names, shown, reason)
		}
	}
}

// A collection without a definition, or without a data directory, names no
// directory; an index without a root registers only the last segment.
func TestNameIndex_Edges(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
		"a": nil, "A": {ID: "A"}, "b": {ID: "b", SubCollections: map[string]*ingitdb.CollectionDef{"s": nil, "S": nil}},
	}}
	idx := newNameIndex("", nil)
	got := definitionNameFindings(idx, def)
	if len(got) != 1 || got[0].CollectionID != "b/S" || len(idx.findings()) != 0 {
		t.Errorf("got %s", findingTexts(got))
	}
	root := t.TempDir()
	writeTree(t, root, map[string]string{"x/A": "", "x/a": ""})
	idx = newNameIndex(root, nil)
	idx.usePath("c", root)                          // the root itself: nothing to check
	idx.usePath("c", filepath.Dir(root))            // outside the root: its last segment only
	idx.usePath("c", filepath.Join(root, "x", "a")) // below it: every segment
	if f := idx.findings(); len(f) != 1 || f[0].CollectionID != "c" {
		t.Errorf("got %s", findingTexts(f))
	}
}

// Two collections keeping their shared files in one data directory are each
// validated: a change to both validates both files.
func TestIncrementalValidator_SharedFilesInOneDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"data/a.yaml": "a1:\n  title: a\n", "data/b.yaml": "b1:\n  title: [\n"})
	shared := func(id, name string) *ingitdb.CollectionDef {
		return &ingitdb.CollectionDef{ID: id, DirPath: filepath.Join(root, "data"), Columns: titleColumns(),
			RecordFile: &ingitdb.RecordFileDef{Name: name, Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.MapOfRecords}}
	}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"a": shared("a", "a.yaml"), "b": shared("b", "b.yaml")}}
	inc := incrementalFindings(t, root, def, added("data/a.yaml"), added("data/b.yaml"))
	full := fullFindings(t, root, def)
	if len(inc) != 1 || len(full) != 1 || inc[0].CollectionID != "b" || findingTexts(inc) != findingTexts(full) {
		t.Errorf("incremental %s\nfull %s", findingTexts(inc), findingTexts(full))
	}
}

// Incremental validation reaches a subcollection instance only below a
// parent full validation walks: a parent directory name the parent-directory
// rule accepts, or the name of a parent record (a record may have any name,
// "$records" and "$views" included).
func TestIncrementalValidator_ParentsFullValidationWalks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"c/$records/.hidden/notes/$records/n.yaml": "title: [\n", // hidden: not a parent
		"c/$records/$views/notes/$records/n.yaml":  "title: [\n", // reserved, no record: not a parent
		"d/$records.yaml":                          "title: d\n", // a record named $records ...
		"d/$records/notes/$records/n.yaml":         "title: [\n", // ... is a parent
		"m/all.yaml":                               "$views:\n  title: m\n",
		"m/$views/notes/$records/n.yaml":           "title: [\n", // a shared-file record named $views
		"m/$records/notes/$records/n.yaml":         "title: [\n", // no such record: not a parent
	})
	def := plainDB(root)
	dot := "."
	notes := func() map[string]*ingitdb.CollectionDef {
		return map[string]*ingitdb.CollectionDef{"notes": {ID: "notes", RecordFile: plainRecordFile(), Columns: titleColumns()}}
	}
	def.Collections["d"] = &ingitdb.CollectionDef{ID: "d", DirPath: filepath.Join(root, "d"), Columns: titleColumns(), SubCollections: notes(),
		RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord, RecordsDir: &dot}}
	def.Collections["m"] = &ingitdb.CollectionDef{ID: "m", DirPath: filepath.Join(root, "m"), Columns: titleColumns(), SubCollections: notes(),
		RecordFile: &ingitdb.RecordFileDef{Name: "all.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.MapOfRecords}}
	var changed []ingitdb.ChangedFile
	for _, p := range []string{"c/$records/.hidden/notes/$records/n.yaml", "c/$records/$views/notes/$records/n.yaml", "d/$records/notes/$records/n.yaml", "m/$views/notes/$records/n.yaml", "m/$records/notes/$records/n.yaml"} {
		changed = append(changed, added(p))
	}
	inc := incrementalFindings(t, root, def, changed...)
	full := fullFindings(t, root, def)
	if findingTexts(inc) != findingTexts(full) || len(inc) != 2 || inc[0].CollectionID != "d/notes" || inc[1].CollectionID != "m/notes" {
		t.Errorf("incremental %s\nfull %s", findingTexts(inc), findingTexts(full))
	}
}

// A parent directory whose name holds glob characters is a literal name for
// both passes: its records are validated and its collisions found.
func TestIncrementalValidator_GlobCharactersInParentNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"c/$records/[p]/notes/$records/N.yaml":   "title: a\n",
		"c/$records/[p]/notes/$records/n.yaml":   "title: b\n",
		"c/$records/[p]/notes/$records/bad.yaml": "title: [\n",
		"c/$records/p/notes/$records/ok.yaml":    "title: c\n",
		"c/$records/a*?/notes/$records/bad.yaml": "title: [\n",
	})
	def := plainDB(root)
	inc := incrementalFindings(t, root, def, added("c/$records/[p]/notes/$records/n.yaml"), added("c/$records/[p]/notes/$records/bad.yaml"),
		added("c/$records/a*?/notes/$records/bad.yaml"))
	full := fullFindings(t, root, def)
	all := findingTexts(full)
	if findingTexts(inc) != all || len(full) != 3 || strings.Count(all, "failed to parse record file") != 2 || !strings.Contains(all, "names N.yaml, n.yaml in ") {
		t.Errorf("incremental %s\nfull %s", findingTexts(inc), all)
	}
	if passed, total := fullCounts(t, root, def, "c/notes"); passed != 3 || total != 5 {
		t.Errorf("c/notes = %d/%d", passed, total)
	}
}

func fullCounts(t *testing.T, root string, def *ingitdb.Definition, id string) (int, int) {
	t.Helper()
	res, err := NewValidator().Validate(context.Background(), root, def)
	if err != nil {
		t.Fatal(err)
	}
	return res.GetRecordCounts(id)
}

// A changed entry of a records directory that is not a record file still
// counts for collisions, as every entry does in the full pass; changes
// outside any records directory do not.
func TestIncrementalValidator_NonRecordEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"c/$records/Bob.yaml":                    "title: a\n",
		"c/$records/bob.YAML":                    "x\n",
		"c/$records/p1/notes/$records/README.md": "x\n",
		"c/$records/p1/notes/$records/Readme.md": "x\n",
		"c/$records/p1/notes/$records/n.yaml":    "title: n\n",
		"c/README.md":                            "x\n",
		"c/readme.md":                            "x\n",
	})
	def := plainDB(root)
	inc := incrementalFindings(t, root, def, added("c/$records/bob.YAML"), added("c/$records/p1/notes/$records/Readme.md"), added("c/readme.md"))
	sameFindings(t, fullFindings(t, root, def), inc)
	if len(inc) != 2 {
		t.Errorf("got %s", findingTexts(inc))
	}
}

// Directory names are literal in every pass, whatever bytes they hold: glob
// characters, a backslash (an escape character in a glob on Linux and
// macOS), invalid UTF-8, and a checkout path holding glob characters. Full
// and incremental validation find the same records and the same errors.
func TestRecordFiles_DirectoryNamesAreLiteral(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "proj[1]*?")
	parents := []string{"[p]", "a*?", "c\xff"}
	if filepath.Separator == '/' {
		parents = append(parents, `p\q`)
	}
	files := map[string]string{"c/$records/ok.yaml": "title: a\n", "c/$records/bad.yaml": "title: [\n"}
	var changed []ingitdb.ChangedFile
	for _, p := range parents {
		files["c/$records/"+p+"/notes/$records/n.yaml"] = "title: n\n"
		files["c/$records/"+p+"/notes/$records/bad.yaml"] = "title: [\n"
		changed = append(changed, added("c/$records/"+p+"/notes/$records/n.yaml"), added("c/$records/"+p+"/notes/$records/bad.yaml"))
	}
	changed = append(changed, added("c/$records/ok.yaml"), added("c/$records/bad.yaml"))
	writeTree(t, root, files)
	def := plainDB(root)
	full := fullFindings(t, root, def)
	inc := incrementalFindings(t, root, def, changed...)
	if len(full) != len(parents)+1 || len(inc) != len(full) {
		t.Errorf("want %d errors each:\nfull %s\nincremental %s", len(parents)+1, findingTexts(full), findingTexts(inc))
	}
	if passed, total := fullCounts(t, root, def, "c/notes"); passed != len(parents) || total != 2*len(parents) {
		t.Errorf("c/notes = %d/%d", passed, total)
	}
}

// record_file.name keeps its glob semantics below the literal records
// directory, for record files and for the parent records that own
// subcollection instances: a record .hidden/index.yaml under
// "{key}/[i]ndex.yaml" owns the instance below .hidden in both passes.
func TestRecordFiles_NamePatternKeepsGlobSemantics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"c/$records/.hidden/index.yaml":               "title: h\n",
		"c/$records/.hidden/notes/$records/bad.yaml":  "title: [\n",
		"c/$records/.missing/notes/$records/bad.yaml": "title: [\n", // no record: no instance
		"c/$records/plain/index.yaml":                 "title: [\n",
	})
	def := plainDB(root)
	def.Collections["c"].RecordFile.Name = "{key}/[i]ndex.yaml"
	full := fullFindings(t, root, def)
	inc := incrementalFindings(t, root, def, added("c/$records/.hidden/notes/$records/bad.yaml"), added("c/$records/.missing/notes/$records/bad.yaml"),
		added("c/$records/plain/index.yaml"))
	if len(full) != 2 || findingTexts(full) != findingTexts(inc) {
		t.Errorf("full %s\nincremental %s", findingTexts(full), findingTexts(inc))
	}
}

// The matcher compares path segments below the records directory only, so
// no part of the directory (a Windows volume such as \\server\share[x] or
// \\?\C:\, glob characters, any bytes) is ever read as a pattern; it splits
// on the separator it is given, so Windows paths are checked here on Linux.
func TestMatchRecordPath(t *testing.T) {
	t.Parallel()
	_, segments := recordPattern(&ingitdb.CollectionDef{RecordFile: &ingitdb.RecordFileDef{Name: "{key}/[i]ndex.yaml"}})
	for rel, want := range map[string]bool{
		`a\index.yaml`:        true,
		`[x]\index.yaml`:      true,
		`a\b\index.yaml`:      false,
		`a\other.yaml`:        false,
		`..\index.yaml`:       false,
		`index.yaml`:          false,
		`\\?\C:\a\index.yaml`: false, // a volume is never below the records directory
	} {
		if got := matchRecordPath(segments, rel, `\`); got != want {
			t.Errorf("%q: %v, want %v", rel, got, want)
		}
	}
	if matchRecordPath([]string{"[a-"}, "x", "/") {
		t.Error("a malformed pattern matches nothing")
	}
}

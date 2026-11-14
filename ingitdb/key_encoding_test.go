package ingitdb

// specscore: feature/record-key-encoding

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dalrecord "github.com/dal-go/record"
	"golang.org/x/text/unicode/norm"
)

// Verifies record-key-encoding#ac:tilde-v1-examples.
func TestTildeV1_Examples(t *testing.T) {
	t.Parallel()
	for key, want := range map[string]string{
		"abc":                   "abc",
		"Firestore-Auto_Id09":   "Firestore-Auto_Id09",
		"a.b@c+d=e,f":           "a.b@c+d=e,f",
		"a/b":                   "a~2Fb",
		`a\b`:                   "a~5Cb",
		"~":                     "~7E",
		".":                     "~2E",
		"..":                    "~2E~2E",
		".hidden":               "~2Ehidden",
		"trailing.":             "trailing~2E",
		"trailing ":             "trailing~20",
		"a b":                   "a~20b",
		"k3v9q:agr-swim-2026":   "k3v9q~3Aagr-swim-2026",
		"$records":              "~24records",
		"%41":                   "~2541",
		"CON":                   "~43ON",
		"con.txt":               "~63on.txt",
		"Lpt9":                  "~4Cpt9",
		"COM1.a.b":              "~43OM1.a.b",
		"COMX":                  "COMX",
		"CONSOLE":               "CONSOLE",
		"é":                     "~C3~A9",    // é, NFC
		"é":                    "e~CC~81",   // é, NFD: another key, another name
		"\x00\x1f\x7f":          "~00~1F~7F", // control bytes
		"<>:\"|?*":              "~3C~3E~3A~22~7C~3F~2A",
		"\xff":                  "~FF", // not UTF-8: still a key
		"10:00":                 "10~3A00",
		"datatug@demo-project1": "datatug@demo-project1",
	} {
		got, err := EncodeRecordKey(KeyEncodingTildeV1, key)
		if err != nil || got != want {
			t.Errorf("encode %q = %q, %v; want %q", key, got, err, want)
			continue
		}
		back, err := DecodeRecordKey(KeyEncodingTildeV1, got)
		if err != nil || back != key {
			t.Errorf("decode %q = %q, %v; want %q", got, back, err, key)
		}
	}
}

// Verifies record-key-encoding#ac:tilde-v1-one-name-per-key: a name the
// encoding never writes is rejected, with the name it would write.
func TestTildeV1_RejectsOtherNames(t *testing.T) {
	t.Parallel()
	for name, canonical := range map[string]string{
		"a~2fb":  "a~2Fb", // lower-case hex
		"a~2":    "a~7E2", // incomplete escape
		"~":      "~7E",
		"~ZZ":    "~7EZZ",
		"a~41":   "aA", // escapes a byte that is kept
		"a b":    "a~20b",
		".x":     "~2Ex",
		"x.":     "x~2E",
		"CON":    "~43ON",
		"~overf": "~7Eoverf",
	} {
		_, err := DecodeRecordKey(KeyEncodingTildeV1, name)
		if !errors.Is(err, ErrInvalidKeyName) || !strings.Contains(err.Error(), `is "`+canonical+`"`) {
			t.Errorf("decode %q: got %v, want ErrInvalidKeyName naming %q", name, err, canonical)
		}
	}
	if _, err := DecodeRecordKey(KeyEncodingTildeV1, ""); !errors.Is(err, ErrInvalidKeyName) {
		t.Errorf("empty name: %v", err)
	}
	if _, err := EncodeRecordKey(KeyEncodingTildeV1, ""); !errors.Is(err, ErrKeyNotStorable) {
		t.Errorf("empty key: %v", err)
	}
}

// Verifies record-key-encoding#ac:tilde-v1-round-trip: every key, hostile
// ones included, has a safe name that decodes back to it, and no two keys
// share a name.
func TestTildeV1_RoundTripProperty(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(20261114))
	alphabet := []byte("aZ09-_.@+=,~/\\:%$ \t\x00\x7f\xc3\xa9\xccCONPRAUXLT")
	names := map[string]string{}
	for i := 0; i < 20000; i++ {
		b := make([]byte, 1+rng.Intn(12))
		for j := range b {
			if rng.Intn(4) == 0 {
				b[j] = byte(rng.Intn(256))
			} else {
				b[j] = alphabet[rng.Intn(len(alphabet))]
			}
		}
		key := string(b)
		name, err := EncodeRecordKey(KeyEncodingTildeV1, key)
		if err != nil {
			t.Fatal(err)
		}
		if other, seen := names[name]; seen && other != key {
			t.Fatalf("keys %q and %q share the name %q", other, key, name)
		}
		names[name] = key
		if back, err := DecodeRecordKey(KeyEncodingTildeV1, name); err != nil || back != key {
			t.Fatalf("round trip of %q via %q gave %q, %v", key, name, back, err)
		}
		assertSafeName(t, key, name)
	}
}

func FuzzTildeV1(f *testing.F) {
	for _, seed := range []string{"a", "..", "CON", "~", "a/b", "\xff", "x. "} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, key string) {
		name, err := EncodeRecordKey(KeyEncodingTildeV1, key)
		if key == "" {
			if err == nil {
				t.Fatal("empty key encoded")
			}
			return
		}
		if back, err := DecodeRecordKey(KeyEncodingTildeV1, name); err != nil || back != key {
			t.Fatalf("round trip of %q via %q gave %q, %v", key, name, back, err)
		}
		assertSafeName(t, key, name)
	})
}

// assertSafeName checks that name can be a file name on Linux, macOS and
// Windows without being hidden, reinterpreted or altered.
func assertSafeName(t *testing.T, key, name string) {
	t.Helper()
	for i := 0; i < len(name); i++ {
		if c := name[i]; c != '~' && !tildeV1Keeps(c) {
			t.Fatalf("name %q of %q holds byte %q", name, key, c)
		}
	}
	if name == "." || name == ".." || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		t.Fatalf("name %q of %q is hidden, a directory alias or ends in a dot", name, key)
	}
	if windowsDeviceName(name) {
		t.Fatalf("name %q of %q is a Windows device name", name, key)
	}
}

func TestKeyEncoding_NoneAndUnknown(t *testing.T) {
	t.Parallel()
	for _, enc := range []string{"", KeyEncodingNone} {
		if got, err := EncodeRecordKey(enc, "a b"); err != nil || got != "a b" {
			t.Errorf("encode with %q: %q, %v", enc, got, err)
		}
		if got, err := DecodeRecordKey(enc, "a~20b"); err != nil || got != "a~20b" {
			t.Errorf("decode with %q: %q, %v", enc, got, err)
		}
	}
	if _, err := EncodeRecordKey("percent", "a"); err == nil {
		t.Error("unknown encoding encoded")
	}
	if _, err := DecodeRecordKey("percent", "a"); err == nil {
		t.Error("unknown encoding decoded")
	}
	rf := RecordFileDef{Name: "{key}.yaml", Format: RecordFormatYAML, RecordType: SingleRecord, KeyEncoding: "percent"}
	if err := rf.Validate(); err == nil || !strings.Contains(err.Error(), "unknown record_file.key_encoding") {
		t.Errorf("Validate: %v", err)
	}
	rf.KeyEncoding = KeyEncodingTildeV1
	if err := rf.Validate(); err != nil {
		t.Errorf("Validate tilde-v1: %v", err)
	}
	if key, err := rf.DecodeKey("a~2Fb"); err != nil || key != "a/b" {
		t.Errorf("DecodeKey: %q, %v", key, err)
	}
	if !rf.EncodesKeys() || (RecordFileDef{KeyEncoding: KeyEncodingNone}).EncodesKeys() || (RecordFileDef{}).EncodesKeys() {
		t.Error("EncodesKeys")
	}
}

// Verifies record-key-encoding#ac:over-long-key-fails.
func TestKeyEncoding_LengthLimit(t *testing.T) {
	t.Parallel()
	rf := RecordFileDef{Name: "{key}.yaml", Format: RecordFormatYAML, RecordType: SingleRecord, KeyEncoding: KeyEncodingTildeV1}
	fits := strings.Repeat("a", MaxFileNameBytes-len(".yaml"))
	if err := rf.CheckKeyFits(fits); err != nil {
		t.Errorf("a %d-byte name fits: %v", MaxFileNameBytes, err)
	}
	long := strings.Repeat("é", 43) // 86 bytes, 258 encoded
	err := rf.CheckKeyFits(long)
	if !errors.Is(err, ErrKeyNotStorable) || !strings.Contains(err.Error(), "263 bytes long, over the 255-byte limit") ||
		!strings.Contains(err.Error(), `record_file.type "map[$record_id]map[$field_name]any"`) {
		t.Errorf("over-long key: %v", err)
	}
	if _, err := rf.KeyDirName(long); !errors.Is(err, ErrKeyNotStorable) {
		t.Errorf("KeyDirName over-long: %v", err)
	}
	if name, err := rf.KeyDirName("a/b"); err != nil || name != "a~2Fb" {
		t.Errorf("KeyDirName: %q, %v", name, err)
	}
	if _, err := rf.KeyDirName(""); err == nil {
		t.Error("KeyDirName of an empty key")
	}
	if err := rf.CheckKeyFits(""); !errors.Is(err, ErrKeyNotStorable) {
		t.Errorf("empty key: %v", err)
	}
	// Without an encoding the key is measured as it is: the same rule, so a
	// per-file collection never meets a bare "file name too long".
	none := RecordFileDef{Name: "{key}.yaml"}
	if err := none.CheckKeyFits(long + long + long); !errors.Is(err, ErrKeyNotStorable) || strings.Contains(err.Error(), "key_encoding") {
		t.Errorf("no encoding, over-long: %v", err)
	}
	if err := none.CheckKeyFits(long); err != nil { // 86 bytes as it is
		t.Errorf("no encoding, fits as it is: %v", err)
	}
	if _, err := none.KeyDirName(long + long + long); !errors.Is(err, ErrKeyNotStorable) {
		t.Errorf("no encoding KeyDirName over-long: %v", err)
	}
	if name, err := none.KeyDirName("a b"); err != nil || name != "a b" {
		t.Errorf("no encoding KeyDirName: %q, %v", name, err)
	}
	// Not checked without one record per file.
	shared := RecordFileDef{Name: "all.yaml", KeyEncoding: KeyEncodingTildeV1}
	if err := shared.CheckKeyFits(long); err != nil {
		t.Errorf("one file: %v", err)
	}
	nested := RecordFileDef{Name: "{key}/data.yaml", KeyEncoding: KeyEncodingTildeV1}
	if err := nested.CheckKeyFits(long); !errors.Is(err, ErrKeyNotStorable) {
		t.Errorf("{key} directory: %v", err)
	}
}

func TestGetRecordFileName_KeyEncoding(t *testing.T) {
	t.Parallel()
	rf := RecordFileDef{Name: "{key}.yaml", KeyEncoding: KeyEncodingTildeV1}
	rec := dalrecord.NewRecordWithData(dalrecord.NewKeyWithID("c", "a/b"), map[string]any{})
	if name, err := rf.GetRecordFileName(rec); err != nil || name != "a~2Fb.yaml" {
		t.Errorf("tilde-v1: %q, %v", name, err)
	}
	long := dalrecord.NewRecordWithData(dalrecord.NewKeyWithID("c", strings.Repeat("~", 90)), map[string]any{})
	if _, err := rf.GetRecordFileName(long); !errors.Is(err, ErrKeyNotStorable) {
		t.Errorf("over-long: %v", err)
	}
	// Without an encoding a separator is still refused.
	if _, err := (RecordFileDef{Name: "{key}.yaml"}).GetRecordFileName(rec); err == nil {
		t.Error("separator accepted without an encoding")
	}
	// ...and a key whose file name is over 255 bytes fails with the remedy.
	plainLong := dalrecord.NewRecordWithData(dalrecord.NewKeyWithID("c", strings.Repeat("k", 251)), map[string]any{})
	if _, err := (RecordFileDef{Name: "{key}.yaml"}).GetRecordFileName(plainLong); !errors.Is(err, ErrKeyNotStorable) {
		t.Errorf("over-long without an encoding: %v", err)
	}
}

func TestNameDifferingInCase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Ab.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := NameDifferingInCase(dir, "ab.yaml"); !ok || got != "Ab.yaml" {
		t.Errorf("got %q, %v", got, ok)
	}
	if _, ok := NameDifferingInCase(dir, "Ab.yaml"); ok {
		t.Error("the same name is not a collision")
	}
	if _, ok := NameDifferingInCase(filepath.Join(dir, "missing"), "x"); ok {
		t.Error("missing directory")
	}
}

// Every {key} of the name is measured: drivers substitute them all.
func TestCheckKeyFits_EveryKeyPlaceholder(t *testing.T) {
	t.Parallel()
	rfd := RecordFileDef{Name: "{key}-{key}.yaml", KeyEncoding: KeyEncodingTildeV1}
	if err := rfd.CheckKeyFits(strings.Repeat("a", 130)); !errors.Is(err, ErrKeyNotStorable) {
		t.Errorf("two 130-byte keys make a 266-byte name: %v", err)
	}
}

func TestPathDifferingInCase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "s", "Ab", "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := PathDifferingInCase(root, filepath.Join(root, "s", "ab", "notes", "n.yaml")); !ok || got != filepath.Join(root, "s", "Ab") {
		t.Errorf("directory: %q, %v", got, ok)
	}
	if _, ok := PathDifferingInCase(root, filepath.Join(root, "s", "Ab", "notes", "n.yaml")); ok {
		t.Error("same names are not a collision")
	}
	for _, p := range []string{root, filepath.Dir(root), "relative"} {
		if _, ok := PathDifferingInCase(root, p); ok {
			t.Errorf("%s: outside or root itself", p)
		}
	}
}

func TestCaseOnlyDifferences(t *testing.T) {
	t.Parallel()
	got := CaseOnlyDifferences([]string{"a.yaml", "b.yaml", "A.yaml", "a.yaml", "B.yaml", "c.yaml"})
	if len(got) != 2 || strings.Join(got[0], " ") != "a.yaml A.yaml" || strings.Join(got[1], " ") != "b.yaml B.yaml" {
		t.Errorf("got %q", got)
	}
	if CaseOnlyDifferences([]string{"x", "y"}) != nil {
		t.Error("no groups expected")
	}
}

// Names that differ only in Unicode normalisation (NFC "é" against NFD
// "e" + U+0301) are one file on macOS, so they collide like a case-only
// pair; a group holds every name of one file, however many there are.
func TestCaseOnlyDifferences_NormalisationAndGroups(t *testing.T) {
	t.Parallel()
	nfc, nfd := "café.yaml", "café.yaml"
	got := CaseOnlyDifferences([]string{nfc, "x.yaml", nfd, "CAFÉ.yaml", "Bob", "bob", "BOB"})
	if len(got) != 2 || strings.Join(got[0], " ") != nfc+" "+nfd+" CAFÉ.yaml" || strings.Join(got[1], " ") != "Bob bob BOB" {
		t.Errorf("got %q", got)
	}
	// Names that differ otherwise, an accent included, and invalid UTF-8
	// (compared as it is) are not collisions.
	if got := CaseOnlyDifferences([]string{"bob", "bobby", "b0b", "cafe.yaml", nfc, "\xff", "\xfe", "a\xff", "A\xfe"}); got != nil {
		t.Errorf("false positives: %q", got)
	}
	if got := CaseOnlyDifferences([]string{"a\xff", "A\xff"}); len(got) != 1 {
		t.Errorf("invalid UTF-8 differing in ASCII case: %q", got)
	}
}

func TestNameDifferingInCase_Normalisation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "café.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := NameDifferingInCase(dir, "CAFÉ.yaml"); !ok || got != "café.yaml" {
		t.Errorf("got %q, %v", got, ok)
	}
	if _, ok := NameDifferingInCase(dir, "cafe.yaml"); ok {
		t.Error("an accent is a difference")
	}
}

// Canonically equivalent names whose combining marks are in another order
// collide even past 30 marks, where norm.NFD's stream-safe format inserts
// U+034F and so keeps them apart; an explicit U+034F in a name is kept.
func TestCaseOnlyDifferences_LongCombiningSequences(t *testing.T) {
	t.Parallel()
	marks := strings.Repeat("̅", 34)
	a, b := "a"+marks+"̖.yaml", "a̖"+marks+".yaml"
	if norm.NFD.String(a) == norm.NFD.String(b) {
		t.Fatal("norm.NFD no longer splits long sequences; this test no longer guards anything")
	}
	if got := CaseOnlyDifferences([]string{a, b}); len(got) != 1 {
		t.Errorf("reordered marks: %q", got)
	}
	if got := CaseOnlyDifferences([]string{"a̖̅.yaml", "a͏̖̅.yaml"}); got != nil {
		t.Errorf("explicit U+034F: %q", got)
	}
	if lc, nf := NameCollisionKind([]string{a, b}); lc || !nf {
		t.Errorf("reordered marks differ in normalisation only: %v %v", lc, nf)
	}
}

// Full case folding merges ß and ss; dotless ı is mapped to i as the NTFS
// upcase table does (ı, i and I upcase to I); İ folds to i + U+0307 and stays
// apart from i.
func TestCaseOnlyDifferences_FullFoldingAndDotlessI(t *testing.T) {
	t.Parallel()
	got := CaseOnlyDifferences([]string{"straße", "STRASSE", "ı.yaml", "i.yaml", "I.yaml", "İ.yaml"})
	if len(got) != 2 || strings.Join(got[0], " ") != "straße STRASSE" || strings.Join(got[1], " ") != "ı.yaml i.yaml I.yaml" {
		t.Errorf("got %q", got)
	}
	if lc, nf := NameCollisionKind([]string{"ı.yaml", "I.yaml"}); !lc || nf {
		t.Errorf("dotless i differs in case only: %v %v", lc, nf)
	}
}

// The key is stable: the key of a key is the key (Unicode D145 needs the
// inner decomposition for that; the outer one decomposes what folding made).
func TestSameFileKey_Stable(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Ab", "café", "café", "ẞ", "ͅ", "ǰ̣", "가", "ı", "İ", "a\xff", "ﬃ"} {
		k := sameFileKey(name)
		if sameFileKey(k) != k {
			t.Errorf("%q: key %q is not stable", name, k)
		}
	}
	if sameFileKey("ǰ̣") != sameFileKey("J̣̌") {
		t.Error("folding then decomposing")
	}
}

func TestNameCollisionKind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		names  []string
		lc, nf bool
	}{
		{[]string{"Ab", "ab"}, true, false},
		{[]string{"café", "café"}, false, true},
		{[]string{"Café", "café"}, true, true},
		{[]string{"a\xff", "A\xff"}, true, false},
		// Singleton decompositions: the Kelvin, Ohm and Angstrom signs are
		// canonically equivalent to letters (case folding merges them too).
		{[]string{"K.yaml", "\u212a.yaml"}, false, true},
		{[]string{"\u03a9", "\u2126"}, false, true},
		{[]string{"\u00c5", "\u212b"}, false, true},
		{[]string{"k", "\u212a"}, true, true},
		{[]string{"\u00c9", "\u00e9"}, true, false},
		{[]string{"E\u0301", "e\u0301"}, true, false},
	} {
		if lc, nf := NameCollisionKind(tc.names); lc != tc.lc || nf != tc.nf {
			t.Errorf("%q: %v %v", tc.names, lc, nf)
		}
	}
}

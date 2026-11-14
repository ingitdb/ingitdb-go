package ingitdb

// specscore: feature/record-key-encoding

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Record key encodings a collection can declare in record_file.key_encoding.
// A definition that declares none keeps the historical behaviour: the key is
// the file name part as it is.
const (
	KeyEncodingNone    = "none"
	KeyEncodingTildeV1 = "tilde-v1"
)

// MaxFileNameBytes is the longest file or directory name a record key may
// produce in a collection that declares a key encoding (see CheckKeyFits).
const MaxFileNameBytes = 255

var (
	// ErrInvalidKeyName reports a file or directory name that the
	// collection's key_encoding never writes, so it names no record key.
	ErrInvalidKeyName = errors.New("not a record key name of this key_encoding")
	// ErrKeyNotStorable reports a record key that cannot become a file name.
	ErrKeyNotStorable = errors.New("record key cannot be stored as a file name")
)

// EncodeRecordKey returns the file or directory name of a record key under
// the given key encoding ("" and "none" return the key as it is).
//
// tilde-v1 keeps the bytes A-Z a-z 0-9 - _ . @ + = , and writes every other
// byte as "~" and two upper-case hexadecimal digits, "~" itself included. It
// also writes a "." that is the first or the last byte that way, and the
// first byte of a key that, up to its first "." and in any letter case, is a
// Windows device name (CON, PRN, AUX, NUL, COM0-COM9, LPT0-LPT9). The result
// is pure ASCII, is never hidden, ".", "..", ends in neither "." nor space and
// holds no character any common file system reserves. An empty key is
// refused. The length of the name is not checked here (see
// RecordFileDef.CheckKeyFits).
func EncodeRecordKey(encoding, key string) (string, error) {
	switch encoding {
	case "", KeyEncodingNone:
		return key, nil
	case KeyEncodingTildeV1:
		if key == "" {
			return "", fmt.Errorf("%w: the key is empty", ErrKeyNotStorable)
		}
		return tildeV1Encode(key), nil
	default:
		return "", fmt.Errorf("unknown key_encoding %q", encoding)
	}
}

// DecodeRecordKey returns the record key of a file or directory name under
// the given key encoding ("" and "none" return the name as it is). For
// tilde-v1 only the names EncodeRecordKey writes decode; any other name
// (a lower-case or incomplete escape, a byte that should have been escaped)
// fails with ErrInvalidKeyName, so every key has exactly one name.
func DecodeRecordKey(encoding, name string) (string, error) {
	switch encoding {
	case "", KeyEncodingNone:
		return name, nil
	case KeyEncodingTildeV1:
		key, ok := tildeV1Decode(name)
		if !ok {
			return "", fmt.Errorf("%w: %q is not a %s name; the name of key %q is %q", ErrInvalidKeyName, name, KeyEncodingTildeV1, key, tildeV1Encode(key))
		}
		return key, nil
	default:
		return "", fmt.Errorf("unknown key_encoding %q", encoding)
	}
}

func tildeV1Keeps(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	return strings.IndexByte("-_.@+=,", c) >= 0
}

// windowsDeviceName reports whether key, up to its first ".", is a reserved
// device name on Windows, in any letter case.
func windowsDeviceName(key string) bool {
	base, _, _ := strings.Cut(strings.ToUpper(key), ".")
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	return len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && '0' <= base[3] && base[3] <= '9'
}

func tildeV1Encode(key string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	device := windowsDeviceName(key)
	for i := 0; i < len(key); i++ {
		c := key[i]
		edgeDot := c == '.' && (i == 0 || i == len(key)-1)
		if !tildeV1Keeps(c) || edgeDot || (i == 0 && device) {
			b.WriteByte('~')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0F])
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// tildeV1Decode returns the key of name and whether name is the one name
// tildeV1Encode writes for it. On failure key is the best-effort reading of
// name (escapes that are well formed decoded), for the error message.
func tildeV1Decode(name string) (key string, ok bool) {
	var b strings.Builder
	ok = true
	for i := 0; i < len(name); i++ {
		if name[i] != '~' {
			b.WriteByte(name[i])
			continue
		}
		hi, lo := -1, -1
		if i+2 < len(name) {
			hi, lo = hexValue(name[i+1]), hexValue(name[i+2])
		}
		if hi < 0 || lo < 0 {
			b.WriteByte('~')
			ok = false
			continue
		}
		// Lower-case digits are read for the error message only: the name
		// is not canonical (the check below fails).
		b.WriteByte(byte(hi<<4 | lo))
		i += 2
	}
	key = b.String()
	return key, ok && key != "" && tildeV1Encode(key) == name
}

func hexValue(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'A' <= c && c <= 'F':
		return int(c-'A') + 10
	case 'a' <= c && c <= 'f':
		return int(c-'a') + 10
	}
	return -1
}

// EncodesKeys reports whether the record file declares a key encoding other
// than the historical as-is one.
func (rfd RecordFileDef) EncodesKeys() bool {
	return rfd.KeyEncoding != "" && rfd.KeyEncoding != KeyEncodingNone
}

// EncodeKey is EncodeRecordKey with the record file's key_encoding.
func (rfd RecordFileDef) EncodeKey(key string) (string, error) {
	return EncodeRecordKey(rfd.KeyEncoding, key)
}

// DecodeKey is DecodeRecordKey with the record file's key_encoding.
func (rfd RecordFileDef) DecodeKey(name string) (string, error) {
	return DecodeRecordKey(rfd.KeyEncoding, name)
}

// CheckKeyFits fails with ErrKeyNotStorable, and the remedy, when the record
// file keeps one record per file (a {key} name) and a path segment
// record_file.name makes from the key (encoded when the record file declares
// a key encoding, as it is otherwise) is longer than MaxFileNameBytes. Keys
// are never hashed or truncated: such a key needs a collection that keeps its
// records in one file. Without an encoding this only names the cause and the
// remedy where the file system would refuse the name anyway.
func (rfd RecordFileDef) CheckKeyFits(key string) error {
	if !strings.Contains(rfd.Name, "{key}") {
		return nil
	}
	name, err := rfd.EncodeKey(key)
	if err != nil {
		return err
	}
	// Every {key} is substituted, as the drivers do (GetRecordFileName
	// substitutes only the first, so this bound holds for it too). Other
	// placeholders ({field}) are not part of the key and are not measured.
	for _, segment := range strings.Split(strings.ReplaceAll(rfd.Name, "{key}", name), "/") {
		if len(segment) > MaxFileNameBytes {
			return fmt.Errorf("%w: %sthe key %.40q... (%d bytes) makes the file name %.40q... %d bytes long, over the %d-byte limit of a file name; keys are never shortened: keep this collection's records in one file (record_file.type %q) instead of one file per record",
				ErrKeyNotStorable, rfd.encodingClause(), key, len(key), segment, len(segment), MaxFileNameBytes, MapOfRecords)
		}
	}
	return nil
}

// KeyDirName returns the name of the directory that holds the subcollection
// instances of the record of key: the encoded key, or the key as it is
// without an encoding. A name longer than MaxFileNameBytes fails with
// ErrKeyNotStorable.
func (rfd RecordFileDef) KeyDirName(key string) (string, error) {
	name, err := rfd.EncodeKey(key)
	if err != nil {
		return "", err
	}
	if len(name) > MaxFileNameBytes {
		return "", fmt.Errorf("%w: %sthe key %.40q... (%d bytes) makes a directory name %d bytes long, over the %d-byte limit, so its subcollections cannot be stored; keys are never shortened",
			ErrKeyNotStorable, rfd.encodingClause(), key, len(key), len(name), MaxFileNameBytes)
	}
	return name, nil
}

// encodingClause starts a length error with the key encoding, if any.
func (rfd RecordFileDef) encodingClause() string {
	if rfd.EncodesKeys() {
		return "with key_encoding " + rfd.KeyEncoding + " "
	}
	return ""
}

func (rfd RecordFileDef) validateKeyEncoding() error {
	switch rfd.KeyEncoding {
	case "", KeyEncodingNone, KeyEncodingTildeV1:
		return nil
	default:
		return fmt.Errorf("unknown record_file.key_encoding %q (supported: %s, %s)", rfd.KeyEncoding, KeyEncodingNone, KeyEncodingTildeV1)
	}
}

// CaseOnlyDifferences returns the groups of names that are equal when letter
// case is ignored and differ otherwise. On a case-insensitive file system
// (Windows, macOS by default) each group is one file. Groups and names are in
// the order of names.
func CaseOnlyDifferences(names []string) [][]string {
	index := map[string]int{}
	var groups [][]string
	for _, name := range names {
		folded := strings.ToLower(name)
		i, seen := index[folded]
		if !seen {
			index[folded] = len(groups)
			groups = append(groups, []string{name})
			continue
		}
		if !containsString(groups[i], name) {
			groups[i] = append(groups[i], name)
		}
	}
	var out [][]string
	for _, g := range groups {
		if len(g) > 1 {
			out = append(out, g)
		}
	}
	return out
}

// NameDifferingInCase returns the entry of dir whose name equals name when
// letter case is ignored but differs from it, if there is one. A writer of a
// collection that declares a key encoding calls it before creating a record
// file or directory, and refuses to write when it finds one
// (record-key-encoding#req:letter-case-kept-collisions-reported). A missing
// directory has no entries.
func NameDifferingInCase(dir, name string) (string, bool) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != name && strings.EqualFold(e.Name(), name) {
			return e.Name(), true
		}
	}
	return "", false
}

// PathDifferingInCase checks every segment of path below root, the record
// file and the directories a write would create (parent record directories
// included), with NameDifferingInCase. It returns the first existing entry
// whose name differs from the segment only in letter case. A path outside
// root is not checked.
func PathDifferingInCase(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	dir := root
	for _, segment := range strings.Split(rel, string(filepath.Separator)) {
		if existing, found := NameDifferingInCase(dir, segment); found {
			return filepath.Join(dir, existing), true
		}
		dir = filepath.Join(dir, segment)
	}
	return "", false
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

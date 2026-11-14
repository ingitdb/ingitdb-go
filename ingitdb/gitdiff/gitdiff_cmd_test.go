package gitdiff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestParseNameStatus(t *testing.T) {
	t.Parallel()

	// -z output: NUL after the status and after every path; paths are never
	// quoted or escaped, whatever they hold.
	out := "M\x00countries/ie.yaml\x00" +
		"A\x00countries/caf\u00e9\t\"x\".yaml\x00" +
		"D\x00countries/fr.yaml\x00" +
		"R100\x00countries/de.yaml\x00countries/germany.yaml\x00" +
		"C075\x00countries/a.yaml\x00countries/b\xff.yaml\x00"
	got := parseNameStatus(out)

	want := []ingitdb.ChangedFile{
		{Kind: ingitdb.ChangeKindModified, Path: "countries/ie.yaml"},
		{Kind: ingitdb.ChangeKindAdded, Path: "countries/caf\u00e9\t\"x\".yaml"},
		{Kind: ingitdb.ChangeKindDeleted, Path: "countries/fr.yaml"},
		{Kind: ingitdb.ChangeKindRenamed, OldPath: "countries/de.yaml", Path: "countries/germany.yaml"},
		{Kind: ingitdb.ChangeKindRenamed, OldPath: "countries/a.yaml", Path: "countries/b\xff.yaml"},
	}
	if len(got) != len(want) {
		t.Fatalf("parseNameStatus returned %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Paths git would quote and escape without -z (non-ASCII under the default
// core.quotePath, a tab, NFC and NFD forms of one name) arrive byte for byte.
func TestCmdGitDiffer_DiffFiles_UnquotedPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	git := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "T")
	git("config", "core.quotePath", "true")
	if err := os.WriteFile(filepath.Join(dir, "base.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "base")
	base := git("rev-parse", "HEAD")
	names := []string{"caf\u00e9.yaml", "cafe\u0301.yaml", "tab\there.yaml"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-m", "names")
	got, err := NewGitDiffer().DiffFiles(context.Background(), dir, base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, c := range got {
		paths[c.Path] = c.Kind == ingitdb.ChangeKindAdded
	}
	for _, name := range names {
		if !paths[name] {
			t.Errorf("%q not reported as added: %+v", name, got)
		}
	}
}

func TestCmdGitDiffer_RequiresFromRef(t *testing.T) {
	t.Parallel()
	if _, err := NewGitDiffer().DiffFiles(context.Background(), t.TempDir(), "", "HEAD"); err == nil {
		t.Fatal("expected error when from ref is empty")
	}
}

func TestCmdGitDiffer_DiffFiles_RealRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	git("init")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "T")
	write("a.yaml", "v: 1\n")
	git("add", ".")
	git("commit", "-m", "base")
	revParse := exec.Command("git", "rev-parse", "HEAD")
	revParse.Dir = dir
	baseOut, err := revParse.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse: %v\n%s", err, baseOut)
	}
	base := strings.TrimSpace(string(baseOut))

	write("a.yaml", "v: 2\n") // modified
	write("b.yaml", "v: 3\n") // added
	git("add", ".")
	git("commit", "-m", "second")

	got, err := NewGitDiffer().DiffFiles(context.Background(), dir, base, "HEAD")
	if err != nil {
		t.Fatalf("DiffFiles: %v", err)
	}
	kinds := map[string]ingitdb.ChangeKind{}
	for _, c := range got {
		kinds[c.Path] = c.Kind
	}
	if kinds["a.yaml"] != ingitdb.ChangeKindModified {
		t.Errorf("a.yaml kind = %q, want modified", kinds["a.yaml"])
	}
	if kinds["b.yaml"] != ingitdb.ChangeKindAdded {
		t.Errorf("b.yaml kind = %q, want added", kinds["b.yaml"])
	}
}

func TestParseNameStatus_EdgeCases(t *testing.T) {
	t.Parallel()

	for _, out := range []string{
		"",
		"\x00",                 // an empty status
		"M\x00",                // a status without its path
		"R100\x00only-one\x00", // a rename without its new path
		"C100\x00only-one",     // a copy cut short
	} {
		if got := parseNameStatus(out); len(got) != 0 {
			t.Errorf("%q: want no entries, got %+v", out, got)
		}
	}
}

func TestCmdGitDiffer_DiffFiles_Error(t *testing.T) {
	t.Parallel()

	// An invalid repo path or non-existent commit ref causes git diff to fail
	_, err := NewGitDiffer().DiffFiles(context.Background(), t.TempDir(), "nonexistent-from-ref", "HEAD")
	if err == nil {
		t.Fatal("expected error from git diff with non-existent ref")
	}
}

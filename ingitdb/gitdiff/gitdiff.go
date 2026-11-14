package gitdiff

// specscore: feature/cli/validate

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// GitDiffer lists files changed between two git refs.
// The default implementation shells out to `git diff --name-status fromRef toRef`.
type GitDiffer interface {
	DiffFiles(ctx context.Context, repoPath, fromRef, toRef string) ([]ingitdb.ChangedFile, error)
}

// NewGitDiffer returns the default GitDiffer, which shells out to git.
func NewGitDiffer() GitDiffer {
	return cmdGitDiffer{}
}

type cmdGitDiffer struct{}

// DiffFiles runs `git diff --name-status -z <fromRef> [<toRef>]` in repoPath
// and parses the result. An empty toRef diffs fromRef against the working
// tree. -z makes git print every path byte for byte: without it a path with
// non-ASCII bytes (under the default core.quotePath), a tab, a quote or a
// control character comes quoted and octal-escaped, matches no collection,
// and is silently not validated.
func (cmdGitDiffer) DiffFiles(ctx context.Context, repoPath, fromRef, toRef string) ([]ingitdb.ChangedFile, error) {
	if fromRef == "" {
		return nil, fmt.Errorf("from ref is required")
	}
	args := []string{"diff", "--name-status", "-z", fromRef}
	if toRef != "" {
		args = append(args, toRef)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	return parseNameStatus(string(out)), nil
}

// parseNameStatus parses `git diff --name-status -z` output into ChangedFile
// entries. Every field ends with NUL: a status, then its path, or for a
// rename or copy (a score-suffixed status) the old path and the new path.
// Output cut short yields the entries before the cut.
func parseNameStatus(out string) []ingitdb.ChangedFile {
	var changed []ingitdb.ChangedFile
	// Only fields followed by a NUL are complete.
	fields := strings.Split(out, "\x00")
	fields = fields[:len(fields)-1]
	for i := 0; i+1 < len(fields) && fields[i] != ""; {
		status := fields[i]
		switch status[0] {
		case 'R', 'C':
			if i+2 >= len(fields) {
				return changed
			}
			changed = append(changed, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindRenamed, OldPath: fields[i+1], Path: fields[i+2]})
			i += 3
			continue
		case 'A':
			changed = append(changed, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindAdded, Path: fields[i+1]})
		case 'D':
			changed = append(changed, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindDeleted, Path: fields[i+1]})
		default: // M, T (type change) and anything else → treat as modified
			changed = append(changed, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindModified, Path: fields[i+1]})
		}
		i += 2
	}
	return changed
}

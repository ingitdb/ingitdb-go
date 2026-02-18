package gitdiff

import (
	"context"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// GitDiffer lists files changed between two git refs.
// The default implementation shells out to `git diff --name-status fromRef toRef`.
type GitDiffer interface {
	DiffFiles(ctx context.Context, repoPath, fromRef, toRef string) ([]ingitdb.ChangedFile, error)
}

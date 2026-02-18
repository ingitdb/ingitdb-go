package ingitdb

// ChangeKind describes how a file changed between two git refs.
type ChangeKind string

const (
	ChangeKindAdded    ChangeKind = "added"
	ChangeKindModified ChangeKind = "modified"
	ChangeKindDeleted  ChangeKind = "deleted"
	ChangeKindRenamed  ChangeKind = "renamed"
)

// ChangedFile is one file that differed between two git refs.
type ChangedFile struct {
	Kind    ChangeKind
	Path    string // repo-relative path
	OldPath string // set only for ChangeKindRenamed
}

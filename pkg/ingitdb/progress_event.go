package ingitdb

// ProgressKind describes the type of progress update.
type ProgressKind string

const (
	ProgressKindStarted   ProgressKind = "started"
	ProgressKindItemDone  ProgressKind = "item_done"
	ProgressKindSkipped   ProgressKind = "skipped"
	ProgressKindCompleted ProgressKind = "completed"
	ProgressKindAborted   ProgressKind = "aborted"
	ProgressKindError     ProgressKind = "error"
)

// ProgressEvent carries one progress update from a running task.
type ProgressEvent struct {
	Kind     ProgressKind
	TaskName string            // "validate" or "materialize"
	Scope    string            // collection or view ID
	ItemKey  string            // record key or output file name
	Done     int               // items completed so far
	Total    int               // total items; 0 = unknown
	Err      *ValidationError  // non-nil only for ProgressKindError
}

package watcher

import "context"

// Trigger is a named handler that reacts to a RecordEvent.
// Built-in trigger types include webhook and shell exec.
// The watcher calls each registered Trigger.Fire() on change.
type Trigger interface {
	Name() string
	Fire(ctx context.Context, event RecordEvent) error
}

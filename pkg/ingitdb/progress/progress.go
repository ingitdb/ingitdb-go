package progress

import (
	"context"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ProgressReporter receives events from the dispatcher. Goroutine-safe; must not block.
type ProgressReporter interface {
	Report(event ingitdb.ProgressEvent)
}

// Signal is a user-initiated control action.
type Signal int

const (
	SignalNone      Signal = iota
	SignalSkipItem         // skip current record/view, continue
	SignalAbort            // cancel entire operation
	SignalDrillDown        // request detail display (TUI only)
)

// Steerer emits control signals from the TUI into the dispatcher.
// Goroutine-safe; Steer() resets the signal to SignalNone on read.
type Steerer interface {
	Steer() Signal
}

// Task is a unit of work the dispatcher schedules.
type Task interface {
	Name() string
	Run(ctx context.Context, reporter ProgressReporter, steerer Steerer) error
}

// Dispatcher schedules and coordinates Tasks.
type Dispatcher interface {
	RunSequential(ctx context.Context, tasks []Task) error
	// RunParallel runs tasks concurrently; concurrency 0 uses runtime.NumCPU().
	RunParallel(ctx context.Context, tasks []Task, concurrency int) error
}

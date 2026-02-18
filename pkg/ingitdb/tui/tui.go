package tui

import (
	"fmt"
	"io"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/progress"
)

// Screen is the minimal display interface. Goroutine-safe.
type Screen interface {
	Update(event ingitdb.ProgressEvent)
	ShowDetail(item ingitdb.ProgressEvent, errs []ingitdb.ValidationError)
	Close() error
}

// TUI combines Screen and Steerer. The command layer passes one TUI to dispatcher setup.
type TUI interface {
	Screen
	progress.Steerer
}

type noOpTUI struct {
	w io.Writer
}

// NewNoOpTUI returns a TUI that prints to w and never emits control signals.
// Use for non-interactive (CI, piped) invocations.
func NewNoOpTUI(w io.Writer) TUI {
	return &noOpTUI{w: w}
}

func (n *noOpTUI) Update(event ingitdb.ProgressEvent) {
	_, _ = fmt.Fprintf(n.w, "%s %s %s\n", event.Kind, event.TaskName, event.Scope)
}

func (n *noOpTUI) ShowDetail(item ingitdb.ProgressEvent, errs []ingitdb.ValidationError) {
	_, _ = item, errs
}

func (n *noOpTUI) Close() error { return nil }

func (n *noOpTUI) Steer() progress.Signal { return progress.SignalNone }

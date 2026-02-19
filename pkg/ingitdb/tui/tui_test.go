package tui

import (
	"bytes"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/progress"
)

func TestNewNoOpTUI(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tui := NewNoOpTUI(&buf)

	if tui == nil {
		t.Fatalf("NewNoOpTUI returned nil")
	}

	noOp, ok := tui.(*noOpTUI)
	if !ok {
		t.Fatalf("expected *noOpTUI, got %T", tui)
	}

	if noOp.w != &buf {
		t.Errorf("expected writer to be set to provided io.Writer")
	}
}

func TestNoOpTUI_Update(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tui := NewNoOpTUI(&buf)

	event := ingitdb.ProgressEvent{
		Kind:     ingitdb.ProgressKindStarted,
		TaskName: "validate",
		Scope:    "test-collection",
	}

	tui.Update(event)

	output := buf.String()
	expected := "started validate test-collection\n"
	if output != expected {
		t.Errorf("Update output = %q, want %q", output, expected)
	}
}

func TestNoOpTUI_ShowDetail(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tui := NewNoOpTUI(&buf)

	event := ingitdb.ProgressEvent{
		Kind:     ingitdb.ProgressKindError,
		TaskName: "validate",
		Scope:    "test-collection",
	}

	errs := []ingitdb.ValidationError{
		{
			Severity:     ingitdb.SeverityError,
			CollectionID: "test-collection",
			FilePath:     "/path/to/file.yaml",
		},
	}

	// ShowDetail should do nothing (no-op)
	tui.ShowDetail(event, errs)

	// Verify nothing was written
	output := buf.String()
	if output != "" {
		t.Errorf("ShowDetail wrote %q, expected nothing", output)
	}
}

func TestNoOpTUI_Close(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tui := NewNoOpTUI(&buf)

	err := tui.Close()
	if err != nil {
		t.Errorf("Close returned error: %v, want nil", err)
	}
}

func TestNoOpTUI_Steer(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tui := NewNoOpTUI(&buf)

	signal := tui.Steer()
	if signal != progress.SignalNone {
		t.Errorf("Steer returned %v, want %v", signal, progress.SignalNone)
	}
}

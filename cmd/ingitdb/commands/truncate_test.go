package commands

import (
	"testing"
)

func TestTruncate_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Truncate()
	if cmd == nil {
		t.Fatal("Truncate() returned nil")
	}
	if cmd.Name != "truncate" {
		t.Errorf("expected name 'truncate', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestTruncate_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := Truncate()
	err := runCLICommand(cmd, "--collection=test.items")
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

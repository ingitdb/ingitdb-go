package commands

import (
	"testing"
)

func TestSetup_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Setup()
	if cmd == nil {
		t.Fatal("Setup() returned nil")
	}
	if cmd.Name != "setup" {
		t.Errorf("expected name 'setup', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestSetup_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := Setup()
	err := runCLICommand(cmd)
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

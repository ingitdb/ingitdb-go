package commands

import (
	"errors"
	"testing"
)

func TestExpandHome_NoTilde(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }

	got, err := expandHome("/tmp/db", homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/db" {
		t.Fatalf("expected /tmp/db, got %s", got)
	}
}

func TestExpandHome_Tilde(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }

	got, err := expandHome("~", homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/home" {
		t.Fatalf("expected /tmp/home, got %s", got)
	}
}

func TestExpandHome_Error(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "", errors.New("no home") }

	got, err := expandHome("~", homeDir)
	if err == nil {
		t.Fatal("expected error")
	}
	if got != "" {
		t.Fatalf("expected empty result, got %s", got)
	}
}

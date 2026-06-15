// Package testutil provides shared test helpers for ingitdb-go tests.
package testutil

import (
	"strings"
	"testing"
)

// MustErrContain asserts that err is non-nil and its message contains every
// substring in substrs. It reports failure via t.Fatalf.
func MustErrContain(t testing.TB, err error, substrs ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	for _, s := range substrs {
		if !strings.Contains(msg, s) {
			t.Fatalf("error %q does not contain %q", msg, s)
		}
	}
}

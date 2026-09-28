package testutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type mockTB struct {
	testing.TB
	failed   bool
	fatalMsg string
}

func (m *mockTB) Helper() {}

func (m *mockTB) Fatal(args ...any) {
	m.failed = true
	m.fatalMsg = fmt.Sprint(args...)
	panic("fatal")
}

func (m *mockTB) Fatalf(format string, args ...any) {
	m.failed = true
	m.fatalMsg = fmt.Sprintf(format, args...)
	panic("fatal")
}

func callMustErrContain(tb *mockTB, err error, substrs ...string) {
	defer func() {
		_ = recover()
	}()
	MustErrContain(tb, err, substrs...)
}

func TestMustErrContain(t *testing.T) {
	// Success case
	tb := &mockTB{}
	callMustErrContain(tb, errors.New("hello world foo bar"), "hello", "world")
	if tb.failed {
		t.Errorf("expected no failure, got fatalMsg=%s", tb.fatalMsg)
	}

	// Error is nil
	tbNil := &mockTB{}
	callMustErrContain(tbNil, nil, "hello")
	if !tbNil.failed || !strings.Contains(tbNil.fatalMsg, "expected error, got nil") {
		t.Errorf("expected failure on nil error, got fatalMsg=%s", tbNil.fatalMsg)
	}

	// Error doesn't contain substr
	tbMissing := &mockTB{}
	callMustErrContain(tbMissing, errors.New("something else"), "expected_substring")
	if !tbMissing.failed || !strings.Contains(tbMissing.fatalMsg, "does not contain") {
		t.Errorf("expected failure on missing substring, got fatalMsg=%s", tbMissing.fatalMsg)
	}
}

package ingitdb

import "testing"

func TestNewReadOptions_Defaults(t *testing.T) {
	t.Parallel()

	opts := NewReadOptions()
	if opts.IsValidationRequired() {
		t.Fatal("expected validation to be disabled by default")
	}
}

func TestNewReadOptions_WithValidate(t *testing.T) {
	t.Parallel()

	opts := NewReadOptions(Validate())
	if !opts.IsValidationRequired() {
		t.Fatal("expected validation to be enabled")
	}
}

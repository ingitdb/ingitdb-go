package ingitdb

import "testing"

func TestValidate(t *testing.T) {
	err := Validate(".")
	if err == nil {
		t.Fatal("expected error, got none")
	}
}

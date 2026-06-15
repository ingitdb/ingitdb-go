package ingitdb

import "testing"

func TestColumnDefValidate_EmptyType(t *testing.T) {
	t.Parallel()

	def := ColumnDef{}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestColumnDefValidate_Success(t *testing.T) {
	t.Parallel()

	def := ColumnDef{Type: "string"}
	err := def.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
}

func TestColumnDefValidate_UnsupportedType(t *testing.T) {
	t.Parallel()

	def := ColumnDef{Type: "map[uuid]string"}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

package ingitdb

import (
	"strings"
	"testing"
)

func TestCollectionDefValidate_Errors(t *testing.T) {
	t.Parallel()

	columns := map[string]*ColumnDef{
		"name": {Type: "string"},
	}
	recordFile := &RecordFileDef{
		Format: "JSON",
		Name:   "{key}.json",
	}

	tests := []struct {
		name string
		def  *CollectionDef
		err  string
	}{
		{
			name: "missing_columns",
			def: &CollectionDef{
				Columns:    map[string]*ColumnDef{},
				RecordFile: recordFile,
			},
			err: "missing 'columns' in collection definition",
		},
		{
			name: "missing_column_type",
			def: &CollectionDef{
				Columns: map[string]*ColumnDef{
					"name": {},
				},
				RecordFile: recordFile,
			},
			err: "invalid column 'name': missing 'type' in column definition",
		},
		{
			name: "columns_order_unknown_column",
			def: &CollectionDef{
				Columns:      columns,
				ColumnsOrder: []string{"age"},
				RecordFile:   recordFile,
			},
			err: "columns_order[0] references unspecified column: age",
		},
		{
			name: "columns_order_duplicate",
			def: &CollectionDef{
				Columns: map[string]*ColumnDef{
					"name": {Type: "string"},
					"age":  {Type: "int"},
				},
				ColumnsOrder: []string{"name", "age", "name"},
				RecordFile:   recordFile,
			},
			err: "duplicate value in columns_order at indexes 0 and 2: name",
		},
		{
			name: "missing_record_file",
			def: &CollectionDef{
				Columns: columns,
			},
			err: "missing 'record_file' in collection definition",
		},
		{
			name: "invalid_record_file",
			def: &CollectionDef{
				Columns:    columns,
				RecordFile: &RecordFileDef{},
			},
			err: "invalid record_file definition",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.def.Validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			errMsg := err.Error()
			if !strings.Contains(errMsg, tt.err) {
				t.Fatalf("expected error to contain %q, got %q", tt.err, errMsg)
			}
		})
	}
}

func TestCollectionDefValidate_Success(t *testing.T) {
	t.Parallel()

	def := &CollectionDef{
		Columns: map[string]*ColumnDef{
			"name": {Type: "string"},
		},
		ColumnsOrder: []string{"name"},
		RecordFile: &RecordFileDef{
			Format: "JSON",
			Name:   "{key}.json",
		},
	}

	err := def.Validate()
	if err != nil {
		errMsg := err.Error()
		t.Fatalf("expected no error, got %s", errMsg)
	}
}

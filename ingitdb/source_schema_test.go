package ingitdb

import (
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestSourceSchemaValidation(t *testing.T) {
	cols := map[string]*ColumnDef{"id": {Type: ColumnTypeString}, "value": {Type: ColumnTypeString}}
	cases := []struct {
		name   string
		schema *SourceSchemaDef
		want   string
	}{
		{"nil", nil, ""},
		{"valid", &SourceSchemaDef{KeyMode: "source-primary-key", RelationshipComparison: "raw", StorageClassFiles: []string{"source-storage-0001.jsonl"}, Fields: []SourceFieldDef{{Name: "id", Type: "string"}}, Indexes: []SourceIndexDef{{Name: "ix", Fields: []string{"id"}}}, ForeignKeys: []SourceForeignKeyDef{{Fields: []string{"id"}, ReferencedCollection: "parent"}}}, ""},
		{"key mode", &SourceSchemaDef{KeyMode: "guess"}, "key_mode"},
		{"comparison", &SourceSchemaDef{RelationshipComparison: "guess"}, "relationship_comparison"},
		{"empty file", &SourceSchemaDef{StorageClassFiles: []string{""}}, "storage class file"},
		{"dot file", &SourceSchemaDef{StorageClassFiles: []string{".hidden.jsonl"}}, "storage class file"},
		{"parent path", &SourceSchemaDef{StorageClassFiles: []string{"../x.jsonl"}}, "storage class file"},
		{"extension", &SourceSchemaDef{StorageClassFiles: []string{"data.json"}}, "storage class file"},
		{"duplicate file", &SourceSchemaDef{StorageClassFiles: []string{"data.jsonl", "data.jsonl"}}, "storage class file"},
		{"empty field", &SourceSchemaDef{Fields: []SourceFieldDef{{Name: "", Type: "string"}}}, "field"},
		{"unknown field", &SourceSchemaDef{Fields: []SourceFieldDef{{Name: "other", Type: "string"}}}, "field"},
		{"duplicate field", &SourceSchemaDef{Fields: []SourceFieldDef{{Name: "id", Type: "string"}, {Name: "id", Type: "string"}}}, "field"},
		{"field type", &SourceSchemaDef{Fields: []SourceFieldDef{{Name: "id"}}}, "no type"},
		{"index name", &SourceSchemaDef{Indexes: []SourceIndexDef{{Fields: []string{"id"}}}}, "index"},
		{"index field", &SourceSchemaDef{Indexes: []SourceIndexDef{{Name: "ix", Fields: []string{"other"}}}}, "missing field"},
		{"fk target", &SourceSchemaDef{ForeignKeys: []SourceForeignKeyDef{{Fields: []string{"id"}}}}, "foreign key"},
		{"fk fields", &SourceSchemaDef{ForeignKeys: []SourceForeignKeyDef{{ReferencedCollection: "parent"}}}, "foreign key"},
		{"fk arity", &SourceSchemaDef{ForeignKeys: []SourceForeignKeyDef{{Fields: []string{"id", "value"}, ReferencedCollection: "parent", ReferencedFields: []string{"id"}}}}, "foreign key"},
		{"fk field", &SourceSchemaDef{ForeignKeys: []SourceForeignKeyDef{{Fields: []string{"other"}, ReferencedCollection: "parent"}}}, "missing field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.schema.Validate(cols)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestCollectionUsesSourceSchemaValidationAndMarshal(t *testing.T) {
	col := &CollectionDef{ID: "x", Columns: map[string]*ColumnDef{"id": {Type: ColumnTypeString}}, SourceSchema: &SourceSchemaDef{KeyMode: "invalid"}}
	if err := col.Validate(); err == nil || !strings.Contains(err.Error(), "key_mode") {
		t.Fatalf("invalid source schema accepted: %v", err)
	}
	col.SourceSchema.KeyMode = "source-primary-key"
	data, err := yaml.Marshal(col)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "source_schema:") {
		t.Fatalf("source schema absent: %s", data)
	}
}

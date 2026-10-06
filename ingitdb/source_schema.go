package ingitdb

import "fmt"

// SourceSchemaDef records the portable source schema of an imported table.
// Foreign-key actions and secondary indexes are preserved for introspection,
// but are not enforced by inGitDB's record writer.
type SourceSchemaDef struct {
	KeyMode string `yaml:"key_mode,omitempty" json:"key_mode,omitempty"`
	// The following JSON payloads retain optional DALgo provider metadata
	// without making the native schema depend on a particular SQL dialect.
	SourceDefinitionJSON string `yaml:"source_definition_json,omitempty" json:"source_definition_json,omitempty"`
	SourceRightsJSON     string `yaml:"source_rights_json,omitempty" json:"source_rights_json,omitempty"`
	// ConstraintValidation records a provider-side source snapshot audit.
	// It does not mean native write-time enforcement.
	ConstraintValidation string `yaml:"constraint_validation,omitempty" json:"constraint_validation,omitempty"`
	// Raw tuple comparison is opt-in because SQL equality may apply collation
	// and affinity rules that differ from stored Go-value equality.
	RelationshipComparison string                `yaml:"relationship_comparison,omitempty" json:"relationship_comparison,omitempty"`
	Fields                 []SourceFieldDef      `yaml:"fields,omitempty" json:"fields,omitempty"`
	Indexes                []SourceIndexDef      `yaml:"indexes,omitempty" json:"indexes,omitempty"`
	ForeignKeys            []SourceForeignKeyDef `yaml:"foreign_keys,omitempty" json:"foreign_keys,omitempty"`
}

type SourceFieldDef struct {
	Name          string `yaml:"name" json:"name"`
	Type          string `yaml:"type" json:"type"`
	Nullable      bool   `yaml:"nullable" json:"nullable"`
	AutoIncrement bool   `yaml:"auto_increment,omitempty" json:"auto_increment,omitempty"`
	Precision     int    `yaml:"precision,omitempty" json:"precision,omitempty"`
	Scale         int    `yaml:"scale,omitempty" json:"scale,omitempty"`
	Length        *int   `yaml:"length,omitempty" json:"length,omitempty"`
	DefaultKind   string `yaml:"default_kind,omitempty" json:"default_kind,omitempty"`
	DefaultType   string `yaml:"default_type,omitempty" json:"default_type,omitempty"`
	DefaultJSON   string `yaml:"default_json,omitempty" json:"default_json,omitempty"`
	Encoding      string `yaml:"encoding,omitempty" json:"encoding,omitempty"`
}

type SourceIndexDef struct {
	Name   string   `yaml:"name" json:"name"`
	Fields []string `yaml:"fields" json:"fields"`
	Unique bool     `yaml:"unique,omitempty" json:"unique,omitempty"`
}

type SourceForeignKeyDef struct {
	Name                 string   `yaml:"name,omitempty" json:"name,omitempty"`
	Fields               []string `yaml:"fields" json:"fields"`
	ReferencedCollection string   `yaml:"referenced_collection" json:"referenced_collection"`
	ReferencedNamespace  string   `yaml:"referenced_namespace,omitempty" json:"referenced_namespace,omitempty"`
	ReferencedFields     []string `yaml:"referenced_fields" json:"referenced_fields"`
	SourceEnforcement    string   `yaml:"source_enforcement,omitempty" json:"source_enforcement,omitempty"`
	OnUpdate             string   `yaml:"on_update,omitempty" json:"on_update,omitempty"`
	OnDelete             string   `yaml:"on_delete,omitempty" json:"on_delete,omitempty"`
}

func (s *SourceSchemaDef) Validate(columns map[string]*ColumnDef) error {
	if s == nil {
		return nil
	}
	if s.KeyMode != "" && s.KeyMode != "source-primary-key" && s.KeyMode != "export-ordinal" {
		return fmt.Errorf("invalid source_schema.key_mode %q", s.KeyMode)
	}
	if s.RelationshipComparison != "" && s.RelationshipComparison != "raw" && s.RelationshipComparison != "provider-native" && s.RelationshipComparison != "unverified" {
		return fmt.Errorf("invalid source_schema.relationship_comparison %q", s.RelationshipComparison)
	}
	seen := map[string]bool{}
	for _, f := range s.Fields {
		if f.Name == "" || seen[f.Name] || columns[f.Name] == nil {
			return fmt.Errorf("invalid source_schema field %q", f.Name)
		}
		seen[f.Name] = true
		if f.Type == "" {
			return fmt.Errorf("source_schema field %q has no type", f.Name)
		}
	}
	for _, idx := range s.Indexes {
		if idx.Name == "" {
			return fmt.Errorf("invalid source_schema index %q", idx.Name)
		}
		for _, f := range idx.Fields {
			if columns[f] == nil {
				return fmt.Errorf("source_schema index %q references missing field %q", idx.Name, f)
			}
		}
	}
	for _, fk := range s.ForeignKeys {
		if fk.ReferencedCollection == "" || len(fk.Fields) == 0 || (len(fk.ReferencedFields) > 0 && len(fk.Fields) != len(fk.ReferencedFields)) {
			return fmt.Errorf("invalid source_schema foreign key %q", fk.Name)
		}
		for _, f := range fk.Fields {
			if columns[f] == nil {
				return fmt.Errorf("source_schema foreign key %q references missing field %q", fk.Name, f)
			}
		}
	}
	return nil
}

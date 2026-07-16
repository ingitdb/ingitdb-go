package ingitdb

// specscore: feature/record-count-constraints

import (
	"errors"
	"fmt"
)

type CollectionDef struct {
	ID      string `json:"-"` // Taken from dir name
	DirPath string `yaml:"-" json:"-"`
	// Inherits, when set, names a base partial definition to overlay under this
	// one. The value is a filesystem path resolved relative to the directory
	// containing this definition file (the `.collection/` schema directory, or
	// the `.collections/<name>/` directory in the shared layout). The base is a
	// partial CollectionDef: it need not be a loadable collection on its own and
	// commonly declares only shared columns. Resolution happens at definition
	// load (validator.ReadDefinition): columns merge by name (this definition
	// wins), other inheritable fields fill in where this one leaves them unset,
	// a base may itself declare `inherits` (chains are resolved transitively),
	// and a missing base or an inheritance cycle is a load-time error. After
	// resolution the field is cleared, so a fully-loaded definition never
	// carries it. See spec/features/definition-inheritance.
	Inherits     string                `yaml:"inherits,omitempty" json:"-"`
	Titles       map[string]string     `yaml:"titles,omitempty"`
	RecordFile   *RecordFileDef        `yaml:"record_file"`
	DataDir      string                `yaml:"data_dir,omitempty"`
	Columns      map[string]*ColumnDef `yaml:"columns"`
	ColumnsOrder []string              `yaml:"columns_order,omitempty"`
	// PrimaryKey lists the column names composing the source primary key,
	// in declared order. Persisted by CreateCollection so that
	// DescribeCollection can round-trip the real PK column names instead of
	// the synthesized "$key" placeholder. Omitted from older
	// definition.yaml files; callers should fall back to "$key" when empty.
	PrimaryKey  []string `yaml:"primary_key,omitempty"`
	DefaultView *ViewDef `yaml:"default_view,omitempty"`
	// SubCollections are not part of the collection definition file,
	// they are stored in the "subcollections" subdirectory as directories,
	// each containing their own .collection/definition.yaml.
	SubCollections map[string]*CollectionDef `yaml:"-" json:"-"`
	// Views are not part of the collection definition file,
	// they are stored in the "views" subdirectory.
	Views map[string]*ViewDef `yaml:"-" json:"-"`

	Readme *CollectionReadmeDef `yaml:"readme,omitempty" json:"readme,omitempty"`

	// MinRecordsCount and MaxRecordsCount constrain how many records the
	// collection may hold. They are collection-level integrity invariants:
	// `min_records_count: 1` means "this collection must not be empty",
	// `max_records_count: 0` means "this collection must be empty". A negative
	// bound, or a min above the max, is a definition-load error (Validate); a
	// record count outside the range is a validation error emitted during
	// whole-database validation (datavalidator).
	//
	// Pointer-typed on purpose, for the same reason as MinValue/MaxValue and
	// MinLength/MaxLength: a declared zero must be distinguishable from "not
	// declared". `max_records_count: 0` is meaningful, and with a plain int the
	// natural "!= 0" guard would read that declared zero as unset and enforce
	// nothing — the silent-no-op failure this whole validation stack exists to
	// end (geo-ingitdb declared these on three subcollections and inGitDB read
	// and dropped them; ingitdb-go#8).
	MinRecordsCount *int `yaml:"min_records_count,omitempty" json:"min_records_count,omitempty"`
	MaxRecordsCount *int `yaml:"max_records_count,omitempty" json:"max_records_count,omitempty"`

	// ConflictResolution overrides the database-level conflict-resolution
	// settings for this collection. Nil fields inherit the database default.
	ConflictResolution *ConflictResolutionConfig `yaml:"conflict_resolution,omitempty" json:"conflict_resolution,omitempty"`
}

type CollectionReadmeDef struct {
	HideColumns        bool     `yaml:"hide_columns,omitempty" json:"hide_columns,omitempty"`
	HideSubcollections bool     `yaml:"hide_subcollections,omitempty" json:"hide_subcollections,omitempty"`
	HideViews          bool     `yaml:"hide_views,omitempty" json:"hide_views,omitempty"`
	HideTriggers       bool     `yaml:"hide_triggers,omitempty" json:"hide_triggers,omitempty"`
	DataPreview        *ViewDef `yaml:"data_preview,omitempty" json:"data_preview,omitempty"`
}

func (r *CollectionReadmeDef) Validate() error {
	if r.DataPreview != nil {
		if r.DataPreview.Template == "" {
			r.DataPreview.Template = "md-table"
		}
		if err := r.DataPreview.Validate(); err != nil {
			return fmt.Errorf("invalid data_preview: %w", err)
		}
	}
	return nil
}

func (v *CollectionDef) Validate() error {
	if v.ID == "" {
		return fmt.Errorf("missing 'id' in collection definition")
	}
	if err := v.validateRecordCountBounds(); err != nil {
		return err
	}
	var allErrors []error
	if len(v.Columns) == 0 {
		return fmt.Errorf("missing 'columns' in collection definition")
	}
	for id, col := range v.Columns {
		if err := col.Validate(); err != nil {
			return fmt.Errorf("invalid column '%s': %w", id, err)
		}
		if col.Formula != "" {
			if err := validateComputedColumn(v.ID, id, col, v.Columns); err != nil {
				return err
			}
		}
		if col.RequiredWhen != "" {
			if err := validateRequiredWhen(v.ID, id, col, v.Columns); err != nil {
				return err
			}
		}
	}
	for i, colName := range v.ColumnsOrder {
		if _, ok := v.Columns[colName]; !ok {
			return fmt.Errorf("columns_order[%d] references unspecified column: %s", i, colName)
		}
		for j, prevCol := range v.ColumnsOrder[:i] {
			if prevCol == colName {
				return fmt.Errorf("duplicate value in columns_order at indexes %d and %d: %s", j, i, colName)
			}
		}
	}
	if v.RecordFile == nil {
		return fmt.Errorf("missing 'record_file' in collection definition")
	}
	if err := v.RecordFile.Validate(); err != nil {
		return fmt.Errorf("invalid record_file definition: %w", err)
	}
	if v.SubCollections != nil {
		for id, subColDef := range v.SubCollections {
			if err := subColDef.Validate(); err != nil {
				allErrors = append(allErrors, fmt.Errorf("invalid subcollection '%s': %w", id, err))
			}
		}
	}
	if v.Views != nil {
		for id, viewDef := range v.Views {
			if err := viewDef.Validate(); err != nil {
				allErrors = append(allErrors, fmt.Errorf("invalid view '%s': %w", id, err))
			}
		}
	}

	// Validate DefaultView if present
	if v.DefaultView != nil {
		v.DefaultView.ID = DefaultViewID
		if err := v.DefaultView.Validate(); err != nil {
			allErrors = append(allErrors, fmt.Errorf("invalid default_view: %w", err))
		}
	}

	// Check for multiple views with IsDefault == true
	defaultCount := 0
	for _, viewDef := range v.Views {
		if viewDef.IsDefault {
			defaultCount++
		}
	}
	if defaultCount > 1 {
		allErrors = append(allErrors, fmt.Errorf("multiple views with IsDefault set"))
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("%d errors: %w", len(allErrors), errors.Join(allErrors...))
	}

	if v.Readme != nil {
		if err := v.Readme.Validate(); err != nil {
			return fmt.Errorf("invalid readme: %w", err)
		}
	}

	return nil
}

// validateRecordCountBounds rejects a record-count bound that cannot describe
// any valid collection: a negative min or max (a collection can never hold a
// negative number of records), or a min above the max (no count satisfies
// both). An impossible bound is an author mistake in the schema itself, caught
// at definition-load rather than deferred to a validation-time finding — the
// same policy the column value-range constraints use for an inverted range.
func (v *CollectionDef) validateRecordCountBounds() error {
	if v.MinRecordsCount != nil && *v.MinRecordsCount < 0 {
		return fmt.Errorf("min_records_count must not be negative, got %d", *v.MinRecordsCount)
	}
	if v.MaxRecordsCount != nil && *v.MaxRecordsCount < 0 {
		return fmt.Errorf("max_records_count must not be negative, got %d", *v.MaxRecordsCount)
	}
	if v.MinRecordsCount != nil && v.MaxRecordsCount != nil && *v.MinRecordsCount > *v.MaxRecordsCount {
		return fmt.Errorf("min_records_count %d exceeds max_records_count %d", *v.MinRecordsCount, *v.MaxRecordsCount)
	}
	return nil
}

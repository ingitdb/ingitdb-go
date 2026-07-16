package ingitdb

import (
	"fmt"
	"slices"
	"strings"
)

// ValidateForeignKeys checks that every foreign_key in the definition names a
// collection the definition actually contains.
//
// This cannot live on CollectionDef.Validate: a collection cannot see its
// siblings, and a foreign_key resolves against the definition's root
// collections (the same lookup materializer/view_builder.go performs when it
// builds FK views). So it runs once, after all collections are loaded.
//
// A typo'd target is otherwise invisible. Nothing reads ColumnDef.ForeignKey
// during validation — the ForeignKeyIndex interface in datavalidator has no
// implementation and no callers — so `foreign_key: equivalance_classes` simply
// never resolves and never complains.
//
// Errors are collected across the whole definition rather than returning the
// first, so one pass reports every broken reference.
func ValidateForeignKeys(def *Definition) error {
	if def == nil {
		return nil
	}
	targets := make([]string, 0, len(def.Collections))
	for id := range def.Collections {
		targets = append(targets, id)
	}
	slices.Sort(targets)

	var problems []string
	var walk func(path string, cols map[string]*CollectionDef)
	walk = func(path string, cols map[string]*CollectionDef) {
		ids := make([]string, 0, len(cols))
		for id := range cols {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			col := cols[id]
			full := id
			if path != "" {
				full = path + "/" + id
			}
			names := make([]string, 0, len(col.Columns))
			for name := range col.Columns {
				names = append(names, name)
			}
			slices.Sort(names)
			for _, name := range names {
				fk := col.Columns[name].ForeignKey
				if fk == "" {
					continue
				}
				if _, ok := def.Collections[fk]; !ok {
					problems = append(problems, fmt.Sprintf(
						"collection '%s': column '%s' declares foreign_key '%s', which is not a collection in this definition (known collections: %s)",
						full, name, fk, strings.Join(targets, ", ")))
				}
			}
			walk(full, col.SubCollections)
		}
	}
	walk("", def.Collections)

	if len(problems) > 0 {
		return fmt.Errorf("invalid foreign keys:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

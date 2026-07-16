package ingitdb

import (
	"fmt"
	"slices"
	"strings"
)

// collectionModule returns the module prefix of a collection's full id: the
// segment before the first ".". Root collections register module-namespaced as
// `<module>.<name>` (`commerce.countries`, `geo.countries`), and a
// subcollection's full path keeps that dotted root ahead of any "/" segments
// (`commerce.orders/order_details`), so the module is always the text before
// the first ".". A collection id with no "." (e.g. can-i-use's `capabilities`)
// has no module.
func collectionModule(fullID string) string {
	if dot := strings.Index(fullID, "."); dot >= 0 {
		return fullID[:dot]
	}
	return ""
}

// ResolveForeignKey resolves a column's foreign_key to a root collection id,
// module-relative to the declaring collection.
//
//   - A foreign_key that already contains a "." is fully qualified and is looked
//     up as-is.
//   - A bare foreign_key is tried module-first: `<declaring-module>.<fk>`, then
//     the bare `<fk>`.
//
// Module-relative resolution keeps a module portable across mount points: a
// column in `commerce.addresses` says `foreign_key: countries` and reaches
// `commerce.countries` without hard-coding the mount, and without colliding with
// `geo.countries`. It returns the resolved id and whether a collection with that
// id exists.
func ResolveForeignKey(declaringFullID, fk string, collections map[string]*CollectionDef) (string, bool) {
	if strings.Contains(fk, ".") {
		_, ok := collections[fk]
		return fk, ok
	}
	if module := collectionModule(declaringFullID); module != "" {
		qualified := module + "." + fk
		if _, ok := collections[qualified]; ok {
			return qualified, true
		}
	}
	if _, ok := collections[fk]; ok {
		return fk, true
	}
	return "", false
}

// ValidateForeignKeys checks that every foreign_key in the definition resolves
// (module-relative, per ResolveForeignKey) to a collection the definition
// actually contains.
//
// This cannot live on CollectionDef.Validate: a collection cannot see its
// siblings, and a foreign_key resolves against the definition's root
// collections (the same lookup materializer/view_builder.go performs when it
// builds FK views). So it runs once, after all collections are loaded.
//
// A typo'd target is otherwise invisible. Before this, nothing read
// ColumnDef.ForeignKey during validation, so `foreign_key: equivalance_classes`
// simply never resolved and never complained.
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
				if _, ok := ResolveForeignKey(full, fk, def.Collections); !ok {
					problems = append(problems, fmt.Sprintf(
						"collection '%s': column '%s' declares foreign_key '%s', which does not resolve to any collection in this definition (known collections: %s)",
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

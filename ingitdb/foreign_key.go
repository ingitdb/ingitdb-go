package ingitdb

// specscore: feature/column-validation

import (
	"fmt"
	"reflect"
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

// ForeignKeyElements normalizes a foreign_key column's raw record value into
// the scalar values that a caller checks or groups against the target
// collection. Every caller that resolves FK values (the dangling-reference
// check in datavalidator, and the reverse-index $fk view builder in
// materializer) shares this so a list-valued column behaves the same way in
// both: previously each stringified the whole raw value with
// fmt.Sprintf("%v", raw), so a `type: any` or list column (`event_ids: [a,
// b]`) turned into the single literal value "[a b]" — never a real key, so
// every such record either failed FK validation or fell into one bogus $fk
// group.
//
//   - A scalar value (string, number, bool, ...) stringifies to itself, exactly
//     as fmt.Sprintf("%v", raw) always has — unchanged behaviour.
//   - A list value ([]any, []string, or any other slice/array) is walked
//     element by element: a nil element is skipped, a scalar element
//     stringifies on its own (so it is checked/grouped independently), and a
//     non-scalar element (a nested list or map) cannot be turned into a key at
//     all, so it is reported back via elementErrs instead of silently
//     stringifying into something like "[a b]".
//   - A map value keeps prior behaviour unchanged: the whole value stringifies
//     as one unit, the same as any other non-slice value.
//
// An element (or the whole scalar value) that stringifies to "" is omitted
// from values, matching the empty-string skip every caller already performed.
func ForeignKeyElements(raw any) (values []string, elementErrs []string) {
	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return nil, nil
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		if s := fmt.Sprintf("%v", raw); s != "" {
			values = append(values, s)
		}
		return values, nil
	}
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i).Interface()
		if elem == nil {
			continue
		}
		ev := reflect.ValueOf(elem)
		if ev.Kind() == reflect.Slice || ev.Kind() == reflect.Array || ev.Kind() == reflect.Map {
			elementErrs = append(elementErrs, fmt.Sprintf("%v", elem))
			continue
		}
		if s := fmt.Sprintf("%v", elem); s != "" {
			values = append(values, s)
		}
	}
	return values, elementErrs
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

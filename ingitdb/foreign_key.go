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
// collection. Every caller within ingitdb-go that resolves FK values (the
// dangling-reference check in datavalidator, and the reverse-index $fk view
// builder in materializer) shares this so a list-valued column behaves the
// same way in both: previously each stringified the whole raw value with
// fmt.Sprintf("%v", raw), so a `type: any` or list column (`event_ids: [a,
// b]`) turned into the single literal value "[a b]" — never a real key, so
// every such record either failed FK validation or fell into one bogus $fk
// group. (dalgo2ingitdb's own write/delete FK checks are a separate repo and
// do not go through this helper; that is tracked as a follow-up there, not
// fixed here: https://github.com/ingitdb/dalgo2ingitdb/issues/18.)
//
//   - A scalar value (string, number, bool, ...) stringifies to itself, exactly
//     as fmt.Sprintf("%v", raw) always has — unchanged behaviour.
//   - A pointer is dereferenced first, at any depth; a nil pointer (like a nil
//     interface value) means "nothing here" and is skipped, exactly like an
//     absent value.
//   - A []byte (or fixed-size byte array) is treated as one scalar — its
//     string form — never walked byte by byte.
//   - A list value ([]any, []string, or any other slice/array, excluding a
//     byte slice/array) is walked element by element: a nil or nil-pointer
//     element is skipped, a scalar element stringifies on its own (so it is
//     checked/grouped independently), and a non-scalar element (a nested list
//     or map) cannot be turned into a key at all, so it is reported back via
//     elementErrs instead of silently stringifying into something like "[a
//     b]". A repeated element is checked/grouped once — order-preserving
//     dedup, so `[e1, e1]` yields `["e1"]` and never a doubled validation
//     error or a duplicate row in a $fk view.
//   - A map value keeps prior behaviour unchanged: the whole value stringifies
//     as one unit, the same as any other non-slice value.
//
// An element (or the whole scalar value) that stringifies to "" is omitted
// from values, matching the empty-string skip every caller already performed.
func ForeignKeyElements(raw any) (values []string, elementErrs []string) {
	rv := dereferencedForeignKeyValue(reflect.ValueOf(raw))
	if !rv.IsValid() {
		return nil, nil
	}
	if s, ok := scalarForeignKeyString(rv); ok {
		if s != "" {
			values = append(values, s)
		}
		return values, nil
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		// A map (or anything else non-slice) keeps prior whole-value stringify
		// behaviour.
		if s := fmt.Sprintf("%v", rv.Interface()); s != "" {
			values = append(values, s)
		}
		return values, nil
	}
	seen := make(map[string]bool, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		ev := dereferencedForeignKeyValue(reflect.ValueOf(rv.Index(i).Interface()))
		if !ev.IsValid() {
			continue // a nil element, or a nil-pointer element, is skipped
		}
		if s, ok := scalarForeignKeyString(ev); ok {
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			values = append(values, s)
			continue
		}
		elementErrs = append(elementErrs, fmt.Sprintf("%v", ev.Interface()))
	}
	return values, elementErrs
}

// dereferencedForeignKeyValue follows pointer indirection at any depth,
// returning an invalid reflect.Value for a nil raw value or a nil pointer —
// both mean "nothing here" to every ForeignKeyElements caller.
func dereferencedForeignKeyValue(v reflect.Value) reflect.Value {
	for v.IsValid() && v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

// scalarForeignKeyString reports whether v is a scalar for FK purposes —
// anything that is not a slice/array/map — or a []byte/byte-array value,
// which is treated as one scalar (its string form) rather than walked
// element by element. It assumes v is already dereferenced and valid.
func scalarForeignKeyString(v reflect.Value) (string, bool) {
	if (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) && v.Type().Elem().Kind() == reflect.Uint8 {
		b := make([]byte, v.Len())
		for i := 0; i < v.Len(); i++ {
			b[i] = byte(v.Index(i).Uint())
		}
		return string(b), true
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array || v.Kind() == reflect.Map {
		return "", false
	}
	return fmt.Sprintf("%v", v.Interface()), true
}

// ForeignKeyValueIsList reports whether raw is a list-shaped FK value (a
// slice/array that ForeignKeyElements would walk element by element), as
// opposed to a scalar or a []byte-style scalar. materializer's $fk view
// builder uses this to decide whether a list-valued FK column must stay in
// the exported view (its value is not fully implied by the $fk partition the
// record landed in, unlike a scalar FK column's value).
func ForeignKeyValueIsList(raw any) bool {
	v := dereferencedForeignKeyValue(reflect.ValueOf(raw))
	if !v.IsValid() {
		return false
	}
	if _, isScalar := scalarForeignKeyString(v); isScalar {
		return false
	}
	return v.Kind() == reflect.Slice || v.Kind() == reflect.Array
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

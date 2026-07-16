package ingitdb

import (
	"fmt"
	"slices"
	"strings"
)

// computedColumnTypes lists the declared column types supported for computed
// (formula) columns in this Feature.
var computedColumnTypes = []ColumnType{
	ColumnTypeString,
	ColumnTypeInt,
	ColumnTypeFloat,
	ColumnTypeBool,
	ColumnTypeAny,
}

// validateComputedColumn validates a column whose Formula is non-empty.
// It enforces that:
//   - the column's own name does not shadow a Starlark builtin,
//   - the declared Type is one of the supported computed-column types, and
//   - the Formula resolves against the stored (non-computed) sibling columns.
//
// Errors name the collection and the column to aid diagnosis.
func validateComputedColumn(collectionID, colName string, col *ColumnDef, columns map[string]*ColumnDef) error {
	if err := validateComputedColumnName(collectionID, colName); err != nil {
		return err
	}
	if !slices.Contains(computedColumnTypes, col.Type) {
		return fmt.Errorf("collection '%s': computed column '%s' has unsupported type '%s': computed columns support only string, int, float, bool, and any",
			collectionID, colName, col.Type)
	}
	return validateFormulaExpr(collectionID, colName, "formula", col.Formula, columns)
}

// validateComputedColumnName rejects a computed column whose name is already a
// Starlark builtin.
//
// This is what keeps validateFormulaExpr's resolver-only approach sound: that
// approach detects a computed-column reference by the resolver reporting
// "undefined: X", but a name that lives in the universe is never undefined, so
// the check would silently never fire for a column named e.g. 'len'. It cannot
// be fixed by tightening the is-predeclared predicate — starlark.FileProgram
// hardwires resolve.File(f, isPredeclared, Universe.Has), and the isUniversal
// parameter exists to avoid a cyclic dependency on starlark.Universe, not so
// callers can redefine it.
func validateComputedColumnName(collectionID, colName string) error {
	if isFormulaBuiltin(colName) {
		return fmt.Errorf("collection '%s': computed column '%s' shadows a Starlark builtin of the same name: rename the column",
			collectionID, colName)
	}
	return nil
}

// validateFormulaExpr resolves a Starlark expression against the collection's
// stored columns, so an undeclared identifier or a reference to a computed
// column fails at definition-load time rather than silently at evaluation.
//
// Resolution is delegated to Starlark's own resolver rather than a hand-rolled
// walk over *syntax.Ident nodes. A walk cannot tell a field reference from a
// builtin or a comprehension-bound local: 'len', 'True' and 'abs' are all
// *syntax.Ident, so "every identifier must be a declared column" would reject
// len(tags) > 0 and [c for c in counties]. Supplying the real predeclared set
// — stored siblings plus the evaluator's universe — makes the resolver do it
// correctly, with no bespoke traversal.
//
// Computed siblings are deliberately excluded from the predeclared set: that
// is what turns a reference to one into "undefined: X", which is remapped here
// into a message naming the actual problem.
func validateFormulaExpr(collectionID, colName, kind, expr string, columns map[string]*ColumnDef) error {
	stored := make([]string, 0, len(columns))
	for name, def := range columns {
		if def.Formula == "" {
			stored = append(stored, name)
		}
	}

	if _, err := compileFormulaStrict(expr, stored); err != nil {
		if ref := computedColumnReference(err, columns); ref != "" {
			return fmt.Errorf("collection '%s': %s for column '%s' references computed column '%s': a %s may reference only stored fields",
				collectionID, kind, colName, ref, kind)
		}
		return fmt.Errorf("collection '%s': invalid %s for column '%s': %w", collectionID, kind, colName, err)
	}
	return nil
}

// computedColumnReference reports which computed column an "undefined: X"
// resolver error refers to, or "" if the undefined name is not a computed
// column of this collection.
func computedColumnReference(err error, columns map[string]*ColumnDef) string {
	for name, def := range columns {
		if def.Formula == "" {
			continue
		}
		if strings.Contains(err.Error(), "undefined: "+name) {
			return name
		}
	}
	return ""
}

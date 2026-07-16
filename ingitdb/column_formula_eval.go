package ingitdb

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// maxFormulaSteps caps the number of Starlark execution steps a single formula
// evaluation may take. It is a safety ceiling against pathological expressions
// (e.g. a comprehension over a large range) that could otherwise hang or
// exhaust memory while computing a column on read. Legitimate single-record
// formulas use a few dozen steps; this ceiling is far above any real formula
// yet bounds abuse.
const maxFormulaSteps = 1_000_000

// formulaResultVar is the synthetic global the compiled program assigns the
// formula's value to. The name is deliberately unlikely to collide with a
// real stored field.
const formulaResultVar = "__ingitdb_formula_result__"

// formulaProgramCache memoises compiled Starlark programs so a formula is
// parsed and compiled once rather than on every record read. Programs are
// immutable and safe to Init concurrently.
//
// The key is the formula source AND the predeclared set it was compiled
// against — never the source alone. Resolution depends on which names are
// predeclared, so the same source legitimately compiles in one collection and
// fails in another. Only successes are cached (an error returns before
// LoadOrStore), so a source-only key would let a collection that MUST fail
// take a hit from a different collection that compiled the same text
// successfully — silently, and dependent on load order.
var formulaProgramCache sync.Map // map[string]*starlark.Program

// formulaUniverse is the set of names predeclared for every formula regardless
// of collection: Starlark's universe plus the curated numeric helpers. It
// mirrors exactly what EvaluateFormula binds, so the resolver's view at load
// time matches the evaluator's at read time.
var formulaUniverse = func() starlark.StringDict {
	env := maps.Clone(starlark.Universe)
	addFormulaBuiltins(env)
	return env
}()

// isFormulaBuiltin reports whether name is predeclared for every formula.
// Used to reject computed column names that would shadow a universe member.
func isFormulaBuiltin(name string) bool {
	_, ok := formulaUniverse[name]
	return ok
}

// strictFormulaCacheKey builds the cache key for a formula compiled against the
// universe plus declared. The declared names are sorted and delimited so the key
// is stable regardless of map iteration order. The "strict" prefix keeps these
// entries in a separate namespace from open-mode ones, so the two compilations
// of one source can never be confused for each other.
func strictFormulaCacheKey(formula string, declared []string) string {
	names := slices.Clone(declared)
	slices.Sort(names)
	var sb strings.Builder
	sb.WriteString("strict")
	sb.WriteByte(0)
	sb.WriteString(formula)
	sb.WriteByte(0)
	for _, n := range names {
		sb.WriteString(n)
		sb.WriteByte(1)
	}
	return sb.String()
}

// EvaluateFormula evaluates a computed-column formula as a single Starlark
// expression in a deterministic, side-effect-free sandbox.
//
// Each entry of fields is bound as a top-level variable available to the
// expression. Supported Go input types are string, bool, int, int8, int16,
// int32, int64, uint, uint8, uint16, uint32, uint64, float32, and float64.
// The result is converted back to a Go-native value: string, bool, int64, or
// float64. A nil field value or a None result maps to/from Go nil.
//
// The sandbox exposes no network, filesystem, clock, or randomness, and
// installs no load() loader, so evaluation has zero side effects and is
// deterministic: identical formula and fields always yield identical output.
// Evaluation is bounded by maxFormulaSteps. The compiled program is cached, so
// repeated reads of the same formula do not re-parse it.
func EvaluateFormula(formula string, fields map[string]any) (any, error) {
	prog, err := compileFormulaOpen(formula)
	if err != nil {
		return nil, err
	}

	// Start from the shared universe so the standard deterministic builtins
	// (len, min, max, native string methods, ...) and the curated numeric
	// helpers remain available, then bind the record's fields last so a field
	// shadows a same-named builtin.
	predeclared := maps.Clone(formulaUniverse)
	for name, raw := range fields {
		v, convErr := goToStarlark(raw)
		if convErr != nil {
			return nil, fmt.Errorf("field '%s': %w", name, convErr)
		}
		predeclared[name] = v
	}

	thread := &starlark.Thread{
		Name: "formula",
		// Route print to a no-op so a reachable print() has no side effect.
		Print: func(_ *starlark.Thread, _ string) {},
	}
	thread.SetMaxExecutionSteps(maxFormulaSteps)

	globals, err := prog.Init(thread, predeclared)
	if err != nil {
		return nil, err
	}
	// The compiled program is a single assignment to formulaResultVar, so a
	// successful Init always binds it.
	return starlarkToGo(globals[formulaResultVar])
}

// compileFormulaStrict compiles a formula resolving every free identifier
// against formulaUniverse plus declared, so an undeclared identifier is
// rejected with "undefined: x" when the definition loads rather than surviving
// to evaluation. This is the load-time path; passing
// func(string) bool { return true } here is what let undeclared identifiers
// through before.
//
// The cache key includes declared, because resolution depends on it: the same
// source legitimately compiles against one collection's columns and fails
// against another's.
func compileFormulaStrict(formula string, declared []string) (*starlark.Program, error) {
	predeclared := make(map[string]bool, len(declared))
	for _, n := range declared {
		predeclared[n] = true
	}
	return compileFormulaWith(formula, strictFormulaCacheKey(formula, declared), func(name string) bool {
		return predeclared[name] || isFormulaBuiltin(name)
	})
}

// compileFormulaOpen compiles a formula treating every free identifier as
// predeclared. This is the evaluation path, and it is deliberately permissive
// for two reasons.
//
// First, declaredness is already enforced at load time by
// compileFormulaStrict, so re-resolving here adds no safety.
//
// Second, resolving against a record's fields would be wrong AND unbounded. A
// declared-but-omitted optional field is legitimately absent from a sparse
// record, so "undefined: count" would be a false error; and since the cache key
// must reflect the predeclared set, keying on per-record field names would mint
// an entry per distinct field shape — up to 2^n for n optional columns. Keying
// open-mode programs by source alone keeps that at one entry per formula, which
// is sound precisely because the predicate is constant.
//
// A formula naming something the record lacks still fails, at Init rather than
// at compile.
func compileFormulaOpen(formula string) (*starlark.Program, error) {
	return compileFormulaWith(formula, "open\x00"+formula, func(string) bool { return true })
}

// compileFormulaWith returns the cached compiled program for cacheKey,
// compiling and memoising it on first use. The formula is wrapped as a single
// assignment so its value is readable from the program's globals after Init.
func compileFormulaWith(formula, cacheKey string, isPredeclared func(string) bool) (*starlark.Program, error) {
	if cached, ok := formulaProgramCache.Load(cacheKey); ok {
		return cached.(*starlark.Program), nil
	}
	src := formulaResultVar + " = (" + formula + ")\n"
	var opts syntax.FileOptions
	_, prog, err := starlark.SourceProgramOptions(&opts, "formula", src, isPredeclared)
	if err != nil {
		return nil, err
	}
	actual, _ := formulaProgramCache.LoadOrStore(cacheKey, prog)
	return actual.(*starlark.Program), nil
}

// goToStarlark converts a supported Go value into its Starlark equivalent.
//
// Composite values are converted recursively so that list and map columns are
// usable from an expression: a []string column supports len(tags) > 0, and a
// map[string]any column supports constraints["maxButtons"]. Decoded JSON
// arrives as []any and map[string]any, so both spellings are handled.
//
// Converted composites are frozen. They are bound as predeclared values shared
// across evaluations, and Starlark's lists and dicts are mutable by default;
// freezing keeps evaluation side-effect-free, so one record's formula cannot
// mutate a value another evaluation observes.
func goToStarlark(v any) (starlark.Value, error) {
	switch t := v.(type) {
	case nil:
		return starlark.None, nil
	case bool:
		return starlark.Bool(t), nil
	case string:
		return starlark.String(t), nil
	case []any:
		return goSliceToStarlark(t)
	case []string:
		elems := make([]any, len(t))
		for i, s := range t {
			elems[i] = s
		}
		return goSliceToStarlark(elems)
	case map[string]any:
		return goMapToStarlark(t)
	case map[string]string:
		m := make(map[string]any, len(t))
		for k, s := range t {
			m[k] = s
		}
		return goMapToStarlark(m)
	case int:
		return starlark.MakeInt64(int64(t)), nil
	case int8:
		return starlark.MakeInt64(int64(t)), nil
	case int16:
		return starlark.MakeInt64(int64(t)), nil
	case int32:
		return starlark.MakeInt64(int64(t)), nil
	case int64:
		return starlark.MakeInt64(t), nil
	case uint:
		return starlark.MakeUint64(uint64(t)), nil
	case uint8:
		return starlark.MakeUint64(uint64(t)), nil
	case uint16:
		return starlark.MakeUint64(uint64(t)), nil
	case uint32:
		return starlark.MakeUint64(uint64(t)), nil
	case uint64:
		return starlark.MakeUint64(t), nil
	case float32:
		return starlark.Float(float64(t)), nil
	case float64:
		return starlark.Float(t), nil
	default:
		return nil, fmt.Errorf("unsupported field type %T", v)
	}
}

// goSliceToStarlark converts a Go slice into a frozen Starlark list.
func goSliceToStarlark(s []any) (starlark.Value, error) {
	elems := make([]starlark.Value, len(s))
	for i, raw := range s {
		v, err := goToStarlark(raw)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		elems[i] = v
	}
	list := starlark.NewList(elems)
	list.Freeze()
	return list, nil
}

// goMapToStarlark converts a Go string-keyed map into a frozen Starlark dict.
// Keys are inserted in sorted order so iteration is deterministic: Starlark
// dicts preserve insertion order, and Go map ranging does not.
func goMapToStarlark(m map[string]any) (starlark.Value, error) {
	dict := starlark.NewDict(len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		v, err := goToStarlark(m[k])
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k, err)
		}
		if err := dict.SetKey(starlark.String(k), v); err != nil {
			return nil, fmt.Errorf("key %q: %w", k, err)
		}
	}
	dict.Freeze()
	return dict, nil
}

// starlarkToGo converts a Starlark result value into a Go-native value.
func starlarkToGo(v starlark.Value) (any, error) {
	switch t := v.(type) {
	case starlark.NoneType:
		return nil, nil
	case starlark.Bool:
		return bool(t), nil
	case starlark.String:
		return string(t), nil
	case starlark.Int:
		i, ok := t.Int64()
		if !ok {
			return nil, fmt.Errorf("integer result %s does not fit in int64", t.String())
		}
		return i, nil
	case starlark.Float:
		return float64(t), nil
	default:
		return nil, fmt.Errorf("unsupported result type %s", v.Type())
	}
}

// addFormulaBuiltins installs the curated, deterministic numeric helpers as
// bare top-level names: abs, round, floor, and ceil. Starlark's universe
// already provides len, min, max, and the native string methods; no
// IO-capable or non-deterministic module is exposed.
func addFormulaBuiltins(env starlark.StringDict) {
	env["abs"] = starlark.NewBuiltin("abs", formulaAbs)
	env["round"] = starlark.NewBuiltin("round", formulaRound)
	env["floor"] = starlark.NewBuiltin("floor", formulaFloor)
	env["ceil"] = starlark.NewBuiltin("ceil", formulaCeil)
}

// formulaAbs returns the absolute value, preserving int vs float.
func formulaAbs(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var x starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &x); err != nil {
		return nil, err
	}
	switch t := x.(type) {
	case starlark.Int:
		if t.Sign() < 0 {
			return zeroInt.Sub(t), nil
		}
		return t, nil
	case starlark.Float:
		return starlark.Float(math.Abs(float64(t))), nil
	default:
		return nil, fmt.Errorf("%s: got %s, want int or float", b.Name(), x.Type())
	}
}

// formulaRound rounds to the nearest integer and returns an int.
func formulaRound(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return floatUnaryToInt(b, args, kwargs, math.Round)
}

// formulaFloor returns the greatest integer <= x as an int.
func formulaFloor(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return floatUnaryToInt(b, args, kwargs, math.Floor)
}

// formulaCeil returns the least integer >= x as an int.
func formulaCeil(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return floatUnaryToInt(b, args, kwargs, math.Ceil)
}

// zeroInt is a reusable zero used to negate integers without allocation churn.
var zeroInt = starlark.MakeInt(0)

// floatUnaryToInt applies fn to an int-or-float argument and returns an int.
func floatUnaryToInt(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple, fn func(float64) float64) (starlark.Value, error) {
	var x starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &x); err != nil {
		return nil, err
	}
	switch t := x.(type) {
	case starlark.Int:
		return t, nil
	case starlark.Float:
		return starlark.NumberToInt(starlark.Float(fn(float64(t))))
	default:
		return nil, fmt.Errorf("%s: got %s, want int or float", b.Name(), x.Type())
	}
}

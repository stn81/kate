package db

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/stn81/kate/orm/flavor"
	ksql "github.com/stn81/kate/orm/sql"
)

// RawQuery is a hand-written SQL template with structured args. Build it
// via Raw and pass to ScanAll / ScanOne / ScanIter / ExecRaw. Template
// placeholders are named (`{name}`) — no positional `?` to keep ordering
// implicit and arg/template drift out of the picture.
//
// Two kinds of arguments:
//   - Arg(name, value): the value becomes a parameterized placeholder
//     (?/$N depending on flavor); injection-safe.
//   - Subst(name, sql.Literal): the literal text is substituted directly.
//     The marker Literal type forces callers to explicitly cast bare
//     strings — grep "sql.Literal(" to audit every text-substitution site.
type RawQuery struct {
	template string
	args     []Arg
}

// Arg is a named parameter inside a RawQuery template.
type Arg struct {
	name string
	val  any
	// when subst is true, val must be ksql.Literal and is inserted as text;
	// otherwise val is bound to a flavor placeholder.
	subst bool
}

// Raw constructs a RawQuery from a template and named args. Each `{name}`
// in the template is resolved against the args list. Unmatched placeholders
// or unused args trigger a Build-time error.
func Raw(template string, args ...Arg) RawQuery {
	return RawQuery{template: template, args: args}
}

// NewArg constructs a value-binding argument. Prefer the package-level
// alias Arg() exported below.
func newArg(name string, val any) Arg { return Arg{name: name, val: val} }

// ArgOf constructs a value-binding argument (placeholder + bound value).
// Renamed from "Arg" the constructor to avoid colliding with the Arg type.
func ArgOf(name string, val any) Arg { return Arg{name: name, val: val} }

// Subst constructs a text-substitution argument. v must be a sql.Literal
// so every text-substitution site is grep-visible.
func Subst(name string, v ksql.Literal) Arg {
	return Arg{name: name, val: string(v), subst: true}
}

// Build compiles the raw template against a flavor, resolving placeholders
// and producing the final SQL + arg slice.
func (rq RawQuery) Build(f ksql.Flavor) (string, []any, error) {
	argByName := make(map[string]Arg, len(rq.args))
	for _, a := range rq.args {
		argByName[a.name] = a
	}
	var sb strings.Builder
	var bound []any
	idx := 0
	for {
		open := strings.IndexByte(rq.template[idx:], '{')
		if open < 0 {
			sb.WriteString(rq.template[idx:])
			break
		}
		open += idx
		// emit literal prefix
		sb.WriteString(rq.template[idx:open])
		// handle escape: "{{" → "{"
		if open+1 < len(rq.template) && rq.template[open+1] == '{' {
			sb.WriteByte('{')
			idx = open + 2
			continue
		}
		close := strings.IndexByte(rq.template[open:], '}')
		if close < 0 {
			return "", nil, fmt.Errorf("kate/db.Raw: unclosed '{' at offset %d", open)
		}
		close += open
		name := rq.template[open+1 : close]
		arg, ok := argByName[name]
		if !ok {
			return "", nil, fmt.Errorf("kate/db.Raw: placeholder %q has no matching Arg/Subst", name)
		}
		if arg.subst {
			// text substitution — emit verbatim
			s, ok := arg.val.(string)
			if !ok {
				return "", nil, fmt.Errorf("kate/db.Raw: Subst(%q) value is not a string", name)
			}
			sb.WriteString(s)
		} else {
			bound = append(bound, arg.val)
			sb.WriteString(f.Placeholder(len(bound)))
		}
		idx = close + 1
	}
	return sb.String(), bound, nil
}

// ----- Scan helpers -----

// ScanAll runs a RawQuery and scans every row into a []T. Like List but
// with a hand-written SQL string. Uses QueryContext directly — never
// PrepareContext — so it sidesteps the clickhouse-go v2 prepare-on-WITH
// bug that v1's Raw().QueryRows() routinely tripped.
func ScanAll[T any](ctx context.Context, exe Executor, rq RawQuery) ([]T, error) {
	sqlStr, args, err := buildRaw(exe, rq)
	if err != nil {
		return nil, &Error{Op: "ScanAll", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	rows, err := exe.queryContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("ScanAll", sqlStr, args, err)
		firePostError(exe, ctx, "ScanAll", sqlStr, args, err)
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, wrapErr("ScanAll", sqlStr, args, err)
	}
	tType := reflect.TypeOf((*T)(nil)).Elem()
	mapper, err := buildMapper(tType, columns)
	if err != nil {
		return nil, wrapErr("ScanAll", sqlStr, args, err)
	}
	sliceVal := reflect.New(reflect.SliceOf(tType)).Elem()
	if _, err := scanInto(rows, mapper, columns, sliceVal, tType, false, false); err != nil {
		err = wrapErr("ScanAll", sqlStr, args, err)
		firePostError(exe, ctx, "ScanAll", sqlStr, args, err)
		return nil, err
	}
	firePostQuery(exe, ctx, "ScanAll", sqlStr, args, time.Since(start))
	return sliceVal.Interface().([]T), nil
}

// ScanOne runs a RawQuery and scans the single resulting row into *T.
// Returns ErrNoRows / ErrTooManyRows like Get.
func ScanOne[T any](ctx context.Context, exe Executor, rq RawQuery) (*T, error) {
	sqlStr, args, err := buildRaw(exe, rq)
	if err != nil {
		return nil, &Error{Op: "ScanOne", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	rows, err := exe.queryContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("ScanOne", sqlStr, args, err)
		firePostError(exe, ctx, "ScanOne", sqlStr, args, err)
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, wrapErr("ScanOne", sqlStr, args, err)
	}
	tType := reflect.TypeOf((*T)(nil)).Elem()
	mapper, err := buildMapper(tType, columns)
	if err != nil {
		return nil, wrapErr("ScanOne", sqlStr, args, err)
	}
	sliceVal := reflect.New(reflect.SliceOf(tType)).Elem()
	n, err := scanInto(rows, mapper, columns, sliceVal, tType, false, true)
	if err != nil {
		return nil, wrapErr("ScanOne", sqlStr, args, err)
	}
	firePostQuery(exe, ctx, "ScanOne", sqlStr, args, time.Since(start))
	if n == 0 {
		return nil, ErrNoRows
	}
	if rows.Next() {
		return nil, ErrTooManyRows
	}
	out := sliceVal.Index(0).Addr().Interface().(*T)
	return out, nil
}

// ScanIter runs a RawQuery and returns a streaming Iterator[T]. Caller
// must Close the iterator.
func ScanIter[T any](ctx context.Context, exe Executor, rq RawQuery) (Iterator[T], error) {
	sqlStr, args, err := buildRaw(exe, rq)
	if err != nil {
		return nil, &Error{Op: "ScanIter", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	rows, err := exe.queryContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("ScanIter", sqlStr, args, err)
		firePostError(exe, ctx, "ScanIter", sqlStr, args, err)
		return nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return nil, wrapErr("ScanIter", sqlStr, args, err)
	}
	tType := reflect.TypeOf((*T)(nil)).Elem()
	mapper, err := buildMapper(tType, columns)
	if err != nil {
		_ = rows.Close()
		return nil, wrapErr("ScanIter", sqlStr, args, err)
	}
	firePostQuery(exe, ctx, "ScanIter", sqlStr, args, time.Since(start))
	return &rowIter[T]{rows: rows, mapper: mapper, columns: columns, tType: tType, scanBuf: make([]any, len(columns))}, nil
}

// ScanRows runs a RawQuery and returns the raw *sql.Rows for callers that
// need to drive scanning manually (e.g. dynamic wide-column result sets
// where the column list isn't known at compile time). Caller is
// responsible for Close.
func ScanRows(ctx context.Context, exe Executor, rq RawQuery) (*stdsql.Rows, error) {
	sqlStr, args, err := buildRaw(exe, rq)
	if err != nil {
		return nil, &Error{Op: "ScanRows", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	rows, err := exe.queryContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("ScanRows", sqlStr, args, err)
		firePostError(exe, ctx, "ScanRows", sqlStr, args, err)
		return nil, err
	}
	firePostQuery(exe, ctx, "ScanRows", sqlStr, args, time.Since(start))
	return rows, nil
}

// ExecRaw runs a non-SELECT RawQuery (INSERT / UPDATE / DELETE / DDL).
func ExecRaw(ctx context.Context, exe Executor, rq RawQuery) (Result, error) {
	sqlStr, args, err := buildRaw(exe, rq)
	if err != nil {
		return Result{}, &Error{Op: "ExecRaw", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	res, err := exe.execContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("ExecRaw", sqlStr, args, err)
		firePostError(exe, ctx, "ExecRaw", sqlStr, args, err)
		return Result{}, err
	}
	firePostQuery(exe, ctx, "ExecRaw", sqlStr, args, time.Since(start))
	return Result{inner: res}, nil
}

func buildRaw(exe Executor, rq RawQuery) (string, []any, error) {
	return rq.Build(exe.flavorOf())
}

// ----- forward sentinel for ergonomic import (db.Flavor type alias) -----

// FlavorOf is a helper that returns the flavor associated with an Executor;
// occasionally useful for callers writing flavor-aware code outside of
// kate's typed builders.
func FlavorOf(exe Executor) flavor.Flavor { return exe.flavorOf() }

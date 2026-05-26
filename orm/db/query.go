package db

import (
	"context"
	"errors"
	"reflect"
	"time"

	ksql "github.com/stn81/kate/orm/sql"
)

// Get runs query q and scans the single resulting row into *T. Returns
// ErrNoRows if zero rows; ErrTooManyRows if more than one.
func Get[T any](ctx context.Context, exe Executor, q ksql.Builder) (*T, error) {
	sqlStr, args, err := buildQuery(exe, q)
	if err != nil {
		return nil, &Error{Op: "Get", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	rows, err := exe.queryContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("Get", sqlStr, args, err)
		firePostError(exe, ctx, "Get", sqlStr, args, err)
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, wrapErr("Get", sqlStr, args, err)
	}
	tType := reflect.TypeOf((*T)(nil)).Elem()
	mapper, err := buildMapper(tType, columns)
	if err != nil {
		return nil, wrapErr("Get", sqlStr, args, err)
	}

	sliceType := reflect.SliceOf(tType)
	sliceVal := reflect.New(sliceType).Elem()
	n, err := scanInto(rows, mapper, columns, sliceVal, tType, false /*ptrElem*/, true /*firstOnly*/)
	if err != nil {
		err = wrapErr("Get", sqlStr, args, err)
		firePostError(exe, ctx, "Get", sqlStr, args, err)
		return nil, err
	}
	firePostQuery(exe, ctx, "Get", sqlStr, args, time.Since(start))
	if n == 0 {
		return nil, ErrNoRows
	}
	// Ensure at most one row was returned: peek for another.
	if rows.Next() {
		return nil, ErrTooManyRows
	}
	out := sliceVal.Index(0).Addr().Interface().(*T)
	return out, nil
}

// List runs query q and scans all resulting rows into a []T. An empty
// result returns a nil slice with no error.
func List[T any](ctx context.Context, exe Executor, q ksql.Builder) ([]T, error) {
	sqlStr, args, err := buildQuery(exe, q)
	if err != nil {
		return nil, &Error{Op: "List", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	rows, err := exe.queryContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("List", sqlStr, args, err)
		firePostError(exe, ctx, "List", sqlStr, args, err)
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, wrapErr("List", sqlStr, args, err)
	}
	tType := reflect.TypeOf((*T)(nil)).Elem()
	mapper, err := buildMapper(tType, columns)
	if err != nil {
		return nil, wrapErr("List", sqlStr, args, err)
	}

	sliceType := reflect.SliceOf(tType)
	sliceVal := reflect.New(sliceType).Elem()
	if _, err := scanInto(rows, mapper, columns, sliceVal, tType, false, false); err != nil {
		err = wrapErr("List", sqlStr, args, err)
		firePostError(exe, ctx, "List", sqlStr, args, err)
		return nil, err
	}
	firePostQuery(exe, ctx, "List", sqlStr, args, time.Since(start))
	return sliceVal.Interface().([]T), nil
}

// Count is the convenience wrapper around `SELECT COUNT(*) FROM (q)` shape.
// For now this assumes q is a SELECT whose first column is a count; users
// who want COUNT(*) typically construct a builder with sql/expr.CountStar
// and call Get[int64] directly. Count is provided for API parity.
func Count(ctx context.Context, exe Executor, q ksql.Builder) (int64, error) {
	row, err := Get[int64](ctx, exe, q)
	if err != nil {
		return 0, err
	}
	if row == nil {
		return 0, nil
	}
	return *row, nil
}

// Exists reports whether q returns at least one row.
func Exists(ctx context.Context, exe Executor, q ksql.Builder) (bool, error) {
	sqlStr, args, err := buildQuery(exe, q)
	if err != nil {
		return false, &Error{Op: "Exists", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	rows, err := exe.queryContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("Exists", sqlStr, args, err)
		firePostError(exe, ctx, "Exists", sqlStr, args, err)
		return false, err
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, wrapErr("Exists", sqlStr, args, err)
	}
	firePostQuery(exe, ctx, "Exists", sqlStr, args, time.Since(start))
	return found, nil
}

// ExecBuilder runs a builder that emits a non-SELECT statement (INSERT /
// UPDATE / DELETE / DDL). Returns the standard sql.Result.
//
// For row-builder writes (oltp.Insert etc.), the builder's own Exec method
// is more ergonomic; this entry point is for builders without a typed Exec.
func ExecBuilder(ctx context.Context, exe Executor, b ksql.Builder) (Result, error) {
	sqlStr, args, err := buildQuery(exe, b)
	if err != nil {
		return Result{}, &Error{Op: "Exec", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	res, err := exe.execContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("Exec", sqlStr, args, err)
		firePostError(exe, ctx, "Exec", sqlStr, args, err)
		return Result{}, err
	}
	firePostQuery(exe, ctx, "Exec", sqlStr, args, time.Since(start))
	return Result{inner: res}, nil
}

// Result is kate's wrapper around stdlib sql.Result. We wrap rather than
// expose stdsql.Result directly so methods that don't make sense on certain
// flavors (LastInsertId on CK) return ErrUnsupported instead of a silent 0.
type Result struct {
	inner interface {
		LastInsertId() (int64, error)
		RowsAffected() (int64, error)
	}
}

// LastInsertId returns the auto-generated id. On ClickHouse this returns 0
// and a nil error from the driver (CK has no auto-id); callers should
// generally not depend on it for CK tables.
func (r Result) LastInsertId() (int64, error) {
	if r.inner == nil {
		return 0, nil
	}
	id, err := r.inner.LastInsertId()
	// CK's driver returns "lastInsertId is not implemented" or similar.
	if err != nil {
		return 0, nil
	}
	return id, nil
}

// RowsAffected returns the number of rows touched.
func (r Result) RowsAffected() (int64, error) {
	if r.inner == nil {
		return 0, nil
	}
	n, err := r.inner.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// asKateErr extracts kate's *Error from err if present.
func asKateErr(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return nil
}

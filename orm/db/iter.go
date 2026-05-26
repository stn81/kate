package db

import (
	"context"
	stdsql "database/sql"
	"reflect"
	"time"

	ksql "github.com/stn81/kate/orm/sql"
)

// Iterator is a streaming row reader. Callers loop with Next, read the
// current row via Row, and must call Close when done.
type Iterator[T any] interface {
	Next() bool
	Row() T
	Err() error
	Close() error
}

// Iter runs query q and returns a streaming iterator of T. The caller is
// responsible for calling Close (deferring on the returned Iterator).
// Use Iter instead of List when row count is large or unbounded.
func Iter[T any](ctx context.Context, exe Executor, q ksql.Builder) (Iterator[T], error) {
	sqlStr, args, err := buildQuery(exe, q)
	if err != nil {
		return nil, &Error{Op: "Iter", Cause: err, Code: CodeSyntax}
	}
	start := time.Now()
	rows, err := exe.queryContext(ctx, sqlStr, args...)
	if err != nil {
		err = wrapErr("Iter", sqlStr, args, err)
		firePostError(exe, ctx, "Iter", sqlStr, args, err)
		return nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return nil, wrapErr("Iter", sqlStr, args, err)
	}
	tType := reflect.TypeOf((*T)(nil)).Elem()
	mapper, err := buildMapper(tType, columns)
	if err != nil {
		_ = rows.Close()
		return nil, wrapErr("Iter", sqlStr, args, err)
	}
	firePostQuery(exe, ctx, "Iter", sqlStr, args, time.Since(start))
	return &rowIter[T]{
		rows:    rows,
		mapper:  mapper,
		columns: columns,
		tType:   tType,
		scanBuf: make([]any, len(columns)),
	}, nil
}

type rowIter[T any] struct {
	rows    *stdsql.Rows
	mapper  *rowMapper
	columns []string
	tType   reflect.Type
	scanBuf []any

	current  T
	scanned  bool
	closeErr error
	rowErr   error
}

func (it *rowIter[T]) Next() bool {
	if !it.rows.Next() {
		return false
	}
	newElem := reflect.New(it.tType).Elem()
	for i := 0; i < len(it.columns); i++ {
		path := it.mapper.fields[i]
		if path == nil {
			var scrap any
			it.scanBuf[i] = &scrap
			continue
		}
		fv := newElem
		for _, idx := range path {
			if fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				fv = fv.Elem()
			}
			fv = fv.Field(idx)
		}
		it.scanBuf[i] = fv.Addr().Interface()
	}
	if err := it.rows.Scan(it.scanBuf...); err != nil {
		it.rowErr = err
		return false
	}
	it.current = newElem.Interface().(T)
	it.scanned = true
	return true
}

func (it *rowIter[T]) Row() T {
	return it.current
}

func (it *rowIter[T]) Err() error {
	if it.rowErr != nil {
		return it.rowErr
	}
	if err := it.rows.Err(); err != nil {
		return err
	}
	return nil
}

func (it *rowIter[T]) Close() error {
	it.closeErr = it.rows.Close()
	return it.closeErr
}

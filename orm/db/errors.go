package db

import (
	"errors"
	"fmt"
)

// Sentinel errors callers compare against via errors.Is.
var (
	// ErrNoRows is returned by Get / ScanOne when the query produced zero rows.
	ErrNoRows = errors.New("kate/db: no rows")

	// ErrFlavorMismatch is returned when a builder's dialect doesn't match
	// the target *DB (e.g. oltp.UpdateBuilder against a ClickHouse DB).
	ErrFlavorMismatch = errors.New("kate/db: builder flavor incompatible with DB")

	// ErrTooManyRows is returned by Get when the query produced more than
	// one row. Use List if multiple rows are acceptable.
	ErrTooManyRows = errors.New("kate/db: Get returned more than one row")

	// ErrClosed is returned by operations on a closed *DB.
	ErrClosed = errors.New("kate/db: DB is closed")

	// ErrPartitionRequired is returned by olap.MutateBuilder.Build when
	// RequirePartition was not called. Defined in db (rather than olap) so
	// callers can compare via errors.Is without importing olap.
	ErrPartitionRequired = errors.New("kate/db: olap mutation requires partition predicate (call RequirePartition)")
)

// ErrCode is the typed error category exposed by *Error. Flavor packages
// translate driver-specific error codes (MySQL 1062, CK 159, etc.) into
// these values.
type ErrCode int

const (
	CodeUnknown ErrCode = iota
	CodeNoRows
	CodeConflict
	CodeDeadlock
	CodeTimeout
	CodeFlavorMismatch
	CodeSyntax
	CodePermission
	CodeConnection
)

func (c ErrCode) String() string {
	switch c {
	case CodeNoRows:
		return "no_rows"
	case CodeConflict:
		return "conflict"
	case CodeDeadlock:
		return "deadlock"
	case CodeTimeout:
		return "timeout"
	case CodeFlavorMismatch:
		return "flavor_mismatch"
	case CodeSyntax:
		return "syntax"
	case CodePermission:
		return "permission"
	case CodeConnection:
		return "connection"
	}
	return "unknown"
}

// Error is the rich error type returned by db operations. It carries the
// failing op name, the compiled SQL + args, the underlying driver error,
// and a typed Code for callers that want to react programmatically without
// reading driver-specific text.
type Error struct {
	Op    string
	Query string
	Args  []any
	Cause error
	Code  ErrCode
}

// Error renders a single-line description; for the underlying driver error
// use errors.Unwrap or errors.As.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause == nil {
		return fmt.Sprintf("kate/db: %s: %s", e.Op, e.Code)
	}
	return fmt.Sprintf("kate/db: %s: %s: %v", e.Op, e.Code, e.Cause)
}

// Unwrap exposes the underlying cause for errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// wrapErr constructs an *Error with op + query + args + cause.
func wrapErr(op, query string, args []any, cause error) error {
	if cause == nil {
		return nil
	}
	return &Error{Op: op, Query: query, Args: args, Cause: cause, Code: classifyCause(cause)}
}

// classifyCause maps a known error to its ErrCode. Sentinels go first;
// driver-text matching is left to flavor-specific translators that wrap
// the cause before it reaches here.
func classifyCause(err error) ErrCode {
	switch {
	case errors.Is(err, ErrNoRows):
		return CodeNoRows
	case errors.Is(err, ErrFlavorMismatch):
		return CodeFlavorMismatch
	}
	// Driver-specific code translation: implementations can wrap the
	// driver error in a *Error before returning, but as a fallback we
	// expose CodeUnknown.
	return CodeUnknown
}

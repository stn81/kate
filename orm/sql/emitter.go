package sql

import (
	"strings"

	"github.com/stn81/kate/orm/flavor"
)

// Flavor is the dialect protocol consumed by Emitter. Alias of flavor.Flavor
// so call sites can use sql.Flavor without an extra import.
type Flavor = flavor.Flavor

// Emitter accumulates SQL text and parameter values as a Builder walks its
// AST. It is the single sink for SQL text — no AST node writes to anything
// else.
type Emitter struct {
	buf    strings.Builder
	args   []any
	flavor Flavor
	err    error
}

// NewEmitter creates an Emitter that emits SQL in the given flavor.
func NewEmitter(f Flavor) *Emitter {
	return &Emitter{flavor: f}
}

// Flavor returns the dialect this emitter is producing for.
func (e *Emitter) Flavor() Flavor { return e.flavor }

// WriteString appends raw SQL text. No quoting, no escaping — callers must
// pre-quote identifiers via Flavor.Quote and pass values via Param.
func (e *Emitter) WriteString(s string) { e.buf.WriteString(s) }

// WriteByte appends a single ASCII byte. Returns error to satisfy
// io.ByteWriter; strings.Builder.WriteByte never errors so this is always
// nil.
func (e *Emitter) WriteByte(b byte) error { return e.buf.WriteByte(b) }

// Param appends a placeholder and records the argument value.
func (e *Emitter) Param(v any) {
	e.args = append(e.args, v)
	e.buf.WriteString(e.flavor.Placeholder(len(e.args)))
}

// Quote appends a flavor-quoted identifier.
func (e *Emitter) Quote(ident string) { e.buf.WriteString(e.flavor.Quote(ident)) }

// QualifiedColumn appends a flavor-quoted "schema.table.col" or "table.col" or
// "alias.col" depending on which components are non-empty.
func (e *Emitter) QualifiedColumn(schema, table, alias, col string) {
	q := e.flavor.Quote
	switch {
	case alias != "":
		e.buf.WriteString(q(alias))
		e.buf.WriteByte('.')
		e.buf.WriteString(q(col))
	case table != "":
		if schema != "" {
			e.buf.WriteString(q(schema))
			e.buf.WriteByte('.')
		}
		e.buf.WriteString(q(table))
		e.buf.WriteByte('.')
		e.buf.WriteString(q(col))
	default:
		e.buf.WriteString(q(col))
	}
}

// QualifiedTable appends a flavor-quoted schema.table reference (no alias).
func (e *Emitter) QualifiedTable(schema, table string) {
	q := e.flavor.Quote
	if schema != "" {
		e.buf.WriteString(q(schema))
		e.buf.WriteByte('.')
	}
	e.buf.WriteString(q(table))
}

// SetError records the first non-nil error encountered during emission.
// Subsequent calls are no-ops; the error surfaces via Result.
func (e *Emitter) SetError(err error) {
	if err == nil || e.err != nil {
		return
	}
	e.err = err
}

// Err returns the first emission error, or nil.
func (e *Emitter) Err() error { return e.err }

// Result returns the accumulated SQL string and bound arguments. After this
// call the emitter should not be reused.
func (e *Emitter) Result() (string, []any, error) {
	if e.err != nil {
		return "", nil, e.err
	}
	return e.buf.String(), e.args, nil
}

// Builder is the universal interface for anything that produces a top-level
// SQL statement. *SelectBuilder, *ch.SelectBuilder, *oltp.InsertBuilder[T]
// and db.RawQuery all satisfy it.
type Builder interface {
	Build(f Flavor) (string, []any, error)
}

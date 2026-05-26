// Package olap provides ClickHouse-only write builders: MutateBuilder
// (ALTER TABLE … UPDATE/DELETE, partition-scoped) and BatchInserter[T]
// (native protocol bulk insert).
//
// olap exists separately from oltp so OLAP applications import the right
// package and get the right semantics — MutateBuilder requires a partition
// predicate, mutations return a MutationID (the operation is asynchronous),
// BatchInserter goes through clickhouse-go's PrepareBatch rather than the
// row-by-row INSERT path.
package olap

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"

	"github.com/stn81/kate/orm/db"
	ksql "github.com/stn81/kate/orm/sql"
)

// MutateBuilder represents an ALTER TABLE … UPDATE or DELETE statement.
// Build refuses to emit unless RequirePartition has been called: CK
// mutations on unpartitioned scopes are extremely expensive and almost
// always a bug.
type MutateBuilder struct {
	table     ksql.TableRef
	partition ksql.Predicate
	setCols   []string
	setExprs  map[string]ksql.AnyExpr
	deleteOp  bool // mode: true = DELETE, false = UPDATE
}

// Mutate starts a MutateBuilder against a CK table.
func Mutate(t ksql.TableRef) *MutateBuilder {
	return &MutateBuilder{table: t, setExprs: map[string]ksql.AnyExpr{}}
}

// RequirePartition declares the partition-scope predicate that bounds this
// mutation. Required: Build returns ErrPartitionRequired if omitted.
func (b *MutateBuilder) RequirePartition(p ksql.Predicate) *MutateBuilder {
	out := *b
	out.partition = p
	return &out
}

// Set sets a column to a value expression (UPDATE form). Untyped variant;
// prefer SetTyped[T] for compile-time type matching.
func (b *MutateBuilder) Set(col ksql.AnyCol, e ksql.AnyExpr) *MutateBuilder {
	out := *b
	out.setExprs = copyMap(b.setExprs)
	name := columnNameOf(col)
	if _, exists := out.setExprs[name]; !exists {
		out.setCols = append(append([]string(nil), b.setCols...), name)
	} else {
		out.setCols = append([]string(nil), b.setCols...)
	}
	out.setExprs[name] = e
	return &out
}

// AsDelete switches mutation mode from UPDATE to DELETE. No Set calls
// are needed for DELETE; partition predicate is still required.
func (b *MutateBuilder) AsDelete() *MutateBuilder {
	out := *b
	out.deleteOp = true
	return &out
}

// Build compiles the ALTER TABLE … UPDATE / DELETE statement.
func (b *MutateBuilder) Build(f ksql.Flavor) (string, []any, error) {
	if b.partition == nil {
		return "", nil, db.ErrPartitionRequired
	}
	if !b.deleteOp && len(b.setCols) == 0 {
		return "", nil, fmt.Errorf("kate/olap.Mutate: no Set / SetTyped calls for UPDATE mode")
	}
	em := ksql.NewEmitter(f)
	em.WriteString("ALTER TABLE ")
	if b.table.Schema() != "" {
		em.WriteString(f.Quote(b.table.Schema()))
		_ = em.WriteByte('.')
	}
	em.WriteString(f.Quote(b.table.Name()))
	if b.deleteOp {
		em.WriteString(" DELETE WHERE ")
	} else {
		em.WriteString(" UPDATE ")
		for i, col := range b.setCols {
			if i > 0 {
				em.WriteString(", ")
			}
			em.WriteString(f.Quote(col))
			em.WriteString(" = ")
			b.setExprs[col].Emit(em)
		}
		em.WriteString(" WHERE ")
	}
	_ = em.WriteByte('(')
	b.partition.Emit(em)
	_ = em.WriteByte(')')
	return em.Result()
}

// Update runs the mutation in UPDATE mode and returns a MutationID for
// later polling via db.WaitMutation.
func (b *MutateBuilder) Update(ctx context.Context, exe db.Executor) (MutationID, error) {
	if db.FlavorOf(exe).Name() != "ClickHouse" {
		return "", db.ErrFlavorMismatch
	}
	if _, err := db.ExecBuilder(ctx, exe, b); err != nil {
		return "", err
	}
	return mintMutationID(b.table.Name(), "update"), nil
}

// Delete runs the mutation in DELETE mode.
func (b *MutateBuilder) Delete(ctx context.Context, exe db.Executor) (MutationID, error) {
	if db.FlavorOf(exe).Name() != "ClickHouse" {
		return "", db.ErrFlavorMismatch
	}
	out := b.AsDelete()
	if _, err := db.ExecBuilder(ctx, exe, out); err != nil {
		return "", err
	}
	return mintMutationID(b.table.Name(), "delete"), nil
}

// SetTyped is the typed entry point: col is Col[T], rhs is Expr[T].
// CK mutations frequently use "col = col + 1" style expressions, so the
// RHS is Expr[T] rather than a bare value (unlike oltp.SetTyped which
// takes T directly).
func SetTyped[T any](b *MutateBuilder, col ksql.Col[T], e ksql.Expr[T]) *MutateBuilder {
	return b.Set(col, e)
}

// MutationID identifies an asynchronous CK mutation. The CK driver doesn't
// surface a direct handle, so we synthesize a local ID for tracing; tools
// that want to poll system.mutations use the table name + a timestamp
// captured here.
type MutationID string

var mutationCounter atomic.Uint64

func mintMutationID(table, op string) MutationID {
	n := mutationCounter.Add(1)
	return MutationID(fmt.Sprintf("%s/%s/%d", table, op, n))
}

// columnNameOf extracts the bare column name from an AnyCol. Reuses the
// same trick as oltp: emit with a flavor that doesn't quote, strip.
func columnNameOf(c ksql.AnyCol) string {
	em := ksql.NewEmitter(stripFlavor{})
	c.Emit(em)
	s, _, _ := em.Result()
	// strip "table.col" prefix; CK doesn't accept qualified col in SET LHS
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[i+1:]
		}
	}
	return s
}

type stripFlavor struct{}

func (stripFlavor) Name() string             { return "strip" }
func (stripFlavor) Quote(s string) string    { return s }
func (stripFlavor) Placeholder(i int) string { return "?" }
func (stripFlavor) SupportsCTE() bool        { return true }
func (stripFlavor) SupportsReturning() bool  { return false }

func copyMap(m map[string]ksql.AnyExpr) map[string]ksql.AnyExpr {
	out := make(map[string]ksql.AnyExpr, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// sortedKeys is exposed for deterministic emit tests.
func sortedKeys(m map[string]ksql.AnyExpr) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

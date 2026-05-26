package oltp

import (
	"context"
	"fmt"

	"github.com/stn81/kate/orm/db"
	ksql "github.com/stn81/kate/orm/sql"
)

// DeleteBuilder builds DELETE statements. Construct via DeleteFrom; the
// WHERE clause is required (omitting it returns a build error to prevent
// accidental "delete entire table").
type DeleteBuilder struct {
	table ksql.TableRef
	where []ksql.Predicate
}

// DeleteFrom starts a DeleteBuilder.
func DeleteFrom(t ksql.TableRef) *DeleteBuilder {
	return &DeleteBuilder{table: t}
}

// Where appends AND-joined predicates.
func (b *DeleteBuilder) Where(ps ...ksql.Predicate) *DeleteBuilder {
	out := *b
	out.where = append(append([]ksql.Predicate(nil), b.where...), ps...)
	return &out
}

// Build compiles the DELETE.
func (b *DeleteBuilder) Build(f ksql.Flavor) (string, []any, error) {
	if len(b.where) == 0 {
		return "", nil, fmt.Errorf("kate/oltp.Delete: WHERE is required (refusing full-table DELETE)")
	}
	em := ksql.NewEmitter(f)
	em.WriteString("DELETE FROM ")
	emitTableNoAlias(em, b.table, f)
	em.WriteString(" WHERE ")
	emitAndPreds(em, b.where)
	return em.Result()
}

// Exec compiles and runs.
func (b *DeleteBuilder) Exec(ctx context.Context, exe db.Executor) (db.Result, error) {
	if db.FlavorOf(exe).Name() == "ClickHouse" {
		return db.Result{}, db.ErrFlavorMismatch
	}
	return db.ExecBuilder(ctx, exe, b)
}

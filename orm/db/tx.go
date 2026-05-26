package db

import (
	"context"
	stdsql "database/sql"
	"fmt"

	"github.com/stn81/kate/orm/flavor"
)

// Tx is a transaction handle. Same Get/List/Iter/Exec entry points accept
// *Tx via the Executor interface, so business code rarely cares whether
// it's running inside a transaction.
type Tx struct {
	tx     *stdsql.Tx
	db     *DB
	flavor flavor.Flavor
}

// InTx runs fn inside a transaction. The transaction is committed if fn
// returns nil; otherwise it's rolled back and the error returned. Panic
// in fn triggers rollback then re-panic.
func InTx(ctx context.Context, db *DB, fn func(ctx context.Context, tx *Tx) error) error {
	return InTxOpts(ctx, db, nil, fn)
}

// InTxOpts is InTx with explicit options (isolation level, read-only).
func InTxOpts(ctx context.Context, db *DB, opts *stdsql.TxOptions, fn func(ctx context.Context, tx *Tx) error) error {
	if db.isClosed() {
		return ErrClosed
	}
	stdTx, err := db.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("kate/db.InTx: begin: %w", err)
	}
	tx := &Tx{tx: stdTx, db: db, flavor: db.flavor}
	done := false
	defer func() {
		if !done {
			_ = stdTx.Rollback()
		}
	}()
	if err := fn(ctx, tx); err != nil {
		done = true
		if rbErr := stdTx.Rollback(); rbErr != nil {
			return fmt.Errorf("kate/db.InTx: %w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}
	done = true
	if err := stdTx.Commit(); err != nil {
		return fmt.Errorf("kate/db.InTx: commit: %w", err)
	}
	return nil
}

// ----- Executor impl -----

func (t *Tx) queryContext(ctx context.Context, query string, args ...any) (*stdsql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}
func (t *Tx) execContext(ctx context.Context, query string, args ...any) (stdsql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}
func (t *Tx) flavorOf() flavor.Flavor          { return t.flavor }
func (t *Tx) loggerForCtx(ctx context.Context) Logger { return t.db.loggerFor(ctx) }
func (t *Tx) activeHooks() []Hook              { return t.db.hooks }

// Package db is the top-level entry point of kate/v2. It wires *sql.DB to
// kate's typed builders and exposes the user-facing Get/List/Iter/Exec API.
package db

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stn81/kate/orm/flavor"
	ksql "github.com/stn81/kate/orm/sql"
	"go.uber.org/zap"
)

// Logger is the minimum logging surface kate calls into. Production
// callers usually pass a *zap.Logger; tests can pass a no-op via NoopLogger.
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
}

// NoopLogger is a Logger that drops everything.
type NoopLogger struct{}

func (NoopLogger) Debug(string, ...zap.Field) {}
func (NoopLogger) Info(string, ...zap.Field)  {}
func (NoopLogger) Warn(string, ...zap.Field)  {}
func (NoopLogger) Error(string, ...zap.Field) {}

// Hook is invoked by db operations at query boundaries. Implementations
// typically record metrics, traces, or audit events.
type Hook interface {
	// OnQuery fires after a successful query (Exec or Query). Duration is
	// the round-trip time including driver work but excluding row Scan.
	OnQuery(ctx context.Context, op, query string, args []any, dur time.Duration)
	// OnError fires on any driver error. The error is the wrapped *Error
	// (Unwrap to reach the driver cause).
	OnError(ctx context.Context, op, query string, args []any, err error)
}

// Config configures a *DB. All fields except DSN and Flavor are optional.
type Config struct {
	// DSN is the data source name passed to the underlying database/sql
	// driver (e.g. clickhouse://user:pass@host:9000/db, or a clickhouse-go
	// v2 DSN). The driver name is derived from Flavor.
	DSN string

	// Flavor declares the dialect this connection speaks. Required.
	// Use flavor/mysql.Flavor, flavor/postgres.Flavor, or
	// flavor/clickhouse.Flavor.
	Flavor flavor.Flavor

	// DriverName overrides the registered database/sql driver name. When
	// empty, the default for each flavor is used: "mysql", "postgres", or
	// "clickhouse". Set this if you've registered a custom driver name.
	DriverName string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Logger is the default logger; per-call override via WithLogger(ctx).
	// nil → NoopLogger.
	Logger Logger

	// Hooks fire on every Exec/Query at the boundary. Order is preserved.
	Hooks []Hook
}

// DB is a goroutine-safe handle to a database, parameterized by Flavor.
// After Open, the DB itself is immutable; mutating the underlying *sql.DB
// settings (MaxOpenConns etc.) happens at Open and is fixed thereafter to
// keep the type honest (no "did someone change the pool while I was
// using it?" surprises).
type DB struct {
	pool   *stdsql.DB
	flavor flavor.Flavor
	logger Logger
	hooks  []Hook

	mu     sync.RWMutex
	closed bool
}

// Open builds a *DB from cfg. The underlying *sql.DB is opened and pinged
// before return; failure surfaces immediately.
func Open(cfg Config) (*DB, error) {
	if cfg.Flavor == nil {
		return nil, fmt.Errorf("kate/db.Open: Config.Flavor is required")
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("kate/db.Open: Config.DSN is required")
	}
	driver := cfg.DriverName
	if driver == "" {
		driver = defaultDriverName(cfg.Flavor)
	}
	pool, err := stdsql.Open(driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("kate/db.Open: sql.Open(%q): %w", driver, err)
	}
	if cfg.MaxOpenConns > 0 {
		pool.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		pool.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		pool.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		pool.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
	if err := pool.PingContext(context.Background()); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("kate/db.Open: ping: %w", err)
	}
	logger := cfg.Logger
	if logger == nil {
		logger = NoopLogger{}
	}
	return &DB{
		pool:   pool,
		flavor: cfg.Flavor,
		logger: logger,
		hooks:  append([]Hook(nil), cfg.Hooks...),
	}, nil
}

// Flavor returns the dialect this DB speaks.
func (db *DB) Flavor() flavor.Flavor { return db.flavor }

// Pool exposes the underlying *sql.DB. Use only for ops kate doesn't model
// (driver-level hooks, advanced settings); routine queries go through the
// typed Get/List/Iter/Exec entry points.
func (db *DB) Pool() *stdsql.DB { return db.pool }

// Ping checks connectivity.
func (db *DB) Ping(ctx context.Context) error {
	if db.isClosed() {
		return ErrClosed
	}
	return db.pool.PingContext(ctx)
}

// Close shuts down the underlying pool. Subsequent operations return
// ErrClosed.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return nil
	}
	db.closed = true
	return db.pool.Close()
}

func (db *DB) isClosed() bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.closed
}

// loggerFor returns the logger to use for this call: ctx override > DB default.
func (db *DB) loggerFor(ctx context.Context) Logger {
	if l, ok := loggerFromCtx(ctx); ok {
		return l
	}
	return db.logger
}

func defaultDriverName(f flavor.Flavor) string {
	switch f.Name() {
	case "MySQL":
		return "mysql"
	case "PostgreSQL":
		return "postgres"
	case "ClickHouse":
		return "clickhouse"
	}
	return ""
}

// ----- context helpers -----

type loggerCtxKey struct{}

// WithLogger attaches a per-call logger to the context. Overrides
// Config.Logger for any kate/db operation running with this ctx.
func WithLogger(ctx context.Context, l Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// LoggerFrom returns the per-call logger attached to ctx, if any.
func LoggerFrom(ctx context.Context) Logger {
	if l, ok := loggerFromCtx(ctx); ok {
		return l
	}
	return nil
}

func loggerFromCtx(ctx context.Context) (Logger, bool) {
	if ctx == nil {
		return nil, false
	}
	v := ctx.Value(loggerCtxKey{})
	if v == nil {
		return nil, false
	}
	l, ok := v.(Logger)
	return l, ok
}

// ----- Executor abstraction -----

// Executor is the read-side abstraction shared by *DB and *Tx. Top-level
// Get / List / Iter / Exec take Executor, so the same call works inside
// or outside a transaction.
type Executor interface {
	// queryContext runs a query and returns *sql.Rows.
	queryContext(ctx context.Context, query string, args ...any) (*stdsql.Rows, error)
	// execContext runs a non-query statement and returns the result.
	execContext(ctx context.Context, query string, args ...any) (stdsql.Result, error)
	// flavor returns the dialect of this executor.
	flavorOf() flavor.Flavor
	// loggerFor returns the active logger.
	loggerForCtx(ctx context.Context) Logger
	// hooks returns the active hook list (zero-len slice if none).
	activeHooks() []Hook
}

func (db *DB) queryContext(ctx context.Context, query string, args ...any) (*stdsql.Rows, error) {
	return db.pool.QueryContext(ctx, query, args...)
}
func (db *DB) execContext(ctx context.Context, query string, args ...any) (stdsql.Result, error) {
	return db.pool.ExecContext(ctx, query, args...)
}
func (db *DB) flavorOf() flavor.Flavor          { return db.flavor }
func (db *DB) loggerForCtx(ctx context.Context) Logger { return db.loggerFor(ctx) }
func (db *DB) activeHooks() []Hook              { return db.hooks }

// ----- shared build path -----

// buildQuery compiles a ksql.Builder against the executor's flavor.
// Adapts the flavor.Flavor → ksql.Flavor interface (they're structurally
// identical; the two-package definition exists to break the import cycle
// between sql and flavor).
func buildQuery(exe Executor, b ksql.Builder) (string, []any, error) {
	f := exe.flavorOf()
	// flavor.Flavor and ksql.Flavor have identical methods; pass through.
	adapter := flavorAdapter{f}
	return b.Build(adapter)
}

type flavorAdapter struct{ f flavor.Flavor }

func (a flavorAdapter) Name() string                  { return a.f.Name() }
func (a flavorAdapter) Quote(ident string) string     { return a.f.Quote(ident) }
func (a flavorAdapter) Placeholder(i int) string      { return a.f.Placeholder(i) }
func (a flavorAdapter) SupportsCTE() bool             { return a.f.SupportsCTE() }
func (a flavorAdapter) SupportsReturning() bool       { return a.f.SupportsReturning() }

// firePostQuery notifies hooks of a successful query.
func firePostQuery(exe Executor, ctx context.Context, op, q string, args []any, dur time.Duration) {
	for _, h := range exe.activeHooks() {
		h.OnQuery(ctx, op, q, args, dur)
	}
}

// firePostError notifies hooks of a failed query.
func firePostError(exe Executor, ctx context.Context, op, q string, args []any, err error) {
	if err == nil {
		return
	}
	for _, h := range exe.activeHooks() {
		h.OnError(ctx, op, q, args, err)
	}
}

// isNoRowsError reports whether err is "no rows" from stdsql or kate.
func isNoRowsError(err error) bool {
	return errors.Is(err, stdsql.ErrNoRows) || errors.Is(err, ErrNoRows)
}

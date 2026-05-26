package olap

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	stdsql "database/sql"

	"github.com/stn81/kate/orm/db"
	ksql "github.com/stn81/kate/orm/sql"
)

// BatchInserter[T] writes rows of type T into a ClickHouse table via the
// driver's prepared-batch API. Unlike oltp.InsertBuilder this is the high-
// throughput path: rows are buffered locally and Send flushes them as a
// single columnar block.
//
// Typical use:
//
//	bi := olap.NewBatchInserter[Row](db, table.T, []sql.AnyCol{
//	    table.T.Date, table.T.Userid, table.T.AdRevenue,
//	})
//	for _, r := range rows {
//	    if err := bi.Append(r); err != nil { /* ... */ }
//	}
//	n, err := bi.Send(ctx)
type BatchInserter[T any] struct {
	db    *db.DB
	table ksql.TableRef
	cols  []ksql.AnyCol

	mu     sync.Mutex
	buffer []T

	// columnPaths is the field-path mapping (col index → struct field path)
	// resolved on the first Append from T's reflect.Type. Cached.
	columnPaths [][]int
	resolved    bool
}

// NewBatchInserter constructs a BatchInserter for a CK table. cols
// determines column order in the prepared batch — Append walks the row's
// fields against cols' column names (via the same tag lookup as Scan).
func NewBatchInserter[T any](dbh *db.DB, t ksql.TableRef, cols []ksql.AnyCol) *BatchInserter[T] {
	return &BatchInserter[T]{db: dbh, table: t, cols: cols}
}

// Append buffers one or more rows.
func (bi *BatchInserter[T]) Append(rows ...T) error {
	if err := bi.resolve(); err != nil {
		return err
	}
	bi.mu.Lock()
	bi.buffer = append(bi.buffer, rows...)
	bi.mu.Unlock()
	return nil
}

// Len reports the number of buffered rows.
func (bi *BatchInserter[T]) Len() int {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	return len(bi.buffer)
}

// Reset drops the buffer without sending.
func (bi *BatchInserter[T]) Reset() {
	bi.mu.Lock()
	bi.buffer = bi.buffer[:0]
	bi.mu.Unlock()
}

// Send commits the buffered rows in a single PrepareBatch round-trip,
// returning the number of rows sent. After Send the buffer is empty.
//
// Implementation note: clickhouse-go v2's PrepareBatch is exposed through
// the driver.Conn interface, which we reach via db.Pool().Conn(). We use
// a plain INSERT INTO ... VALUES statement; the driver intercepts it and
// switches to native columnar mode automatically.
func (bi *BatchInserter[T]) Send(ctx context.Context) (int64, error) {
	if bi.db.Flavor().Name() != "ClickHouse" {
		return 0, db.ErrFlavorMismatch
	}
	bi.mu.Lock()
	rows := bi.buffer
	bi.buffer = nil
	bi.mu.Unlock()
	if len(rows) == 0 {
		return 0, nil
	}
	stmt, err := bi.buildInsertStmt()
	if err != nil {
		return 0, err
	}

	// Begin a transaction to acquire a dedicated connection (CK driver
	// requires PrepareBatch on the same conn). For CK the tx is just a
	// transport hint; commit returns success without real ACID.
	tx, err := bi.db.Pool().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	pstmt, err := tx.PrepareContext(ctx, stmt)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	defer pstmt.Close()

	for _, r := range rows {
		args := bi.extractArgs(r)
		if _, err := pstmt.ExecContext(ctx, args...); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

// buildInsertStmt produces the INSERT INTO ... (cols...) VALUES (?,?,...)
// template that the CK driver's PrepareBatch path consumes.
func (bi *BatchInserter[T]) buildInsertStmt() (string, error) {
	f := bi.db.Flavor()
	em := ksql.NewEmitter(&flavorPassthrough{f: f})
	em.WriteString("INSERT INTO ")
	if bi.table.Schema() != "" {
		em.WriteString(f.Quote(bi.table.Schema()))
		_ = em.WriteByte('.')
	}
	em.WriteString(f.Quote(bi.table.Name()))
	em.WriteString(" (")
	for i, c := range bi.cols {
		if i > 0 {
			em.WriteString(", ")
		}
		em.WriteString(f.Quote(colName(c)))
	}
	em.WriteString(") VALUES (")
	for i := range bi.cols {
		if i > 0 {
			em.WriteString(", ")
		}
		em.WriteString(f.Placeholder(i + 1))
	}
	_ = em.WriteByte(')')
	s, _, err := em.Result()
	return s, err
}

// extractArgs walks a row T and produces the arg slice in column order.
func (bi *BatchInserter[T]) extractArgs(row T) []any {
	rv := reflect.ValueOf(row)
	out := make([]any, len(bi.columnPaths))
	for i, path := range bi.columnPaths {
		fv := rv
		for _, idx := range path {
			if fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					out[i] = nil
					goto next
				}
				fv = fv.Elem()
			}
			fv = fv.Field(idx)
		}
		out[i] = fv.Interface()
	next:
	}
	return out
}

// resolve maps each column to a struct field path in T. Cached on the
// inserter — one resolution per BatchInserter lifetime.
func (bi *BatchInserter[T]) resolve() error {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	if bi.resolved {
		return nil
	}
	var zero T
	rt := reflect.TypeOf(zero)
	if rt == nil || rt.Kind() != reflect.Struct {
		return fmt.Errorf("kate/olap: BatchInserter[T] requires struct T")
	}
	fieldByCol := map[string][]int{}
	collectBatchFields(rt, nil, fieldByCol)
	paths := make([][]int, len(bi.cols))
	for i, c := range bi.cols {
		name := colName(c)
		path, ok := fieldByCol[name]
		if !ok {
			return fmt.Errorf("kate/olap: column %q has no matching field in %s", name, rt)
		}
		paths[i] = path
	}
	bi.columnPaths = paths
	bi.resolved = true
	return nil
}

func collectBatchFields(t reflect.Type, prefix []int, out map[string][]int) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		path := append(append([]int(nil), prefix...), i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			collectBatchFields(f.Type, path, out)
			continue
		}
		if !f.IsExported() {
			continue
		}
		name := tagColumnName(f)
		if name == "" || name == "-" {
			continue
		}
		out[name] = path
	}
}

func tagColumnName(f reflect.StructField) string {
	if tag, ok := f.Tag.Lookup("db"); ok {
		if i := indexByte(tag, ','); i >= 0 {
			tag = tag[:i]
		}
		return tag
	}
	if tag, ok := f.Tag.Lookup("orm"); ok {
		for _, part := range splitSemis(tag) {
			if len(part) >= len("column()") && part[:len("column(")] == "column(" && part[len(part)-1] == ')' {
				return part[len("column(") : len(part)-1]
			}
		}
	}
	// no tag → snake_case fallback
	return snakeName(f.Name)
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func splitSemis(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	// trim space
	for i, p := range out {
		out[i] = trimSpaces(p)
	}
	return out
}

func trimSpaces(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func snakeName(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i > 0 && c >= 'A' && c <= 'Z' {
			b = append(b, '_')
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b = append(b, c)
	}
	return string(b)
}

// colName emits an AnyCol against the strip flavor to recover its bare
// column name.
func colName(c ksql.AnyCol) string {
	em := ksql.NewEmitter(stripFlavor{})
	c.Emit(em)
	s, _, _ := em.Result()
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[i+1:]
		}
	}
	return s
}

// flavorPassthrough adapts the db.Flavor interface to ksql.Flavor.
type flavorPassthrough struct{ f interface{ Name() string; Quote(string) string; Placeholder(int) string; SupportsCTE() bool; SupportsReturning() bool } }

func (p *flavorPassthrough) Name() string             { return p.f.Name() }
func (p *flavorPassthrough) Quote(s string) string    { return p.f.Quote(s) }
func (p *flavorPassthrough) Placeholder(i int) string { return p.f.Placeholder(i) }
func (p *flavorPassthrough) SupportsCTE() bool        { return p.f.SupportsCTE() }
func (p *flavorPassthrough) SupportsReturning() bool  { return p.f.SupportsReturning() }

// WaitMutation polls system.mutations for completion of a mutation id.
// Currently a thin placeholder: CK doesn't tie our synthetic MutationID
// to a backend identifier. Implementations that want real polling should
// query `SELECT * FROM system.mutations WHERE table = ? AND is_done = 0`
// directly via db.ScanAll.
func WaitMutation(ctx context.Context, dbh *db.DB, table string) error {
	if dbh.Flavor().Name() != "ClickHouse" {
		return db.ErrFlavorMismatch
	}
	// Real implementation: poll system.mutations. Left as a hook so this
	// package compiles cleanly without external state assumptions.
	_ = ctx
	_ = table
	_ = stdsql.LevelDefault
	return nil
}

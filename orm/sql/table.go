package sql

// TableRef is a typed reference to a table, view, or aliased subquery used
// as a FROM / JOIN source. Codegen-emitted `T` values embed TableRef; CTE
// .Ref() returns a TableRef for the CTE's logical alias.
type TableRef struct {
	schema string // optional, e.g. "hmct"
	name   string // table or view name
	alias  string // optional alias for use in SELECT/JOIN
	// virtual=true means "name" is the alias of a CTE / subquery, schema
	// must be empty. emit() then writes only the alias.
	virtual bool
}

// NewTable constructs a TableRef for a real schema.table with optional alias.
func NewTable(schema, name, alias string) TableRef {
	return TableRef{schema: schema, name: name, alias: alias}
}

// NewVirtualTable constructs a TableRef that emits as a bare identifier (the
// alias / CTE name).
func NewVirtualTable(alias string) TableRef {
	return TableRef{name: alias, alias: alias, virtual: true}
}

// As returns a copy of the TableRef with the given alias attached.
func (t TableRef) As(alias string) TableRef {
	t.alias = alias
	return t
}

// Schema reports the schema component (may be "").
func (t TableRef) Schema() string { return t.schema }

// Name reports the table name.
func (t TableRef) Name() string { return t.name }

// Alias reports the attached alias (may be "").
func (t TableRef) Alias() string { return t.alias }

// emitSource emits the FROM/JOIN source: `"schema"."name" AS "alias"` (or
// for virtual tables, just the bare alias). For column references the
// alias-or-table prefix is chosen by QualifiedColumn at the column level.
func (t TableRef) emitSource(e *Emitter) {
	if t.virtual {
		e.Quote(t.alias)
		return
	}
	e.QualifiedTable(t.schema, t.name)
	if t.alias != "" {
		e.WriteString(" AS ")
		e.Quote(t.alias)
	}
}

// Col constructs a typed column reference against this table. Codegen
// usually pre-builds Col[T] values inside the table descriptor; this
// dynamic form is used when accessing columns of CTEs or aliased subqueries
// where the schema cannot be known at codegen time.
func (t TableRef) Col(name string) AnyCol { return dynamicCol{table: t, name: name} }

// ColAs[T] is the typed dynamic-column accessor. Use when you know the
// column's Go type but not at codegen time (e.g. CTE columns).
func ColAs[T any](t TableRef, name string) Col[T] {
	return Col[T]{schema: t.schema, table: t.name, alias: t.alias, name: name}
}

// CTERefColumn returns a typed column reference into a CTE Ref.
//
// Equivalent to ColAs[T] but reads more clearly at call sites that pull
// columns out of a CTE.
func CTERefColumn[T any](t TableRef, name string) Col[T] { return ColAs[T](t, name) }

// dynamicCol is the AnyCol returned by TableRef.Col(name) when the Go type
// is not specified. Useful for GROUP BY / ORDER BY where the column's value
// type is irrelevant.
type dynamicCol struct {
	table TableRef
	name  string
}

func (dynamicCol) expr()      {}
func (dynamicCol) column()    {}
func (dynamicCol) Type() Type { return TypeAny }
func (c dynamicCol) Emit(e *Emitter) {
	e.QualifiedColumn(c.table.schema, c.table.name, c.table.alias, c.name)
}

// View is a marker wrapper around TableRef indicating "read-only". oltp
// builders refuse to write to a View at construction time.
type View struct{ TableRef }

// NewView constructs a View reference.
func NewView(schema, name, alias string) View {
	return View{TableRef: NewTable(schema, name, alias)}
}

// NoTable is a sentinel used for SELECT-without-FROM contexts (e.g. CK
// `SELECT arrayJoin(bitmapToArray(...))` with no source).
var NoTable = TableRef{virtual: true}

// Package flavor defines the dialect protocol consumed by kate/v2/sql. Each
// concrete dialect (MySQL / PostgreSQL / ClickHouse) implements Flavor in
// its own subpackage and exports a package-level value.
//
// The interface is intentionally narrow: only the primitives that the
// sql.Emitter actually calls during build. Dialect-specific clause emit
// (CK FINAL / PREWHERE / SETTINGS / LIMIT BY) is reached by type-asserting
// to ClickHouseFlavor, not by adding methods every flavor would have to
// stub.
package flavor

// Flavor is the minimum dialect protocol consumed by sql.Emitter.
type Flavor interface {
	Name() string
	Quote(ident string) string
	Placeholder(i int) string // i is 1-indexed
	SupportsCTE() bool
	SupportsReturning() bool
}

// ClickHouseFlavor is the extended protocol implemented by the ClickHouse
// flavor. olap/ch builders type-assert to this and panic if the underlying
// flavor doesn't satisfy — but in normal use, ch builders are only
// constructible against a *DB whose flavor is ClickHouse, so the assertion
// is a defense-in-depth check.
type ClickHouseFlavor interface {
	Flavor
	IsClickHouse() // marker — exists only on the CH flavor impl
}

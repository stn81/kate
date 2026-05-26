package sql

import "fmt"

// ErrSubqueryShape is returned by Subquery / InSubquery builders when the
// inner SELECT does not project exactly one column (the single-column
// constraint cannot be expressed in the Go type system because column
// counts are dynamic).
type ErrSubqueryShape struct {
	Got int
}

func (e ErrSubqueryShape) Error() string {
	return fmt.Sprintf("kate/sql: subquery must project exactly 1 column, got %d", e.Got)
}

// errCTEUnsupported is returned when WITH is attached but the target flavor
// does not support CTEs (e.g. MySQL < 8).
func errCTEUnsupported(flavor string) error {
	return fmt.Errorf("kate/sql: flavor %q does not support CTE (WITH)", flavor)
}

// errCKOnly is returned when a CK-only clause is build-emitted against a
// non-CK flavor. CK-only methods live on *ch.SelectBuilder so this is
// nominally unreachable from typed code; the guard exists for defense
// against manual flavor swaps.
func errCKOnly(clause, flavor string) error {
	return fmt.Errorf("kate/sql: clause %q is ClickHouse-only, current flavor %q", clause, flavor)
}

// ErrMutationPartitionRequired is returned by olap.MutateBuilder.Build when
// RequirePartition was not called — surfaced here so the sql package can
// reference the sentinel without introducing a cycle.
var ErrMutationPartitionRequired = fmt.Errorf("kate/olap: mutation requires partition predicate (call RequirePartition)")

// ExposeErrCKOnly is the package-level adapter for olap/ch to invoke when
// emitting a CK clause and finding the flavor is not actually CK.
func ExposeErrCKOnly(clause, flavor string) error { return errCKOnly(clause, flavor) }

package sql_test

import (
	"testing"

	"github.com/stn81/kate/orm/flavor/clickhouse"
	"github.com/stn81/kate/orm/flavor/mysql"
	"github.com/stn81/kate/orm/flavor/postgres"
	ksql "github.com/stn81/kate/orm/sql"
)

// usersTable is a hand-built table descriptor that mimics what kate-gen
// would emit for a real table.
var usersTable = struct {
	Ref      ksql.TableRef
	Userid   ksql.Col[uint64]
	Channel  ksql.Col[string]
	RegTime  ksql.Col[int64]
	Verified ksql.Col[bool]
}{
	Ref:      ksql.NewTable("app", "users", ""),
	Userid:   ksql.NewCol[uint64]("app", "users", "userid"),
	Channel:  ksql.NewCol[string]("app", "users", "channel"),
	RegTime:  ksql.NewCol[int64]("app", "users", "reg_time"),
	Verified: ksql.NewCol[bool]("app", "users", "verified"),
}

type adapter struct {
	name        string
	quoteOpen   byte
	quoteClose  byte
	placeholder func(int) string
}

func toAdapter(f interface {
	Name() string
	Quote(string) string
	Placeholder(int) string
	SupportsCTE() bool
	SupportsReturning() bool
}) ksql.Flavor {
	return wrap{f: f}
}

type wrap struct {
	f interface {
		Name() string
		Quote(string) string
		Placeholder(int) string
		SupportsCTE() bool
		SupportsReturning() bool
	}
}

func (w wrap) Name() string             { return w.f.Name() }
func (w wrap) Quote(s string) string    { return w.f.Quote(s) }
func (w wrap) Placeholder(i int) string { return w.f.Placeholder(i) }
func (w wrap) SupportsCTE() bool        { return w.f.SupportsCTE() }
func (w wrap) SupportsReturning() bool  { return w.f.SupportsReturning() }

// TestSelect_BasicMySQL exercises a typical filter+order+limit SELECT against
// the MySQL flavor and asserts the literal SQL + arg list.
func TestSelect_BasicMySQL(t *testing.T) {
	q := ksql.From(usersTable.Ref).
		Select(ksql.Erase(usersTable.Userid), ksql.Erase(usersTable.Channel)).
		Where(usersTable.Channel.Eq("web")).
		OrderBy(ksql.Desc(usersTable.RegTime)).
		Limit(10)

	got, args, err := q.Build(toAdapter(mysql.Flavor))
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	want := "SELECT `app`.`users`.`userid`, `app`.`users`.`channel` " +
		"FROM `app`.`users` " +
		"WHERE `app`.`users`.`channel` = ? " +
		"ORDER BY `app`.`users`.`reg_time` DESC LIMIT 10"
	if got != want {
		t.Errorf("MySQL SQL mismatch\ngot:  %s\nwant: %s", got, want)
	}
	if len(args) != 1 || args[0] != "web" {
		t.Errorf("args mismatch: %v", args)
	}
}

// TestSelect_PostgresPlaceholders confirms positional $N placeholders are
// emitted in order for PG.
func TestSelect_PostgresPlaceholders(t *testing.T) {
	q := ksql.From(usersTable.Ref).
		Select(ksql.Erase(usersTable.Userid)).
		Where(
			usersTable.Channel.Eq("web"),
			usersTable.Userid.Gt(100),
		)

	got, args, err := q.Build(toAdapter(postgres.Flavor))
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	// PG should use $1, $2 and double-quoted identifiers.
	want := `SELECT "app"."users"."userid" FROM "app"."users" ` +
		`WHERE ("app"."users"."channel" = $1) AND ("app"."users"."userid" > $2)`
	if got != want {
		t.Errorf("PG SQL mismatch\ngot:  %s\nwant: %s", got, want)
	}
	if len(args) != 2 || args[0] != "web" || args[1] != uint64(100) {
		t.Errorf("args mismatch: %v", args)
	}
}

// TestSelect_CTE confirms a single CTE attached via With(...) is emitted
// as `WITH <name> AS (<inner>) SELECT ...`.
func TestSelect_CTE(t *testing.T) {
	inner := ksql.From(usersTable.Ref).
		Select(ksql.Erase(usersTable.Userid)).
		Where(usersTable.Verified.Eq(true))
	cte := ksql.NewCTE[any]("verified_uids", inner)

	q := ksql.From(usersTable.Ref).
		With(cte).
		Select(ksql.Erase(usersTable.Userid)).
		Where(ksql.InSubquery(usersTable.Userid,
			ksql.From(cte.Ref()).Select(ksql.Erase(ksql.ColAs[uint64](cte.Ref(), "userid")))))

	got, _, err := q.Build(toAdapter(clickhouse.Flavor))
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	wantPrefix := "WITH `verified_uids` AS ("
	if got[:len(wantPrefix)] != wantPrefix {
		t.Errorf("CTE prefix mismatch\ngot:  %s", got)
	}
}

// TestColTypeSafety is a compile-time check disguised as a runtime test:
// the body uses typed helpers in a way that would fail to compile if
// Col[uint64].Eq accepted strings. The fact that this file compiles is
// the test.
func TestColTypeSafety(t *testing.T) {
	// userid.Eq(123) — uint64 to uint64, OK.
	_ = usersTable.Userid.Eq(123)
	// channel.Like("%foo%") — Col[string], allowed by the Stringish constraint.
	_ = ksql.Like(usersTable.Channel, "%foo%")
	// The following lines would fail to compile if uncommented:
	//   _ = usersTable.Userid.Eq("abc")              // string ≠ uint64
	//   _ = ksql.Like(usersTable.Userid, "%foo%")    // uint64 not Stringish
	//   _ = ksql.Like(usersTable.Verified, "%foo%")  // bool not Stringish
	t.Log("compile-time type safety preserved")
}

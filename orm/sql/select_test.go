package sql_test

import (
	"testing"

	"github.com/stn81/kate/orm/flavor/clickhouse"
	"github.com/stn81/kate/orm/flavor/mysql"
	"github.com/stn81/kate/orm/flavor/postgres"
	ksql "github.com/stn81/kate/orm/sql"
)

// usersTable is a hand-built table descriptor mimicking what kate-gen
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

// TestSelect_BasicMySQL exercises a typical filter+order+limit SELECT.
func TestSelect_BasicMySQL(t *testing.T) {
	q := ksql.From(usersTable.Ref).
		Select(usersTable.Userid, usersTable.Channel).
		Where(usersTable.Channel.Eq("web")).
		OrderBy(ksql.Desc(usersTable.RegTime)).
		Limit(10)

	got, args, err := q.Build(mysql.Flavor)
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

// TestSelect_PostgresPlaceholders confirms $N placeholders for PG.
func TestSelect_PostgresPlaceholders(t *testing.T) {
	q := ksql.From(usersTable.Ref).
		Select(usersTable.Userid).
		Where(
			usersTable.Channel.Eq("web"),
			usersTable.Userid.Gt(100),
		)

	got, args, err := q.Build(postgres.Flavor)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	want := `SELECT "app"."users"."userid" FROM "app"."users" ` +
		`WHERE ("app"."users"."channel" = $1) AND ("app"."users"."userid" > $2)`
	if got != want {
		t.Errorf("PG SQL mismatch\ngot:  %s\nwant: %s", got, want)
	}
	if len(args) != 2 || args[0] != "web" || args[1] != uint64(100) {
		t.Errorf("args mismatch: %v", args)
	}
}

// TestSelect_CTE confirms a CTE emits as WITH name AS (...) SELECT ...
func TestSelect_CTE(t *testing.T) {
	inner := ksql.From(usersTable.Ref).
		Select(usersTable.Userid).
		Where(usersTable.Verified.Eq(true))
	cte := ksql.NewCTE("verified_uids", inner)

	q := ksql.From(usersTable.Ref).
		With(cte).
		Select(usersTable.Userid).
		Where(ksql.InSubquery(usersTable.Userid,
			ksql.From(cte.Ref()).Select(ksql.ColAs[uint64](cte.Ref(), "userid"))))

	got, _, err := q.Build(clickhouse.Flavor)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	wantPrefix := "WITH `verified_uids` AS ("
	if got[:len(wantPrefix)] != wantPrefix {
		t.Errorf("CTE prefix mismatch\ngot:  %s", got)
	}
}

// TestSelect_As exercises the As() chain on different Expr[T] producers
// (Col, RawExpr, AliasedExpr re-alias).
func TestSelect_As(t *testing.T) {
	q := ksql.From(usersTable.Ref).Select(
		usersTable.Userid.As("uid"),
		ksql.RawExpr[uint64]("count(*)").As("n"),
	)
	got, _, err := q.Build(mysql.Flavor)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	want := "SELECT `app`.`users`.`userid` AS `uid`, count(*) AS `n` FROM `app`.`users`"
	if got != want {
		t.Errorf("As mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

// TestSelect_EqCol confirms col.EqCol(other) emits col=col.
func TestSelect_EqCol(t *testing.T) {
	a := usersTable.Userid
	b := ksql.NewCol[uint64]("app", "audit", "userid").Of(ksql.NewTable("app", "audit", "x"))
	pred := a.EqCol(b)
	q := ksql.From(usersTable.Ref).Select(usersTable.Userid).Where(pred)
	got, _, err := q.Build(mysql.Flavor)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	wantSub := "`app`.`users`.`userid` = `x`.`userid`"
	if !contains(got, wantSub) {
		t.Errorf("EqCol mismatch\ngot:  %s\nwant substring: %s", got, wantSub)
	}
}

// TestColTypeSafety is a compile-time check disguised as a runtime test.
// Lines commented out below would fail to compile.
func TestColTypeSafety(t *testing.T) {
	_ = usersTable.Userid.Eq(123)
	_ = ksql.Like(usersTable.Channel, "%foo%")
	// _ = usersTable.Userid.Eq("abc")              // string ≠ uint64
	// _ = ksql.Like(usersTable.Userid, "%foo%")    // uint64 not Stringish
	// _ = ksql.Like(usersTable.Verified, "%foo%")  // bool not Stringish
	t.Log("compile-time type safety preserved")
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

package ch_test

import (
	"strings"
	"testing"

	"github.com/stn81/kate/orm/flavor/clickhouse"
	"github.com/stn81/kate/orm/olap/ch"
	"github.com/stn81/kate/orm/olap/chexpr"
	ksql "github.com/stn81/kate/orm/sql"
)

// chFlavor is the clickhouse package's flavor value, used directly as
// ksql.Flavor (now a type alias of flavor.Flavor).
var chFlavor = clickhouse.Flavor

var statTable = struct {
	Ref     ksql.TableRef
	Date    ksql.Col[int64]
	Userid  ksql.Col[uint64]
	Revenue ksql.Col[float64]
}{
	Ref:     ksql.NewTable("hmct", "stat", ""),
	Date:    ksql.NewCol[int64]("hmct", "stat", "date"),
	Userid:  ksql.NewCol[uint64]("hmct", "stat", "userid"),
	Revenue: ksql.NewCol[float64]("hmct", "stat", "revenue"),
}

// TestCh_Final verifies that FINAL is emitted after the FROM table.
func TestCh_Final(t *testing.T) {
	q := ch.From(statTable.Ref).
		Final().
		Select(statTable.Userid).
		Where(statTable.Date.Eq(20260101))

	got, _, err := q.Build(chFlavor)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.Contains(got, "`hmct`.`stat` FINAL") {
		t.Errorf("FINAL not emitted after table\nSQL: %s", got)
	}
}

// TestCh_Prewhere verifies PREWHERE precedes WHERE.
func TestCh_Prewhere(t *testing.T) {
	q := ch.From(statTable.Ref).
		Select(statTable.Userid).
		Prewhere(statTable.Date.Eq(20260101)).
		Where(statTable.Revenue.Gt(0.0))

	got, _, err := q.Build(chFlavor)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	preIdx := strings.Index(got, "PREWHERE")
	whrIdx := strings.Index(got, "WHERE")
	if preIdx < 0 || whrIdx < 0 || preIdx > whrIdx {
		t.Errorf("PREWHERE/WHERE order wrong: pre=%d where=%d sql=%s", preIdx, whrIdx, got)
	}
}

// TestCh_Settings confirms SETTINGS appears at the end with deterministic
// key order.
func TestCh_Settings(t *testing.T) {
	q := ch.From(statTable.Ref).
		Select(statTable.Userid).
		Settings(map[string]any{"max_threads": 4, "join_algorithm": "hash"})

	got, args, err := q.Build(chFlavor)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.Contains(got, " SETTINGS ") {
		t.Errorf("SETTINGS not emitted\nSQL: %s", got)
	}
	// keys should be sorted: join_algorithm, max_threads
	if !strings.Contains(got, "join_algorithm = ?") {
		t.Errorf("settings emit shape\nSQL: %s", got)
	}
	if len(args) != 2 || args[0] != "hash" || args[1] != 4 {
		t.Errorf("args mismatch: %v", args)
	}
}

// TestCh_BitmapPipeline simulates the report-api cohort pattern:
//   userid IN (SELECT arrayJoin(bitmapToArray(bitmapAnd(a, b))))
// against the typed CK expression builders.
func TestCh_BitmapPipeline(t *testing.T) {
	bmA := ksql.NewCol[chexpr.BitMap64]("hmct", "cohort", "bm_a")
	bmB := ksql.NewCol[chexpr.BitMap64]("hmct", "cohort", "bm_b")
	merged := chexpr.BitmapAnd(bmA, bmB)
	// SELECT arrayJoin(bitmapToArray(bitmapAnd(bm_a, bm_b)))
	uidsSub := ch.SelectLiteral().Select(
		chexpr.ArrayJoin[uint64](chexpr.BitmapToArray(merged)),
	)

	q := ch.From(statTable.Ref).
		Select(statTable.Userid).
		Where(ksql.InSubquery(statTable.Userid, uidsSub.Inner()))

	got, _, err := q.Build(chFlavor)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	wantSubstrs := []string{
		"arrayJoin",
		"bitmapToArray",
		"bitmapAnd",
		" IN (SELECT arrayJoin(bitmapToArray(bitmapAnd(",
	}
	for _, s := range wantSubstrs {
		if !strings.Contains(got, s) {
			t.Errorf("bitmap pipeline missing %q\nSQL: %s", s, got)
		}
	}
}

package date

import (
	"encoding/json"
	"testing"
	"time"
)

// TestUnmarshalJSON_TimezoneAlignedWithNew 验证 UnmarshalJSON 和 New 在同一日期下出来的
// Date 完全相等（time.Time.Equal + .Location() 一致），避免 UTC vs Local 差时区偏移量的 bug。
//
// 这是历史 bug 的回归测试：早期版本 UnmarshalJSON 走 time.Parse 默认 UTC，而 New 显式用
// time.Local，在非 UTC 环境（如 Asia/Shanghai = +0800）下 "2026-05-25" 两边相差 8 小时，
// After/Before/Equal/Sub 全部错位。
func TestUnmarshalJSON_TimezoneAlignedWithNew(t *testing.T) {
	var fromJSON Date
	if err := fromJSON.UnmarshalJSON([]byte(`"2026-05-25"`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	fromNew := New(2026, 5, 25)

	if !fromJSON.Time.Equal(fromNew.Time) {
		t.Errorf("Time.Equal mismatch: from JSON = %v, from New = %v", fromJSON.Time, fromNew.Time)
	}
	if fromJSON.Time.Location().String() != fromNew.Time.Location().String() {
		t.Errorf("Location mismatch: from JSON = %v, from New = %v",
			fromJSON.Time.Location(), fromNew.Time.Location())
	}
	if fromJSON.After(fromNew) || fromJSON.Before(fromNew) {
		t.Errorf("same date should not be After/Before each other: JSON=%v New=%v", fromJSON.Time, fromNew.Time)
	}
}

// TestUnmarshalBind_TimezoneAlignedWithNew 验证 UnmarshalBind（用于 form / query 解析）
// 与 New 时区基准一致。
func TestUnmarshalBind_TimezoneAlignedWithNew(t *testing.T) {
	var fromBind Date
	if err := fromBind.UnmarshalBind("2026-05-25"); err != nil {
		t.Fatalf("UnmarshalBind: %v", err)
	}
	fromNew := New(2026, 5, 25)
	if !fromBind.Time.Equal(fromNew.Time) {
		t.Errorf("Time.Equal mismatch: from Bind = %v, from New = %v", fromBind.Time, fromNew.Time)
	}
}

// TestRoundTrip 测试 MarshalJSON → UnmarshalJSON 应得到同一 Date。
func TestRoundTrip(t *testing.T) {
	orig := New(2026, 5, 25)
	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Date
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !orig.Time.Equal(back.Time) {
		t.Errorf("round trip not equal: orig=%v back=%v", orig.Time, back.Time)
	}
}

// TestAfterEqualOnSameDate 验证修复前的 bug 场景：JSON 解出来的 Date 与 Today() 派生的
// yesterday 比较时不应该报告"after"。
func TestAfterEqualOnSameDate(t *testing.T) {
	// 模拟 handler 拿 yesterday 的代码路径：time.Now().In(loc).AddDate(0,0,-1) → FromTime
	now := time.Now()
	yesterdayT := now.AddDate(0, 0, -1)
	yesterday := FromTime(yesterdayT)

	// 从 JSON 解出来同一天日期的 Date
	var fromJSON Date
	if err := fromJSON.UnmarshalJSON([]byte(`"` + yesterday.String() + `"`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	if fromJSON.After(yesterday) {
		t.Errorf("same-day Date from JSON should NOT be After yesterday: JSON=%v yesterday=%v",
			fromJSON.Time, yesterday.Time)
	}
	if fromJSON.Before(yesterday) {
		t.Errorf("same-day Date from JSON should NOT be Before yesterday: JSON=%v yesterday=%v",
			fromJSON.Time, yesterday.Time)
	}
	if !fromJSON.Equal(yesterday) {
		t.Errorf("same-day Date from JSON should Equal yesterday: JSON=%v yesterday=%v",
			fromJSON.Time, yesterday.Time)
	}
}

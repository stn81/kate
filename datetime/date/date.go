package date

import (
	"database/sql/driver"
	"errors"
	"time"
)

type Date struct {
	time.Time
}

func Today() Date {
	return FromTime(time.Now())
}

func FromTime(t time.Time) Date {
	year, month, day := t.Date()
	return New(year, int(month), day)
}

func New(year, month, day int) Date {
	return Date{
		Time: time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local),
	}
}

func (dt Date) NextDay(days int) Date {
	return Date{
		Time: dt.Time.AddDate(0, 0, days),
	}
}

func (dt Date) PrevDay(days int) Date {
	return dt.NextDay(-days)
}

func (dt Date) NextMonth(months int) Date {
	return Date{
		Time: dt.Time.AddDate(0, months, 0),
	}
}

func (dt Date) PrevMonth(months int) Date {
	return dt.NextMonth(-months)
}

func (dt Date) NextYear(years int) Date {
	return Date{
		Time: dt.Time.AddDate(years, 0, 0),
	}
}

func (dt Date) PrevYear(years int) Date {
	return dt.NextYear(-years)
}

func (dt *Date) UnmarshalBind(value string) error {
	var err error
	// 用 ParseInLocation + time.Local，与 New() 构造的 Date 保持同一时区基准。
	// time.Parse 在 layout 不含时区指示时默认返回 UTC（见 https://pkg.go.dev/time#Parse），
	// 那样 "2026-05-25" 解析出来是 UTC 0:00，与 New(2026,5,25) 出来的 Local 0:00
	// 在非 UTC 时区下差一个时区偏移量，After/Before/Equal/Sub 会全部错位。
	if dt.Time, err = time.ParseInLocation(time.DateOnly, value, time.Local); err != nil {
		return err
	}
	return nil
}

func (dt Date) String() string {
	return dt.Time.Format(time.DateOnly)
}

func (dt Date) Value() (driver.Value, error) {
	return dt.String(), nil
}

func (dt *Date) Scan(src any) error {
	t, ok := src.(time.Time)
	if ok {
		dt.Time = FromTime(t).Time
		return nil
	}
	return errors.New("invalid value, must be time.Time")
}

func (dt Date) MarshalJSON() ([]byte, error) {
	bytes := []byte(`"` + dt.String() + `"`)
	return bytes, nil
}

func (dt *Date) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("Date.UnmarshalJSON: input is not a JSON string")
	}
	data = data[len(`"`) : len(data)-len(`"`)]
	// 同 UnmarshalBind：用 ParseInLocation + time.Local 与 New() 对齐时区。
	t, err := time.ParseInLocation(time.DateOnly, string(data), time.Local)
	if err != nil {
		return err
	}
	*dt = Date{
		Time: t,
	}
	return nil
}

func (dt Date) Before(other Date) bool {
	return dt.Time.Before(other.Time)
}

func (dt Date) After(other Date) bool {
	return dt.Time.After(other.Time)
}

func (dt Date) Equal(other Date) bool {
	return dt.Time.Equal(other.Time)
}

func (dt Date) Compare(other Date) int {
	return dt.Time.Compare(other.Time)
}

func (dt Date) Sub(other Date) time.Duration {
	return dt.Time.Sub(other.Time)
}

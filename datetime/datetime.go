package datetime

import (
	"database/sql/driver"
	"time"
)

type DateTime struct {
	time.Time
}

func Now() DateTime {
	return FromTime(time.Now())
}

func FromTime(t time.Time) DateTime {
	return DateTime{
		Time: t.Truncate(time.Second),
	}
}

func New(year, month, day, hour, minute, second int) DateTime {
	return DateTime{
		Time: time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local),
	}
}

func (dt *DateTime) UnmarshalBind(value string) error {
	var err error
	if dt.Time, err = time.Parse(time.DateTime, value); err != nil {
		return err
	}
	return nil
}

func (dt DateTime) String() string {
	return dt.Time.Format(time.DateTime)
}

func (dt DateTime) Value() (driver.Value, error) {
	return dt.Time, nil
}

func (dt DateTime) Before(other DateTime) bool {
	return dt.Time.Before(other.Time)
}

func (dt DateTime) After(other DateTime) bool {
	return dt.Time.After(other.Time)
}

func (dt DateTime) Equal(other DateTime) bool {
	return dt.Time.Equal(other.Time)
}

func (dt DateTime) Compare(other DateTime) int {
	return dt.Time.Compare(other.Time)
}

func (dt DateTime) Sub(other DateTime) time.Duration {
	return dt.Time.Sub(other.Time)
}

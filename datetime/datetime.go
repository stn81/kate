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

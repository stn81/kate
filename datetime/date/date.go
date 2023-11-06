package date

import (
	"database/sql/driver"
	"errors"
	"time"
)

type Date struct {
	time.Time
}

const Day = time.Hour * time.Duration(24)

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

func (dt Date) Prev(days int) Date {
	return dt.Next(-days)
}

func (dt Date) Next(days int) Date {
	return Date{
		Time: dt.Time.Add(Day * time.Duration(days)),
	}
}

func (dt *Date) UnmarshalBind(value string) error {
	var err error
	if dt.Time, err = time.Parse(time.DateOnly, value); err != nil {
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

func (dt Date) Scan(src any) error {
	t, ok := src.(time.Time)
	if ok {
		dt.Time = FromTime(t).Time
		return nil
	}
	return errors.New("invalid value, must be time.Time")
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
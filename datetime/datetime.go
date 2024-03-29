package datetime

import (
	"database/sql/driver"
	"errors"
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
	return dt.String(), nil
}

func (dt *DateTime) Scan(src any) error {
	t, ok := src.(time.Time)
	if ok {
		dt.Time = FromTime(t).Time
		return nil
	}
	return errors.New("invalid value, must be time.Time")
}

func (dt DateTime) MarshalJSON() ([]byte, error) {
	bytes := []byte(`"` + dt.String() + `"`)
	return bytes, nil
}

func (dt *DateTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("DateTime.UnmarshalJSON: input is not a JSON string")
	}
	data = data[len(`"`) : len(data)-len(`"`)]
	t, err := time.Parse(time.DateTime, string(data))
	if err != nil {
		return err
	}
	*dt = DateTime{
		Time: t,
	}
	return nil
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

package datetime

import "time"

type DateTime struct {
	time.Time
}

func (dt *DateTime) UnmarshalBind(value string) error {
	var err error
	if dt.Time, err = time.Parse(time.DateTime, value); err != nil {
		return err
	}
	return nil
}

func (dt *DateTime) String() string {
	return dt.Time.Format(time.DateTime)
}

type Date struct {
	time.Time
}

func (dt *Date) UnmarshalBind(value string) error {
	var err error
	if dt.Time, err = time.Parse(time.DateOnly, value); err != nil {
		return err
	}
	return nil
}

func (dt *Date) String() string {
	return dt.Time.Format(time.DateOnly)
}

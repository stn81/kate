package datetime

import "time"

type DateTime struct {
	time.Time
	Valid bool
}

func (dt *DateTime) UnmarshalBind(value string) error {
	var err error
	if dt.Time, err = time.Parse(time.DateTime, value); err != nil {
		return err
	}
	dt.Valid = true
	return nil
}

type Date struct {
	time.Time
	Valid bool
}

func (dt *Date) UnmarshalBind(value string) error {
	var err error
	if dt.Time, err = time.Parse(time.DateOnly, value); err != nil {
		return err
	}
	dt.Valid = true
	return nil
}

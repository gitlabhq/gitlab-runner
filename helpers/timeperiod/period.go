package timeperiod

import (
	"time"
)

var daysOfWeek = map[string]time.Weekday{
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
	"sun": time.Sunday,
}

type TimePeriod struct {
	DaysOfWeek     map[time.Weekday]bool
	Hours          map[int]bool
	GetCurrentTime func() time.Time
}

func (t *TimePeriod) InPeriod() bool {
	now := t.GetCurrentTime()

	return t.DaysOfWeek[now.Weekday()] && t.Hours[now.Hour()]
}

func TimePeriods(dow []string, h []int) *TimePeriod {
	days := make(map[time.Weekday]bool)
	for _, day := range dow {
		if val, ok := daysOfWeek[day]; ok {
			days[val] = true
		}
	}

	hours := make(map[int]bool)
	for _, hour := range h {
		hours[hour] = true
	}

	return &TimePeriod{
		DaysOfWeek:     days,
		Hours:          hours,
		GetCurrentTime: func() time.Time { return time.Now() },
	}
}

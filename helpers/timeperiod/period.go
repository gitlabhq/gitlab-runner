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
	DaysOfWeek []time.Weekday
	Hours      []int
}

func (t *TimePeriod) InPeriod() bool {
	now := time.Now()

	return containsDay(now.Weekday(), t.DaysOfWeek) && containsHour(now.Hour(), t.Hours)
}

func containsDay(needle time.Weekday, haystack []time.Weekday) bool {
	for _, val := range haystack {
		if needle == val {
			return true
		}
	}

	return false
}

func containsHour(needle int, haystack []int) bool {
	for _, val := range haystack {
		if needle == val {
			return true
		}
	}

	return false
}

func TimePeriods(dow []string, hours []int) *TimePeriod {
	var days []time.Weekday
	for _, day := range dow {
		if val, ok := daysOfWeek[day]; ok {
			days = append(days, val)
		}
	}

	return &TimePeriod{
		DaysOfWeek: days,
		Hours:      hours,
	}
}

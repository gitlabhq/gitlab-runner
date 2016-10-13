package timeperiod

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var daysOfWeekInv = make(map[time.Weekday]string, len(daysOfWeek))

func newTimePeriods(day time.Weekday, hour int) *TimePeriod {
	days := []string{daysOfWeekInv[day]}
	hours := []int{hour}

	return TimePeriods(days, hours)
}

func TestPeriodIn(t *testing.T) {
	now := time.Now()
	day := now.Weekday()
	hour := now.Hour()

	periods := newTimePeriods(day, hour)
	periods.GetCurrentTime = func() time.Time {
		return now
	}

	assert.True(t, periods.InPeriod())
}

func TestPeriodOut(t *testing.T) {
	now := time.Now()
	day := now.Weekday()
	hour := now.Hour()

	periods := newTimePeriods(day, hour)
	periods.GetCurrentTime = func() time.Time {
		return now.Add(time.Hour * 27)
	}

	assert.False(t, periods.InPeriod())
}

func init() {
	for name, value := range daysOfWeek {
		daysOfWeekInv[value] = name
	}
}

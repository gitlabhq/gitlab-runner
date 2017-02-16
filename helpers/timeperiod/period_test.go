package timeperiod

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var daysOfWeek = map[time.Weekday]string{
	time.Monday:    "mon",
	time.Tuesday:   "tue",
	time.Wednesday: "wed",
	time.Thursday:  "thu",
	time.Friday:    "fri",
	time.Saturday:  "sat",
	time.Sunday:    "sun",
}

func newTimePeriods(t *testing.T) (time.Time, *TimePeriod) {
	now := time.Now()
	minute := now.Minute()
	hour := now.Hour()
	day := now.Weekday()
	dayName := daysOfWeek[day]

	periodPattern := fmt.Sprintf("* %d %d * * %s *", minute, hour, dayName)
	timezone := "Europe/Warsaw"
	timePeriods, err := TimePeriods([]string{periodPattern}, timezone)
	assert.NoError(t, err)

	return now, timePeriods
}

func TestInPeriod(t *testing.T) {
	now, timePeriods := newTimePeriods(t)
	timePeriods.GetCurrentTime = func() time.Time {
		return now
	}

	assert.True(t, timePeriods.InPeriod())
}

func TestPeriodOut(t *testing.T) {
	now, timePeriods := newTimePeriods(t)
	timePeriods.GetCurrentTime = func() time.Time {
		return now.Add(time.Hour * 48)
	}
	assert.False(t, timePeriods.InPeriod())

	now, timePeriods = newTimePeriods(t)
	timePeriods.GetCurrentTime = func() time.Time {
		return now.Add(time.Hour * 4)
	}
	assert.False(t, timePeriods.InPeriod())

	now, timePeriods = newTimePeriods(t)
	timePeriods.GetCurrentTime = func() time.Time {
		return now.Add(time.Minute * 4)
	}
	assert.False(t, timePeriods.InPeriod())
}

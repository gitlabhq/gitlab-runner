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
	timePeriods, err := TimePeriods([]string{periodPattern}, "Europe/Berlin")
	assert.NoError(t, err)

	return now, timePeriods
}

func TestInvalidTimezone(t *testing.T) {
	_, err := TimePeriods([]string{}, "InvalidTimezone/String")
	assert.Error(t, err)
}

func TestOffPeakPeriod(t *testing.T) {
	timePeriods, _ := TimePeriods([]string{"* * 10-17 * * * *"}, "Europe/Berlin")
	timePeriods.GetCurrentTime = func() time.Time {
		return time.Date(2017, time.January, 1, 16, 30, 0, 0, time.UTC)
	}
	assert.True(t, timePeriods.InPeriod(), "2017-01-01 16:30:00 UTC (no DST time in Europe/Berlin, so the time is +1h to UTC = 17:30 - which is inside of '* * 10-17 * * * *')")
	timePeriods.GetCurrentTime = func() time.Time {
		return time.Date(2017, time.July, 1, 16, 30, 0, 0, time.UTC)
	}
	assert.False(t, timePeriods.InPeriod(), "2017-07-01 16:30:00 UTC (DST time in Europe/Berlin, so the time is +2h to UTC = 18:30 - which is outside of '* * 10-17 * * * *')")
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

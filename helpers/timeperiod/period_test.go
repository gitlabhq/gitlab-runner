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
	timePeriods, err := TimePeriods([]string{periodPattern}, "Local")
	assert.NoError(t, err)

	return now, timePeriods
}

func TestInvalidTimezone(t *testing.T) {
	_, err := TimePeriods([]string{}, "InvalidTimezone/String")
	assert.Error(t, err)
}

func testTimeperiodsWithTimezone(t *testing.T, period, timezone string, month time.Month, day, hour, minute int, inPeriod bool) {
	timePeriods, _ := TimePeriods([]string{period}, timezone)
	timePeriods.GetCurrentTime = func() time.Time {
		return time.Date(2017, month, day, hour, minute, 0, 0, time.UTC)
	}

	now := timePeriods.GetCurrentTime()
	nowInLocation := now.In(timePeriods.location)
	t.Log(fmt.Sprintf("Checking timeperiod '%s' in timezone '%s' for %s (%s)", period, timezone, now, nowInLocation))

	if inPeriod {
		assert.True(t, timePeriods.InPeriod(), "It should be inside of the period")
	} else {
		assert.False(t, timePeriods.InPeriod(), "It should be outside of the period")
	}
}

func TestOffPeakPeriod(t *testing.T) {
	// inside or outside of the timeperiod, basing on DST status
	testTimeperiodsWithTimezone(t, "* * 10-17 * * * *", "Europe/Berlin", time.January, 1, 16, 30, true)
	testTimeperiodsWithTimezone(t, "* * 10-17 * * * *", "Europe/Berlin", time.July, 1, 16, 30, false)

	// always inside of the timeperiod
	testTimeperiodsWithTimezone(t, "* * 10-17 * * * *", "Europe/Berlin", time.January, 1, 14, 30, true)
	testTimeperiodsWithTimezone(t, "* * 10-17 * * * *", "Europe/Berlin", time.July, 1, 14, 30, true)

	// always outside of the timeperiod
	testTimeperiodsWithTimezone(t, "* * 10-17 * * * *", "Europe/Berlin", time.January, 1, 20, 30, false)
	testTimeperiodsWithTimezone(t, "* * 10-17 * * * *", "Europe/Berlin", time.July, 1, 20, 30, false)
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

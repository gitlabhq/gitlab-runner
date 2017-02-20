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

func newTimePeriods(t *testing.T, timezone string) (time.Time, *TimePeriod) {
	location, err := time.LoadLocation(timezone)

	now := time.Now().In(location)
	minute := now.Minute()
	hour := now.Hour()
	day := now.Weekday()
	dayName := daysOfWeek[day]

	periodPattern := fmt.Sprintf("* %d %d * * %s *", minute, hour, dayName)
	timePeriods, err := TimePeriods([]string{periodPattern}, timezone)
	assert.NoError(t, err)

	return now, timePeriods
}

func TestWrongTimezone(t *testing.T) {
	assert.Panics(t, func() {
		_, _ = newTimePeriods(t, "NoValidTimezone/String")
	}, "calling newTimePeriods with a wrong timezone string should panic")
}

func TestTimezone(t *testing.T) {
	assert := assert.New(t)
	localtimezone, _ := time.Now().Zone()
	// make sure timezones are respected and make no difference
	_, timePeriods := newTimePeriods(t, localtimezone)
	assert.WithinDuration(timePeriods.GetCurrentTime(), time.Now(), 1*time.Second)
	_, timePeriods = newTimePeriods(t, "America/New_York")
	assert.WithinDuration(timePeriods.GetCurrentTime(), time.Now(), 1*time.Second)
	_, timePeriods = newTimePeriods(t, "Australia/Sydney")
	assert.WithinDuration(timePeriods.GetCurrentTime(), time.Now(), 1*time.Second)
	// make sure timezones are calculated correctly
	location, _ := time.LoadLocation("Australia/Sydney")
	assert.Equal(timePeriods.GetCurrentTime().Format("3:04PM"), time.Now().In(location).Format("3:04PM"))
}

func TestInPeriod(t *testing.T) {
	now, timePeriods := newTimePeriods(t, "")
	timePeriods.GetCurrentTime = func() time.Time {
		return now
	}

	assert.True(t, timePeriods.InPeriod())
}

func TestPeriodOut(t *testing.T) {
	now, timePeriods := newTimePeriods(t, "")
	timePeriods.GetCurrentTime = func() time.Time {
		return now.Add(time.Hour * 48)
	}
	assert.False(t, timePeriods.InPeriod())

	now, timePeriods = newTimePeriods(t, "")
	timePeriods.GetCurrentTime = func() time.Time {
		return now.Add(time.Hour * 4)
	}
	assert.False(t, timePeriods.InPeriod())

	now, timePeriods = newTimePeriods(t, "")
	timePeriods.GetCurrentTime = func() time.Time {
		return now.Add(time.Minute * 4)
	}
	assert.False(t, timePeriods.InPeriod())
}

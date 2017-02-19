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
	fmt.Printf("newTimePeriods: %s\n", []string{periodPattern})

	assert.NoError(t, err)

	return now, timePeriods
}

func TestWrongTimezone(t *testing.T) {
	assert.Panics(t, func() {
		_, _ = newTimePeriods(t, "NoValidTimezone/String")
	}, "calling newTimePeriods with a wrong timezone string should panic")
}

func TestTimezone(t *testing.T) {
	localtimezone, _ := time.Now().Zone()
	_, timePeriods := newTimePeriods(t, localtimezone)

	fmt.Printf("TestLocalTimezone: %s - %s\n", timePeriods.GetCurrentTime(), time.Now())
	assert.WithinDuration(t, timePeriods.GetCurrentTime(), time.Now(), 1*time.Second, "The difference should not be more than 1s")

	_, timePeriods = newTimePeriods(t, "America/New_York")
	fmt.Printf("TestNewYorkTimezone: %s - %s\n", timePeriods.GetCurrentTime(), time.Now())
	assert.WithinDuration(t, timePeriods.GetCurrentTime(), time.Now(), 1*time.Second, "The difference should not be more than 1s")

	_, timePeriods = newTimePeriods(t, "Australia/Sydney")
	location, _ := time.LoadLocation("Australia/Sydney")
	fmt.Printf("TestSydneyTimezone: %s - %s\n", timePeriods.GetCurrentTime(), time.Now())
	assert.WithinDuration(t, timePeriods.GetCurrentTime(), time.Now(), 1*time.Second, "The difference should not be more than 1s")
	assert.Equal(t, timePeriods.GetCurrentTime().Format("3:04PM"), time.Now().In(location).Format("3:04PM"))
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
		fmt.Printf("now.Add(time.Hour * 48): %+v\n", now.Add(time.Hour*48))
		return now.Add(time.Hour * 48)
	}
	assert.False(t, timePeriods.InPeriod())

	now, timePeriods = newTimePeriods(t, "")
	timePeriods.GetCurrentTime = func() time.Time {
		fmt.Printf("now.Add(time.Hour * 4): %+v\n", now.Add(time.Hour*4))
		return now.Add(time.Hour * 4)
	}
	assert.False(t, timePeriods.InPeriod())

	now, timePeriods = newTimePeriods(t, "")
	timePeriods.GetCurrentTime = func() time.Time {
		fmt.Printf("now.Add(time.Minute * 4): %+v\n", now.Add(time.Minute*4))
		return now.Add(time.Minute * 4)
	}
	assert.False(t, timePeriods.InPeriod())
}

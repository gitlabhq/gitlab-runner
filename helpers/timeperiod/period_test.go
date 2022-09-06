//go:build !integration

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

func testTimePeriods(t *testing.T, seconds int, getCurrentTime func(now time.Time) time.Time, inPeriod bool) {
	location, _ := time.LoadLocation("Local")
	now := time.Date(2017, time.February, 22, 14, 59, seconds, 0, location)

	minute := now.Minute()
	hour := now.Hour()
	dayofWeek := now.Weekday()
	day := daysOfWeek[dayofWeek]
	periodPattern := fmt.Sprintf("* %d %d * * %s *", minute, hour, day)

	timePeriods, err := TimePeriods([]string{periodPattern}, location.String())
	assert.NoError(t, err)
	timePeriods.GetCurrentTime = func() time.Time {
		return getCurrentTime(now)
	}

	t.Logf(
		"Testing periodPattern %q with time %q and currentTime %q",
		periodPattern,
		now,
		timePeriods.GetCurrentTime(),
	)
	if inPeriod {
		assert.True(t, timePeriods.InPeriod(), "It should be inside of the period")
	} else {
		assert.False(t, timePeriods.InPeriod(), "It should be outside of the period")
	}
}

func TestInPeriod(t *testing.T) {
	testTimePeriods(t, 0, func(now time.Time) time.Time { return now }, true)
	// TODO: Decide if this case should be fixed, and how to do this
	testTimePeriods(t, 59, func(now time.Time) time.Time { return now }, false)
	testTimePeriods(t, 0, func(now time.Time) time.Time { return now.Add(time.Hour * 48) }, false)
	testTimePeriods(t, 0, func(now time.Time) time.Time { return now.Add(time.Hour * 4) }, false)
	testTimePeriods(t, 0, func(now time.Time) time.Time { return now.Add(time.Minute * 4) }, false)
}

func TestInvalidTimezone(t *testing.T) {
	_, err := TimePeriods([]string{}, "InvalidTimezone/String")
	assert.Error(t, err)
}

// nolint:unparam
func testTimeperiodsWithTimezone(
	t *testing.T,
	period, timezone string,
	month time.Month,
	day, hour, minute int,
	inPeriod bool,
) {
	timePeriods, _ := TimePeriods([]string{period}, timezone)
	timePeriods.GetCurrentTime = func() time.Time {
		return time.Date(2017, month, day, hour, minute, 0, 0, time.UTC)
	}

	now := timePeriods.GetCurrentTime()
	nowInLocation := now.In(timePeriods.location)
	t.Logf("Checking timeperiod '%s' in timezone '%s' for %s (%s)", period, timezone, now, nowInLocation)

	if inPeriod {
		assert.True(t, timePeriods.InPeriod(), "It should be inside of the period")
	} else {
		assert.False(t, timePeriods.InPeriod(), "It should be outside of the period")
	}
}

func TestTimeperiodsWithTimezone(t *testing.T) {
	period := "* * 10-17 * * * *"
	timezone := "Europe/Berlin"

	// inside or outside of the timeperiod, basing on DST status
	testTimeperiodsWithTimezone(t, period, timezone, time.January, 1, 16, 30, true)
	testTimeperiodsWithTimezone(t, period, timezone, time.July, 1, 16, 30, false)

	// always inside of the timeperiod
	testTimeperiodsWithTimezone(t, period, timezone, time.January, 1, 14, 30, true)
	testTimeperiodsWithTimezone(t, period, timezone, time.July, 1, 14, 30, true)

	// always outside of the timeperiod
	testTimeperiodsWithTimezone(t, period, timezone, time.January, 1, 20, 30, false)
	testTimeperiodsWithTimezone(t, period, timezone, time.July, 1, 20, 30, false)
}

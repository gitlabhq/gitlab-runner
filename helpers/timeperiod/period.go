package timeperiod

import (
	"time"

	"github.com/gorhill/cronexpr"
)

type TimePeriod struct {
	expressions    []*cronexpr.Expression
	location       *time.Location
	GetCurrentTime func() time.Time
}

func (t *TimePeriod) InPeriod() bool {
	now := t.GetCurrentTime().In(t.location)
	for _, expression := range t.expressions {
		nextIn := expression.Next(now)
		timeSince := now.Sub(nextIn)
		if -time.Second <= timeSince && timeSince <= time.Second {
			return true
		}
	}

	return false
}

func TimePeriods(periods []string, timezone string) (*TimePeriod, error) {
	return TimePeriodsWithTimer(periods, timezone, time.Now)
}

func TimePeriodsWithTimer(periods []string, timezone string, timer func() time.Time) (*TimePeriod, error) {
	var expressions []*cronexpr.Expression

	for _, period := range periods {
		expression, err := cronexpr.Parse(period)
		if err != nil {
			return nil, err
		}

		expressions = append(expressions, expression)
	}

	// if not set, default to system setting (the empty string would mean UTC)
	if timezone == "" {
		timezone = "Local"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}

	timePeriod := &TimePeriod{
		expressions:    expressions,
		location:       location,
		GetCurrentTime: timer,
	}

	return timePeriod, nil
}

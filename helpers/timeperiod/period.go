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
	expressions := make([]*cronexpr.Expression, 0, len(periods))

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
		GetCurrentTime: func() time.Time { return time.Now() },
	}

	return timePeriod, nil
}

package timeperiod

import (
	"fmt"
	"time"

	"github.com/gorhill/cronexpr"
)

type TimePeriod struct {
	expressions    []*cronexpr.Expression
	GetCurrentTime func() time.Time
}

func (t *TimePeriod) InPeriod(timezone string) bool {
	// if not set, default to system setting (the empty string would mean UTC)
	if timezone == "" {
		timezone = "Local"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		// I don't want this function to return an error code.
		// The validity of the input should already be checked on config load.
		// But to be sure and able to test, we have this here.
		panic(fmt.Sprint("Invalid OffPeakTimeZone value: ", err))
	}
	now := t.GetCurrentTime().In(location)
	for _, expression := range t.expressions {
		nextIn := expression.Next(now)
		timeSince := now.Sub(nextIn)
		if timeSince <= time.Second && timeSince >= -time.Second {
			return true
		}
	}

	return false
}

func TimePeriods(periods []string) (*TimePeriod, error) {
	var expressions []*cronexpr.Expression

	for _, period := range periods {
		expression, err := cronexpr.Parse(period)
		if err != nil {
			return nil, err
		}

		expressions = append(expressions, expression)
	}

	timePeriod := &TimePeriod{
		expressions:    expressions,
		GetCurrentTime: func() time.Time { return time.Now() },
	}

	return timePeriod, nil
}

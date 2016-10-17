package timeperiod

import (
	"time"

	"github.com/gorhill/cronexpr"
)

type TimePeriod struct {
	expressions    []*cronexpr.Expression
	GetCurrentTime func() time.Time
}

func (t *TimePeriod) InPeriod() bool {
	for _, expression := range t.expressions {
		nextIn := expression.Next(t.GetCurrentTime())
		timeSince := time.Since(nextIn)
		if timeSince < time.Second && timeSince > -time.Second {
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

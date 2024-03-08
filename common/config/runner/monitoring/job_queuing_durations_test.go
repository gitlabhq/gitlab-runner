//go:build !integration

package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobQueuingDuration_GetActiveConfiguration(t *testing.T) {
	newTimer := func(now time.Time) func() time.Time {
		return func() time.Time {
			return now
		}
	}

	noMatchingDefinition := func(t *testing.T, configuration *JobQueuingDuration) {
		assert.Nil(t, configuration)
	}

	tests := map[string]struct {
		periods             [][]string
		timezone            string
		assertConfiguration func(t *testing.T, configuration *JobQueuingDuration)
	}{
		"no definitions": {
			timezone:            "UTC",
			assertConfiguration: noMatchingDefinition,
		},
		"no matching definitions": {
			periods: [][]string{
				{"* * 10 * * * *"},
				{"* * 08 * * * *"},
			},
			timezone:            "UTC",
			assertConfiguration: noMatchingDefinition,
		},
		"one matching definition": {
			periods: [][]string{
				{"* * 10 * * * *"},
				{"* * 15 * * * *"},
				{"* * 08 * * * *"},
			},
			timezone: "UTC",
			assertConfiguration: func(t *testing.T, configuration *JobQueuingDuration) {
				assert.NotNil(t, configuration)
				assert.Len(t, configuration.Periods, 1)
			},
		},
		"two matching definitions": {
			periods: [][]string{
				{"* * 10 * * * *"},
				{"* * 15 * * * *", "1 2 * * * * *"},
				{"* * 08 * * * *"},
				{"* * 15 * * * *", "3 4 * * * * *"},
			},
			timezone: "UTC",
			assertConfiguration: func(t *testing.T, configuration *JobQueuingDuration) {
				assert.NotNil(t, configuration)
				assert.Len(t, configuration.Periods, 2)
				assert.Contains(t, configuration.Periods, "3 4 * * * * *")
			},
		},
		"definition matching in different time zone": {
			periods: [][]string{
				{"* * 15 * * * *"},
			},
			timezone:            "Europe/Warsaw",
			assertConfiguration: noMatchingDefinition,
		},
		"empty periods field": {
			periods: [][]string{
				{},
			},
			timezone:            "UTC",
			assertConfiguration: noMatchingDefinition,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			var durations JobQueuingDurations

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			require.NoError(t, err)

			for _, periods := range tt.periods {
				durations = append(durations, &JobQueuingDuration{
					Periods:  periods,
					Timezone: tt.timezone,
					timer:    newTimer(parsedTime),
				})
			}

			err = durations.Compile()
			assert.NoError(t, err)

			require.NotNil(t, tt.assertConfiguration, "missing assertion function")
			tt.assertConfiguration(t, durations.GetActiveConfiguration())
		})
	}
}

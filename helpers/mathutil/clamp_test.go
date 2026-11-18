//go:build !integration

package mathutil_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/mathutil"
)

func TestClamp(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		tests := []struct {
			name      string
			v, lo, hi int
			want      int
		}{
			{"below lo", 0, 1, 10, 1},
			{"at lo", 1, 1, 10, 1},
			{"within range", 5, 1, 10, 5},
			{"at hi", 10, 1, 10, 10},
			{"above hi", 15, 1, 10, 10},
			{"lo equals hi", 5, 7, 7, 7},
			{"lo greater than hi returns hi", 5, 10, 3, 3},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.want, mathutil.Clamp(tc.v, tc.lo, tc.hi))
			})
		}
	})

	t.Run("float64", func(t *testing.T) {
		tests := []struct {
			name      string
			v, lo, hi float64
			want      float64
		}{
			{"below lo", 0.5, 1.0, 10.0, 1.0},
			{"within range", 5.5, 1.0, 10.0, 5.5},
			{"above hi", 11.0, 1.0, 10.0, 10.0},
			{"lo greater than hi returns hi", 5.0, 10.0, 3.0, 3.0},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.want, mathutil.Clamp(tc.v, tc.lo, tc.hi))
			})
		}
	})

	t.Run("time.Duration", func(t *testing.T) {
		tests := []struct {
			name      string
			v, lo, hi time.Duration
			want      time.Duration
		}{
			{"below lo", 30 * time.Second, time.Minute, 55 * time.Minute, time.Minute},
			{"within range", 5 * time.Minute, time.Minute, 55 * time.Minute, 5 * time.Minute},
			{"above hi", 60 * time.Minute, time.Minute, 55 * time.Minute, 55 * time.Minute},
			{"lo greater than hi returns hi", 5 * time.Minute, 55 * time.Minute, time.Minute, time.Minute},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.want, mathutil.Clamp(tc.v, tc.lo, tc.hi))
			})
		}
	})
}

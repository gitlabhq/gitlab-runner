//go:build !integration

package helpers_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type testBuffer struct {
	bytes.Buffer
	Error error
}

func (b *testBuffer) SendRawLog(args ...interface{}) {
	if b.Error != nil {
		return
	}

	_, b.Error = b.WriteString(fmt.Sprintln(args...))
}

func TestBuildSection(t *testing.T) {
	for num, tc := range []struct {
		name        string
		skipMetrics bool
		error       error
	}{
		{"Success", false, nil},
		{"Failure", false, fmt.Errorf("failing test")},
		{"SkipMetricsSuccess", true, nil},
		{"SkipMetricsFailure", true, fmt.Errorf("failing test")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := new(testBuffer)

			section := helpers.BuildSection{
				Name:        tc.name,
				SkipMetrics: tc.skipMetrics,
				Run:         func() error { return tc.error },
			}
			_ = section.Execute(logger)

			output := logger.String()
			assert.Nil(t, logger.Error, "case %d: Error: %s", num, logger.Error)
			for _, str := range []string{"section_start:", "section_end:", tc.name} {
				if tc.skipMetrics {
					assert.NotContains(t, output, str)
				} else {
					assert.Contains(t, output, str)
				}
			}
		})
	}
}

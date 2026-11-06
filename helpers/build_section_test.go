//go:build !integration

package helpers_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type testBuffer struct {
	bytes.Buffer
	Error error
}

func (b *testBuffer) SendRawLog(args ...any) {
	if b.Error != nil {
		return
	}

	_, b.Error = fmt.Fprintln(b, args...)
}

func TestBuildSection(t *testing.T) {
	for num, tc := range []struct {
		name        string
		skipMetrics bool
		collapsed   bool
		error       error
	}{
		{"Success", false, false, nil},
		{"Failure", false, false, fmt.Errorf("failing test")},
		{"Collapsed", false, true, nil},
		{"CollapsedFailure", false, true, fmt.Errorf("failing test")},
		{"SkipMetricsSuccess", true, false, nil},
		{"SkipMetricsFailure", true, false, fmt.Errorf("failing test")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := new(testBuffer)

			section := helpers.BuildSection{
				Name:        tc.name,
				SkipMetrics: tc.skipMetrics,
				Collapsed:   tc.collapsed,
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

			if tc.skipMetrics {
				return
			}

			start, end, found := strings.Cut(output, "section_end:")
			assert.True(t, found, "section end marker should be present")
			if tc.collapsed {
				assert.Contains(t, start, tc.name+"[collapsed=true]")
				assert.NotContains(t, end, "[collapsed=true]")
			} else {
				assert.NotContains(t, start, "[collapsed=true]")
			}
		})
	}
}

//go:build !integration

package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestVerifyVersionCommand(t *testing.T) {
	tests := map[string]struct {
		expected string
		wantErr  string
	}{
		"matching version passes": {expected: common.AppVersion.Version},
		"mismatch fails":          {expected: "17.0.0", wantErr: "does not match runner version"},
		"missing expected fails":  {expected: "", wantErr: "missing --expected"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := (&VerifyVersionCommand{Expected: tc.expected}).verify()

			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

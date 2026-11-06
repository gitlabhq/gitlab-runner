//go:build !integration

package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUIDGID(t *testing.T) {
	tests := map[string]struct {
		output  string
		wantUID int
		wantGID int
		wantErr string
	}{
		"named user": {
			output:  "uid=1000(alpine) gid=1000(alpine) groups=1000(alpine)",
			wantUID: 1000,
			wantGID: 1000,
		},
		"numeric user": {
			output:  "uid=1000 gid=1000 groups=1000",
			wantUID: 1000,
			wantGID: 1000,
		},
		"root user": {
			output:  "uid=0(root) gid=0(root) groups=0(root)",
			wantUID: 0,
			wantGID: 0,
		},
		"missing UID": {
			output:  "gid=1000(alpine) groups=1000(alpine)",
			wantErr: "UID not found",
		},
		"missing GID": {
			output:  "uid=1000(alpine) groups=1000(alpine)",
			wantErr: "GID not found",
		},
		"invalid UID": {
			output:  "uid=invalid gid=1000",
			wantErr: "parsing UID",
		},
		"invalid GID": {
			output:  "uid=1000 gid=invalid",
			wantErr: "parsing GID",
		},
		"empty output": {
			output:  "",
			wantErr: "UID not found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			uid, gid, err := ParseUIDGID(tt.output)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantUID, uid)
			assert.Equal(t, tt.wantGID, gid)
		})
	}
}

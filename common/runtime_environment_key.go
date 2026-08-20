package common

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// RuntimeEnvironmentKey identifies a suspended environment. The runner produces it
// when suspending a job and parses it when a follow-up job resumes. The
// runner-id and system-id route the resume back to the same runner instance
// that issued the suspension; the fields carry executor-specific state.
//
// Format: <runner-id>/<url-encoded-system-id>/<url-encoded-fields>
type RuntimeEnvironmentKey struct {
	RunnerID int64
	SystemID string
	Fields   url.Values
}

func (k RuntimeEnvironmentKey) String() string {
	return fmt.Sprintf("%d/%s/%s", k.RunnerID, url.PathEscape(k.SystemID), k.Fields.Encode())
}

func ParseRuntimeEnvironmentKey(s string) (RuntimeEnvironmentKey, error) {
	parts := strings.SplitN(s, "/", 3)
	if len(parts) != 3 {
		return RuntimeEnvironmentKey{}, fmt.Errorf("environment key: expected at least two '/' separators")
	}

	runnerID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return RuntimeEnvironmentKey{}, fmt.Errorf("environment key: invalid runner ID: %w", err)
	}
	if runnerID <= 0 {
		return RuntimeEnvironmentKey{}, fmt.Errorf("environment key: runner ID must be positive")
	}

	systemID, err := url.PathUnescape(parts[1])
	if err != nil {
		return RuntimeEnvironmentKey{}, fmt.Errorf("environment key: invalid system ID encoding: %w", err)
	}
	if systemID == "" {
		return RuntimeEnvironmentKey{}, fmt.Errorf("environment key: system ID is empty")
	}

	fields, err := url.ParseQuery(parts[2])
	if err != nil {
		return RuntimeEnvironmentKey{}, fmt.Errorf("environment key: invalid fields: %w", err)
	}

	return RuntimeEnvironmentKey{
		RunnerID: runnerID,
		SystemID: systemID,
		Fields:   fields,
	}, nil
}

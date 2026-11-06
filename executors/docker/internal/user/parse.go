package user

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseUIDGID parses the UID and GID from the output of the id command.
func ParseUIDGID(output string) (int, int, error) {
	var uid, gid int
	var hasUID, hasGID bool

	for field := range strings.FieldsSeq(output) {
		switch {
		case strings.HasPrefix(field, "uid="):
			value := strings.TrimPrefix(field, "uid=")
			value = strings.SplitN(value, "(", 2)[0]

			parsed, err := strconv.Atoi(value)
			if err != nil {
				return 0, 0, fmt.Errorf("parsing UID %q: %w", value, err)
			}

			uid = parsed
			hasUID = true

		case strings.HasPrefix(field, "gid="):
			value := strings.TrimPrefix(field, "gid=")
			value = strings.SplitN(value, "(", 2)[0]

			parsed, err := strconv.Atoi(value)
			if err != nil {
				return 0, 0, fmt.Errorf("parsing GID %q: %w", value, err)
			}

			gid = parsed
			hasGID = true
		}
	}

	if !hasUID {
		return 0, 0, fmt.Errorf("UID not found in id output %q", output)
	}

	if !hasGID {
		return 0, 0, fmt.Errorf("GID not found in id output %q", output)
	}

	return uid, gid, nil
}

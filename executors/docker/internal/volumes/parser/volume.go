package parser

import (
	"strings"
)

type Volume struct {
	Source      string
	Destination string
	Mode        string
}

func newVolume(source string, destination string, mode string) *Volume {
	return &Volume{
		Source:      source,
		Destination: destination,
		Mode:        mode,
	}
}

func (v *Volume) Definition() string {
	parts := make([]string, 0)

	if v.Source != "" {
		parts = append(parts, v.Source)
	}

	parts = append(parts, v.Destination)

	if v.Mode != "" {
		parts = append(parts, v.Mode)
	}

	return strings.Join(parts, ":")
}

func (v *Volume) Len() int {
	len := 0

	if v.Source != "" {
		len++
	}

	if v.Destination != "" {
		len++
	}

	return len
}

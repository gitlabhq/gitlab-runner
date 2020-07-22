package parser

import (
	"strings"
)

type Volume struct {
	Source          string
	Destination     string
	Mode            string
	BindPropagation string
}

func newVolume(source, destination string, mode string, bindPropagation string) *Volume {
	return &Volume{
		Source:          source,
		Destination:     destination,
		Mode:            mode,
		BindPropagation: bindPropagation,
	}
}

func (v *Volume) Definition() string {
	parts := make([]string, 0)
	builder := strings.Builder{}

	if v.Source != "" {
		parts = append(parts, v.Source)
	}

	parts = append(parts, v.Destination)

	if v.Mode != "" {
		parts = append(parts, v.Mode)
	}

	builder.WriteString(strings.Join(parts, ":"))

	if v.BindPropagation != "" {
		separator := ":"
		if v.Mode != "" {
			separator = ","
		}

		builder.WriteString(separator)
		builder.WriteString(v.BindPropagation)
	}

	return builder.String()
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

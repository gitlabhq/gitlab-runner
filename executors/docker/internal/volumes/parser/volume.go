package parser

import (
	"strings"
)

type Volume struct {
	Source          string
	Destination     string
	Mode            string
	Label           string
	BindPropagation string
}

func newVolume(source, destination, mode, label, bindPropagation string) *Volume {
	return &Volume{
		Source:          source,
		Destination:     destination,
		Mode:            mode,
		Label:           label,
		BindPropagation: bindPropagation,
	}
}

func (v *Volume) Definition() string {
	parts := make([]string, 0)
	builder := strings.Builder{}
	options := make([]string, 0)

	if v.Source != "" {
		parts = append(parts, v.Source)
	}

	parts = append(parts, v.Destination)

	if v.Mode != "" {
		options = append(options, v.Mode)
	}
	if v.Label != "" {
		options = append(options, v.Label)
	}
	if v.BindPropagation != "" {
		options = append(options, v.BindPropagation)
	}

	opts := strings.Join(options, ",")
	if opts != "" {
		parts = append(parts, opts)
	}

	builder.WriteString(strings.Join(parts, ":"))

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

package common

import (
	"fmt"
	"regexp"
)

// Rules of labels validation should be kept in sync with GitLab Rails side.
// Today (September 2025) they are defined at
// https://gitlab.com/gitlab-org/gitlab/-/blob/master/app/validators/json_schemas/ci_runner_labels.json

const (
	maxAllowedNumberOfLabels = 32

	labelKeyAllowedPattern   = `^[a-zA-Z0-9_][a-zA-Z0-9._-]{2,64}$`
	labelValueAllowedPattern = `^[a-zA-Z0-9._-]{1,256}$`
)

var (
	labelKeyAllowedRx   = regexp.MustCompile(labelKeyAllowedPattern)
	labelValueAllowedRx = regexp.MustCompile(labelValueAllowedPattern)

	ErrInvalidLabelKey     = fmt.Errorf("invalid label key, doesn't match %q", labelKeyAllowedPattern)
	ErrInvalidLabelValue   = fmt.Errorf("invalid label value, doesn't match %q", labelValueAllowedPattern)
	ErrLabelsCountExceeded = fmt.Errorf("exceeded maximum computed labels number of %d", maxAllowedNumberOfLabels)
)

type Labels map[string]string

func (l Labels) validatePatterns() error {
	for key, value := range l {
		if !labelKeyAllowedRx.MatchString(key) {
			return fmt.Errorf("%w: %s", ErrInvalidLabelKey, key)
		}

		if !labelValueAllowedRx.MatchString(value) {
			return fmt.Errorf("%w: %s", ErrInvalidLabelValue, value)
		}
	}

	return nil
}

func (l Labels) validateCount() error {
	if len(l) > maxAllowedNumberOfLabels {
		return ErrLabelsCountExceeded
	}

	return nil
}

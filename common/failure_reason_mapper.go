package common

import (
	"errors"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

const (
	maxMappingDepth = 10
)

var (
	errMaxMappingDepthExceeded = errors.New("exceeded max mapping depth")
)

type failureReasonMapper struct {
	supportedByGitLab []spec.JobFailureReason
	compatibilityMap  map[spec.JobFailureReason]spec.JobFailureReason
	maxMappingDepth   int

	// err is used only for tests. It allows us to check if `Map()` behavior is correct
	// and to validate whether the hardcoded failure reasons map creates problems like
	// mapping loop or too big mapping depth.
	err error
}

func newFailureReasonMapper(supported []spec.JobFailureReason) *failureReasonMapper {
	return &failureReasonMapper{
		supportedByGitLab: append(supported, alwaysSupportedFailureReasons...),
		compatibilityMap:  failureReasonsCompatibilityMap,
		maxMappingDepth:   maxMappingDepth,
	}
}

func (f *failureReasonMapper) Map(reason spec.JobFailureReason) spec.JobFailureReason {
	f.err = nil

	// No specific reason means it's a script failure
	// (or Runner doesn't yet detect that it's something else)
	if reason == "" {
		return ScriptFailure
	}

	// If the reason is supported by GitLab - we send it as is
	r, found := f.findSupported(reason)
	if found {
		return r
	}

	// If the reason is not supported by GitLab - it may be a new
	// reason extracted from previously existing one (for example
	// image pulling failure was previously reported as a more general
	// runner system failure)
	r, found = f.findBackwardCompatible(reason)
	if found {
		return r
	}

	// If we can't map the reason to one supported by GitLab -
	// let's call it "unknown".
	return UnknownFailure
}

func (f *failureReasonMapper) findSupported(reason spec.JobFailureReason) (spec.JobFailureReason, bool) {
	for _, supported := range f.supportedByGitLab {
		if reason == supported {
			return reason, true
		}
	}

	return UnknownFailure, false
}

func (f *failureReasonMapper) findBackwardCompatible(reason spec.JobFailureReason) (spec.JobFailureReason, bool) {
	for i := 0; i < f.maxMappingDepth; i++ {
		mappedReason, ok := f.compatibilityMap[reason]
		if !ok {
			return UnknownFailure, false
		}

		r, ok := f.findSupported(mappedReason)
		if ok {
			return r, true
		}

		reason = mappedReason
	}

	f.err = errMaxMappingDepthExceeded

	return UnknownFailure, true
}

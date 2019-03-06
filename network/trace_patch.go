package network

import (
	"errors"
)

type tracePatch struct {
	trace     []byte
	offset    int
	totalSize int
}

func (tp *tracePatch) Patch() []byte {
	return tp.trace[tp.offset:tp.totalSize]
}

func (tp *tracePatch) Offset() int {
	return tp.offset
}

func (tp *tracePatch) TotalSize() int {
	return tp.totalSize
}

func (tp *tracePatch) SetNewOffset(newOffset int) {
	tp.offset = newOffset
}

func (tp *tracePatch) ValidateRange() bool {
	if tp.totalSize >= tp.offset {
		return true
	}

	return false
}

func newTracePatch(trace []byte, offset int) (*tracePatch, error) {
	patch := &tracePatch{
		trace:     trace,
		offset:    offset,
		totalSize: len(trace),
	}

	if !patch.ValidateRange() {
		return nil, errors.New("Range is invalid, limit can't be less than offset")
	}

	return patch, nil
}

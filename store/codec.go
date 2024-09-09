package store

import (
	"encoding/gob"
	"errors"
	"io"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type jobEncoder interface {
	Encode(io.Writer, *common.Job) error
}

type jobDecoder interface {
	Decode(io.Reader, *common.Job) error
}

type jobCodec interface {
	jobEncoder
	jobDecoder
}

type gobJobCodec struct{}

func (gobJobCodec) Encode(w io.Writer, job *common.Job) error {
	if job == nil {
		return errors.New("cannot encode nil job")
	}

	return gob.NewEncoder(w).Encode(job)
}

func (gobJobCodec) Decode(r io.Reader, job *common.Job) error {
	if job == nil {
		return errors.New("cannot decode into nil job")
	}

	return gob.NewDecoder(r).Decode(job)
}

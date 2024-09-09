package store

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"io"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type jobEncoder interface {
	Encode(io.Writer, *common.Job) error
}

type jobDecoder interface {
	Decode(io.Reader) (*common.Job, error)
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

	state, err := job.ToEncoded()
	if err != nil {
		return err
	}

	return gob.NewEncoder(w).Encode(state)
}

func (gobJobCodec) Decode(r io.Reader) (*common.Job, error) {
	var encodedJob common.EncodedJob

	if err := gob.NewDecoder(r).Decode(&encodedJob); err != nil {
		return nil, err
	}

	return encodedJob.FromEncoded()
}

type JSONJobCodec struct{}

func (JSONJobCodec) Encode(w io.Writer, job *common.Job) error {
	if job == nil {
		return errors.New("cannot encode nil job")
	}

	state, err := job.ToEncoded()
	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(state)
}

func (JSONJobCodec) Decode(_ io.Reader) (*common.Job, error) {
	return nil, errors.New("not implemented")
}

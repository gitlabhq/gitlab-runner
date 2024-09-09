//go:build !integration

package store

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestGobJobCodec_EncodeDecode(t *testing.T) {
	codec := gobJobCodec{}

	originalJob := &common.Job{}
	originalJob.JobResponse = &common.JobResponse{ID: 1}

	var buf bytes.Buffer

	err := codec.Encode(&buf, originalJob)
	assert.NoError(t, err, "Encode should not return an error")

	decodedJob := &common.Job{}
	err = codec.Decode(&buf, decodedJob)
	assert.NoError(t, err, "Decode should not return an error")

	assert.Equal(t, originalJob, decodedJob, "Decoded job should be equal to the original job")
}

func TestGobJobCodec_EncodeNilJob(t *testing.T) {
	codec := gobJobCodec{}
	var buf bytes.Buffer

	err := codec.Encode(&buf, nil)
	assert.Error(t, err, "Encode should return an error when job is nil")
}

func TestGobJobCodec_DecodeNilJob(t *testing.T) {
	codec := gobJobCodec{}
	var buf bytes.Buffer

	err := codec.Decode(&buf, nil)
	assert.Error(t, err, "Decode should return an error when job is nil")
}

func TestGobJobCodec_DecodeInvalidData(t *testing.T) {
	codec := gobJobCodec{}
	var buf bytes.Buffer

	buf.Write([]byte("invalid data"))

	decodedJob := &common.Job{}
	err := codec.Decode(&buf, decodedJob)
	assert.Error(t, err, "Decode should return an error when data is invalid")
}

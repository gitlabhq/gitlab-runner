package s3

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
)

type bucketLocationTripper struct {
	bucketLocation string
}

// The Minio Golang library always attempts to query the bucket location and
// currently has no way of statically setting that value.  To avoid that
// lookup, the Runner cache uses the library only to generate the URLs,
// forgoing the library's API for uploading and downloading files. The custom
// Roundtripper stubs out any network requests that would normally be made via
// the library.
func (b *bucketLocationTripper) RoundTrip(req *http.Request) (res *http.Response, err error) {
	var buffer bytes.Buffer
	err = xml.NewEncoder(&buffer).Encode(b.bucketLocation)
	if err != nil {
		return
	}
	res = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&buffer),
	}
	return
}

func (b *bucketLocationTripper) CancelRequest(req *http.Request) {
	// Do nothing
}

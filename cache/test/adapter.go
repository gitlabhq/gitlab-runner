package test

import (
	"net/http"
	"net/url"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type testAdapter struct {
	objectName string
	useGoCloud bool
}

func (t *testAdapter) GetDownloadURL() *url.URL {
	return t.getURL("download")
}

func (t *testAdapter) GetUploadURL() *url.URL {
	return t.getURL("upload")
}

func (t *testAdapter) GetUploadHeaders() http.Header {
	headers := http.Header{}
	headers.Set("header-1", "a value")

	return headers
}

func (t *testAdapter) GetGoCloudURL() *url.URL {
	if t.useGoCloud {
		u, _ := url.Parse("gocloud://test")
		return u
	}

	return nil
}

func (t *testAdapter) GetUploadEnv() map[string]string {
	return map[string]string{
		"FIRST_VAR":  "123",
		"SECOND_VAR": "456",
	}
}

func (t *testAdapter) getURL(operation string) *url.URL {
	return &url.URL{
		Scheme: "test",
		Host:   operation,
		Path:   t.objectName,
	}
}

func New(_ *common.CacheConfig, _ time.Duration, objectName string) (cache.Adapter, error) {
	return &testAdapter{objectName: objectName}, nil
}

func NewGoCloudAdapter(_ *common.CacheConfig, _ time.Duration, objectName string) (cache.Adapter, error) {
	return &testAdapter{objectName: objectName, useGoCloud: true}, nil
}

func init() {
	if err := cache.Factories().Register("test", New); err != nil {
		panic(err)
	}

	if err := cache.Factories().Register("goCloudTest", NewGoCloudAdapter); err != nil {
		panic(err)
	}
}

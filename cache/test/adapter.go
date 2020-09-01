package test

import (
	"net/url"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type testAdapter struct {
	objectName string
}

func (t *testAdapter) GetDownloadURL() *url.URL {
	return t.getURL("download")
}

func (t *testAdapter) GetUploadURL() *url.URL {
	return t.getURL("upload")
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

func init() {
	err := cache.Factories().Register("test", New)
	if err != nil {
		panic(err)
	}
}

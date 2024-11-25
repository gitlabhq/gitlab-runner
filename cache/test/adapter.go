package test

import (
	"context"
	"fmt"
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

func (t *testAdapter) GetDownloadURL(ctx context.Context) cache.PresignedURL {
	return cache.PresignedURL{URL: t.getURL("download")}
}

func (t *testAdapter) GetUploadURL(ctx context.Context) cache.PresignedURL {
	return cache.PresignedURL{URL: t.getURL("upload"), Headers: t.GetUploadHeaders()}
}

func (t *testAdapter) GetUploadHeaders() http.Header {
	headers := http.Header{}
	headers.Set("header-1", "a value")

	return headers
}

func (t *testAdapter) GetGoCloudURL(ctx context.Context, _ bool) cache.GoCloudURL {
	goCloudURL := cache.GoCloudURL{}

	if t.useGoCloud {
		u, _ := url.Parse(fmt.Sprintf("gocloud://test/%s", t.objectName))
		goCloudURL.URL = u

		env, _ := t.GetUploadEnv(ctx)
		goCloudURL.Environment = env
		return goCloudURL
	}

	return goCloudURL
}

func (t *testAdapter) GetUploadEnv(_ context.Context) (map[string]string, error) {
	return map[string]string{
		"FIRST_VAR":  "123",
		"SECOND_VAR": "456",
	}, nil
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

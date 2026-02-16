package test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type testAdapter struct {
	objectName string
	useGoCloud bool
	metadata   map[string]string
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

	for k, v := range t.metadata {
		headers.Set("x-fakecloud-meta-"+k, v)
	}

	return headers
}

func (t *testAdapter) GetGoCloudURL(ctx context.Context, _ bool) (cache.GoCloudURL, error) {
	goCloudURL := cache.GoCloudURL{}

	if t.useGoCloud {
		u, _ := url.Parse(fmt.Sprintf("gocloud://test/%s", t.objectName))

		q := url.Values{}
		for k, v := range t.metadata {
			q.Add("x-fakecloud-meta-"+k, v)
		}
		u.RawQuery = q.Encode()

		goCloudURL.URL = u
		goCloudURL.Environment = t.getUploadEnv(ctx)

		return goCloudURL, nil
	}

	return goCloudURL, nil
}

func (t *testAdapter) WithMetadata(metadata map[string]string) {
	t.metadata = metadata
}

func (t *testAdapter) getUploadEnv(_ context.Context) map[string]string {
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

func New(_ *cacheconfig.Config, _ time.Duration, objectName string) (cache.Adapter, error) {
	return &testAdapter{objectName: objectName}, nil
}

func NewGoCloudAdapter(_ *cacheconfig.Config, _ time.Duration, objectName string) (cache.Adapter, error) {
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

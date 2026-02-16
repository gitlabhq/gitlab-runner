package cache

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type nopAdapter struct{}

func (nopAdapter) GetDownloadURL(context.Context) PresignedURL { return PresignedURL{} }
func (nopAdapter) WithMetadata(map[string]string)              {}
func (nopAdapter) GetUploadURL(context.Context) PresignedURL   { return PresignedURL{} }
func (nopAdapter) GetGoCloudURL(ctx context.Context, upload bool) (GoCloudURL, error) {
	return GoCloudURL{}, nil
}

var createAdapter = getCreateAdapter

func GetAdapter(config *cacheconfig.Config, timeout time.Duration, shortToken, projectId, key string) Adapter {
	if config == nil {
		return nopAdapter{}
	}

	if key == "" {
		logrus.Warning("Empty cache key. Skipping adapter selection.")
		return nopAdapter{}
	}

	// generate object path
	// runners get their own namespace, unless they're shared, in which case the
	// namespace is empty.
	namespace := ""
	if !config.GetShared() {
		namespace = path.Join("runner", shortToken)
	}
	basePath := path.Join(config.GetPath(), namespace, "project", projectId)
	fullPath := path.Join(basePath, key)

	// The typical concerns regarding the use of strings.HasPrefix to detect
	// path traversal do not apply here. The detection here is made easier
	// as we're dealing with URL paths, not filepaths and we're ensuring that
	// the basepath has a final separator (the key can not be empty).
	// TestGenerateObjectName contains path traversal tests.
	if !strings.HasPrefix(fullPath, basePath+"/") {
		logrus.WithError(fmt.Errorf("computed cache path outside of project bucket. Please remove `../` from cache key")).Error("Error while generating cache bucket.")
		return nopAdapter{}
	}

	adapter, err := createAdapter(config, timeout, fullPath)
	if err != nil {
		logrus.WithError(err).Error("Could not create cache adapter")
	}
	if adapter == nil {
		return nopAdapter{}
	}

	return adapter
}

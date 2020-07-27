package cache

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var createAdapter = CreateAdapter

func getCacheConfig(build *common.Build) *common.CacheConfig {
	if build == nil || build.Runner == nil || build.Runner.Cache == nil {
		return nil
	}

	return build.Runner.Cache
}

func generateBaseObjectName(build *common.Build, config *common.CacheConfig) string {
	runnerSegment := ""
	if !config.GetShared() {
		runnerSegment = path.Join("runner", build.Runner.ShortDescription())
	}

	return path.Join(config.GetPath(), runnerSegment, "project", strconv.Itoa(build.JobInfo.ProjectID))
}

func generateObjectName(build *common.Build, config *common.CacheConfig, key string) (string, error) {
	if key == "" {
		return "", nil
	}

	basePath := generateBaseObjectName(build, config)
	fullPath := path.Join(basePath, key)

	// The typical concerns regarding the use of strings.HasPrefix to detect
	// path traversal do not apply here. The detection here is made easier
	// as we're dealing with URL paths, not filepaths and we're ensuring that
	// the basepath has a final separator (the key can not be empty).
	// TestGenerateObjectName contains path traversal tests.
	if !strings.HasPrefix(fullPath, basePath+"/") {
		return "", fmt.Errorf("computed cache path outside of project bucket. Please remove `../` from cache key")
	}

	return fullPath, nil
}

func onAdapter(build *common.Build, key string, handler func(adapter Adapter) *url.URL) *url.URL {
	config := getCacheConfig(build)
	if config == nil {
		logrus.Warning("Cache config not defined. Skipping cache operation.")
		return nil
	}

	objectName, err := generateObjectName(build, config, key)
	if err != nil {
		logrus.WithError(err).Error("Error while generating cache bucket.")
		return nil
	}

	if objectName == "" {
		logrus.Warning("Empty cache key. Skipping adapter selection.")
		return nil
	}

	adapter, err := createAdapter(config, build.GetBuildTimeout(), objectName)
	if err != nil {
		logrus.WithError(err).Error("Could not create cache adapter")
	}

	if adapter == nil {
		return nil
	}

	return handler(adapter)
}

func GetCacheDownloadURL(build *common.Build, key string) *url.URL {
	return onAdapter(build, key, func(adapter Adapter) *url.URL {
		return adapter.GetDownloadURL()
	})
}

func GetCacheUploadURL(build *common.Build, key string) *url.URL {
	return onAdapter(build, key, func(adapter Adapter) *url.URL {
		return adapter.GetUploadURL()
	})
}

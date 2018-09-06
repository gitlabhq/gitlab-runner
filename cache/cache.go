package cache

import (
	"net/url"
	"path"
	"strconv"

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

func generateObjectName(build *common.Build, config *common.CacheConfig, key string) string {
	if key == "" {
		return ""
	}

	runnerSegment := ""
	if !config.GetShared() {
		runnerSegment = path.Join("runner", build.Runner.ShortDescription())
	}

	return path.Join(config.GetPath(), runnerSegment, "project", strconv.Itoa(build.JobInfo.ProjectID), key)
}

func onAdapter(build *common.Build, key string, handler func(adapter Adapter) *url.URL) *url.URL {
	config := getCacheConfig(build)
	if config == nil {
		logrus.Warning("Cache config not defined. Skipping cache operation.")
		return nil
	}

	objectName := generateObjectName(build, config, key)
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

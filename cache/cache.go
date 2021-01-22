package cache

import (
	"fmt"
	"net/http"
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

func buildAdapter(build *common.Build, key string) (Adapter, error) {
	config := getCacheConfig(build)
	if config == nil {
		logrus.Warning("Cache config not defined. Skipping cache operation.")
		return nil, nil
	}

	objectName, err := generateObjectName(build, config, key)
	if err != nil {
		logrus.WithError(err).Error("Error while generating cache bucket.")
		return nil, nil
	}

	if objectName == "" {
		logrus.Warning("Empty cache key. Skipping adapter selection.")
		return nil, nil
	}

	return createAdapter(config, build.GetBuildTimeout(), objectName)
}

func onAdapter(build *common.Build, key string, handler func(adapter Adapter) interface{}) interface{} {
	adapter, err := buildAdapter(build, key)
	if err != nil {
		logrus.WithError(err).Error("Could not create cache adapter")
	}

	if adapter == nil {
		return nil
	}

	return handler(adapter)
}

func GetCacheDownloadURL(build *common.Build, key string) *url.URL {
	return castToURL(func() interface{} {
		return onAdapter(build, key, func(adapter Adapter) interface{} {
			return adapter.GetDownloadURL()
		})
	})
}

func castToURL(handler func() interface{}) *url.URL {
	result := handler()

	u, ok := result.(*url.URL)
	if !ok {
		return nil
	}

	return u
}

func GetCacheUploadURL(build *common.Build, key string) *url.URL {
	return castToURL(func() interface{} {
		return onAdapter(build, key, func(adapter Adapter) interface{} {
			return adapter.GetUploadURL()
		})
	})
}

func GetCacheUploadHeaders(build *common.Build, key string) http.Header {
	result := onAdapter(build, key, func(adapter Adapter) interface{} {
		return adapter.GetUploadHeaders()
	})

	h, ok := result.(http.Header)
	if !ok {
		return nil
	}

	return h
}

func GetCacheGoCloudURL(build *common.Build, key string) *url.URL {
	return castToURL(func() interface{} {
		return onAdapter(build, key, func(adapter Adapter) interface{} {
			return adapter.GetGoCloudURL()
		})
	})
}

func GetCacheUploadEnv(build *common.Build, key string) map[string]string {
	result := onAdapter(build, key, func(adapter Adapter) interface{} {
		return adapter.GetUploadEnv()
	})

	m, ok := result.(map[string]string)
	if !ok {
		return nil
	}

	return m
}

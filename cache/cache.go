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

// generateObjectName returns a fully-qualified name for the cache object,
// ensuring there's no path traversal outside.
func generateObjectName(build *common.Build, config *common.CacheConfig, key string) (string, error) {
	if key == "" {
		return "", nil
	}

	// runners get their own namespace, unless they're shared, in which case the
	// namespace is empty.
	namespace := ""
	if !config.GetShared() {
		namespace = path.Join("runner", build.Runner.ShortDescription())
	}

	basePath := path.Join(config.GetPath(), namespace, "project", strconv.FormatInt(build.JobInfo.ProjectID, 10))
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

func getAdaptorForBuild(build *common.Build, key string) Adapter {
	if build == nil || build.Runner == nil || build.Runner.Cache == nil {
		logrus.Warning("Cache config not defined. Skipping cache operation.")
		return nil
	}

	objectName, err := generateObjectName(build, build.Runner.Cache, key)
	if err != nil {
		logrus.WithError(err).Error("Error while generating cache bucket.")
		return nil
	}

	if objectName == "" {
		logrus.Warning("Empty cache key. Skipping adapter selection.")
		return nil
	}

	adapter, err := createAdapter(build.Runner.Cache, build.GetBuildTimeout(), objectName)
	if err != nil {
		logrus.WithError(err).Error("Could not create cache adapter")
	}

	return adapter
}

func GetCacheDownloadURL(build *common.Build, key string) *url.URL {
	adaptor := getAdaptorForBuild(build, key)
	if adaptor == nil {
		return nil
	}

	return adaptor.GetDownloadURL()
}

func GetCacheUploadURL(build *common.Build, key string) *url.URL {
	adaptor := getAdaptorForBuild(build, key)
	if adaptor == nil {
		return nil
	}

	return adaptor.GetUploadURL()
}

func GetCacheUploadHeaders(build *common.Build, key string) http.Header {
	adaptor := getAdaptorForBuild(build, key)
	if adaptor == nil {
		return nil
	}

	return adaptor.GetUploadHeaders()
}

func GetCacheGoCloudURL(build *common.Build, key string) *url.URL {
	adaptor := getAdaptorForBuild(build, key)
	if adaptor == nil {
		return nil
	}

	return adaptor.GetGoCloudURL()
}

func GetCacheUploadEnv(build *common.Build, key string) map[string]string {
	adaptor := getAdaptorForBuild(build, key)
	if adaptor == nil {
		return nil
	}

	return adaptor.GetUploadEnv()
}

//go:build integration && kubernetes

package kubernetes_test

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

var (
	testFeatureFlag      string
	testFeatureFlagValue bool
	ciNamespace          = cmp.Or(os.Getenv("CI_RUNNER_TEST_NAMESPACE"), common.DefaultKubernetesIntegrationTestNamespace)
)

func init() {
	rand.Seed(time.Now().UnixNano())
	testFeatureFlag = os.Getenv("CI_RUNNER_TEST_FEATURE_FLAG")
	if testFeatureFlag != "" {
		var err error
		testFeatureFlagValue, err = strconv.ParseBool(os.Getenv("CI_RUNNER_TEST_FEATURE_FLAG_VALUE"))
		if err != nil {
			panic(err)
		}
	}
}

type kubernetesNamespaceManagerAction int64

const (
	createNamespace kubernetesNamespaceManagerAction = iota
	deleteNamespace
	// counterServiceImage counts to 10 and exits
	counterServiceImage = "registry.gitlab.com/gitlab-org/gitlab-runner/test/counter-service:v1"
)

type namespaceManager struct {
	action      kubernetesNamespaceManagerAction
	namespace   string
	client      *k8s.Clientset
	maxAttempts int
	timeout     time.Duration
}

func newNamespaceManager(client *k8s.Clientset, action kubernetesNamespaceManagerAction, namespace string) *namespaceManager {
	return &namespaceManager{
		namespace:   namespace,
		action:      action,
		client:      client,
		maxAttempts: 3,
		timeout:     time.Minute,
	}
}

func (n *namespaceManager) Run() (*v1.Namespace, error) {
	var err error
	var ns *v1.Namespace

	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()

	switch n.action {
	case createNamespace:
		k8sNamespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:         n.namespace,
				GenerateName: n.namespace,
			},
		}
		ns, err = n.client.CoreV1().Namespaces().Create(ctx, k8sNamespace, metav1.CreateOptions{})
	case deleteNamespace:
		err = n.client.CoreV1().Namespaces().Delete(ctx, n.namespace, metav1.DeleteOptions{})
	}

	return ns, err
}

func (n namespaceManager) ShouldRetry(tries int, err error) bool {
	return tries < n.maxAttempts && err != nil
}

type featureFlagTest func(t *testing.T, flagName string, flagValue bool)

func TestRunIntegrationTestsWithFeatureFlag(t *testing.T) {
	t.Parallel()

	tests := map[string]featureFlagTest{
		"testKubernetesSuccessRun":                                testKubernetesSuccessRunFeatureFlag,
		"testKubernetesMultistepRun":                              testKubernetesMultistepRunFeatureFlag,
		"testKubernetesTimeoutRun":                                testKubernetesTimeoutRunFeatureFlag,
		"testKubernetesBuildFail":                                 testKubernetesBuildFailFeatureFlag,
		"testKubernetesBuildCancel":                               testKubernetesBuildCancelFeatureFlag,
		"testKubernetesBuildLogLimitExceeded":                     testKubernetesBuildLogLimitExceededFeatureFlag,
		"testKubernetesBuildMasking":                              testKubernetesBuildMaskingFeatureFlag,
		"testKubernetesBuildPassingEnvsMultistep":                 testKubernetesBuildPassingEnvsMultistep,
		"testKubernetesCustomClonePath":                           testKubernetesCustomClonePathFeatureFlag,
		"testKubernetesNoRootImage":                               testKubernetesNoRootImageFeatureFlag,
		"testKubernetesMissingImage":                              testKubernetesMissingImageFeatureFlag,
		"testKubernetesMissingTag":                                testKubernetesMissingTagFeatureFlag,
		"testKubernetesFailingToPullImageTwiceFeatureFlag":        testKubernetesFailingToPullImageTwiceFeatureFlag,
		"testKubernetesFailingToPullServiceImageTwiceFeatureFlag": testKubernetesFailingToPullSvcImageTwiceFeatureFlag,
		"testKubernetesFailingToPullHelperTwiceFeatureFlag":       testKubernetesFailingToPullHelperTwiceFeatureFlag,
		"testOverwriteNamespaceNotMatch":                          testOverwriteNamespaceNotMatchFeatureFlag,
		"testOverwriteServiceAccountNotMatch":                     testOverwriteServiceAccountNotMatchFeatureFlag,
		"testInteractiveTerminal":                                 testInteractiveTerminalFeatureFlag,
		"testKubernetesReplaceEnvFeatureFlag":                     testKubernetesReplaceEnvFeatureFlag,
		"testKubernetesReplaceMissingEnvVarFeatureFlag":           testKubernetesReplaceMissingEnvVarFeatureFlag,
		"testKubernetesWithNonRootSecurityContext":                testKubernetesWithNonRootSecurityContext,
		"testBuildsDirDefaultVolumeFeatureFlag":                   testBuildsDirDefaultVolumeFeatureFlag,
		"testBuildsDirVolumeMountEmptyDirFeatureFlag":             testBuildsDirVolumeMountEmptyDirFeatureFlag,
		"testBuildsDirVolumeMountHostPathFeatureFlag":             testBuildsDirVolumeMountHostPathFeatureFlag,
		"testKubernetesBashFeatureFlag":                           testKubernetesBashFeatureFlag,
		"testKubernetesContainerHookFeatureFlag":                  testKubernetesContainerHookFeatureFlag,
		"testKubernetesGarbageCollection":                         testKubernetesGarbageCollection,
		"testKubernetesNamespaceIsolation":                        testKubernetesNamespaceIsolation,
		"testKubernetesPublicInternalVariables":                   testKubernetesPublicInternalVariables,
		"testKubernetesWaitResources":                             testKubernetesWaitResources,
		"testKubernetesLongLogsFeatureFlag":                       testKubernetesLongLogsFeatureFlag,
		"testKubernetesHugeScriptAndAfterScriptFeatureFlag":       testKubernetesHugeScriptAndAfterScriptFeatureFlag,
		"testKubernetesCustomPodSpec":                             testKubernetesCustomPodSpec,
		"testKubernetesClusterWarningEvent":                       testKubernetesClusterWarningEvent,
		"testKubernetesFailingBuildForBashAndPwshFeatureFlag":     testKubernetesFailingBuildForBashAndPwshFeatureFlag,
		"testKubernetesPodEvents":                                 testKubernetesPodEvents,
		"testKubernetesDumbInitSuccessRun":                        testKubernetesDumbInitSuccessRun,
		"testKubernetesDisableUmask":                              testKubernetesDisableUmask,
		"testKubernetesNoAdditionalNewLines":                      testKubernetesNoAdditionalNewLines,
		"testJobRunningAndPassingWhenServiceStops":                testJobRunningAndPassingWhenServiceStops,
		"testJobAgainstServiceContainerBehaviour":                 testJobAgainstServiceContainerBehaviour,
		"testDeletedPodSystemFailureDuringExecution":              testDeletedPodSystemFailureDuringExecution,
		"testKubernetesServiceContainerAlias":                     testKubernetesServiceContainerAlias,
		"testKubernetesOptionsUserAndGroup":                       testKubernetesOptionsUserAndGroup,
	}

	ffValues := []bool{testFeatureFlagValue}
	ff := testFeatureFlag
	if ff == "" {
		ff = featureflags.UseLegacyKubernetesExecutionStrategy
		ffValues = []bool{true, false}
	}

	for name, testFunc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, ffValue := range ffValues {
				toggleText := "off"
				if ffValue {
					toggleText = "on"
				}

				t.Run(ff+":"+toggleText, func(t *testing.T) {
					t.Parallel()
					testFunc(t, ff, ffValue)
				})
			}
		})
	}
}

func testKubernetesSuccessRunFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func testKubernetesPodEvents(t *testing.T, featureFlagName string, featureFlagValue bool) {
	t.Skip("TODO: Fix events not properly tested for or waited for - https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/8532408889")
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
	build.Image.Name = common.TestAlpineImage
	build.Variables = append(
		build.Variables,
		spec.Variable{Key: featureflags.PrintPodEvents, Value: "true"},
	)

	out, err := buildtest.RunBuildReturningOutput(t, build)
	require.NoError(t, err)

	expectedLines := []string{
		"Type     Reason      Message",
		"Normal   Scheduled   Successfully assigned",
	}

	if build.Variables.Get(featureflags.UseLegacyKubernetesExecutionStrategy) == "false" {
		expectedLines = append(
			expectedLines,
			"Normal   Created   Created container init-permissions",
			"Normal   Started   Started container init-permissions",
		)
	}

	expectedLines = append(
		expectedLines,
		"Normal   Pulling   Pulling image|Normal   Pulled   Successfully pulled image|Normal   Pulled   Container image .* already present on machine",
		"Normal   Created   Created container build",
		"Normal   Started   Started container build",
		"Normal   Created   Created container helper",
		"Normal   Started   Started container helper",
	)

	for _, l := range expectedLines {
		assert.Regexp(t, regexp.MustCompile(fmt.Sprintf(`(?m)%s`, l)), out)
	}
}

func testKubernetesBuildPassingEnvsMultistep(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		if runtime.GOOS != shells.OSWindows && shell == shells.SNPowershell {
			t.Skip("Powershell is not supported on non-windows systems")
		}
		if shell == shells.SNPwsh {
			t.Skip("TODO: Fix pwsh fails")
		}

		build := getTestBuild(t, func() (spec.Job, error) {
			return spec.Job{}, nil
		})
		build.Runner.RunnerSettings.Shell = shell

		withDevHelperImage(t, build, "")

		buildtest.RunBuildWithPassingEnvsMultistep(
			t,
			build.Runner,
			func(_ *testing.T, build *common.Build) {
				buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
			},
		)
	})
}

func testKubernetesDumbInitSuccessRun(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
	buildtest.SetBuildFeatureFlag(build, featureflags.UseDumbInitWithKubernetesExecutor, true)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func testKubernetesDisableUmask(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	customBuildDir := "/custom_builds_dir"
	customCacheDir := "/custom_cache_dir"
	tests := map[string]struct {
		image        string
		shell        string
		buildDir     string
		cacheDir     string
		cache        spec.Caches
		script       string
		runAsUser    int64
		runAsGroup   int64
		disableUmask bool
		envars       spec.Variables
		verifyFn     func(t *testing.T, out string)
	}{
		"umask enabled": {
			image:      common.TestAlpineImage,
			shell:      "bash",
			script:     "ls -lR /builds/gitlab-org/ci-cd/gitlab-runner-pipeline-tests",
			runAsUser:  int64(1234),
			runAsGroup: int64(5678),
			verifyFn: func(t *testing.T, out string) {
				assert.NotContains(t, out, "1234")
				assert.NotContains(t, out, "5678")
				assert.NotContains(t, out, "drwxr-xr-x")
				assert.NotContains(t, out, "-rw-r--r--")
				assert.Regexp(t, regexp.MustCompile(`(?m)^.*root\s*root.*gitlab-test.*$`), out)
			},
		},
		"umask disabled": {
			image:        common.TestAlpineImage,
			shell:        "bash",
			script:       "ls -lR /builds/gitlab-org/ci-cd/gitlab-runner-pipeline-tests",
			runAsUser:    int64(1234),
			runAsGroup:   int64(5678),
			disableUmask: true,
			verifyFn: func(t *testing.T, out string) {
				assert.NotContains(t, out, "root")
				assert.NotContains(t, out, "drwxrwxrwx")
				assert.NotContains(t, out, "-rw-rw-rw-")
				assert.Regexp(t, regexp.MustCompile(`(?m)^.*1234\s*5678.*gitlab-test.*$`), out)
			},
		},
		"umask disabled and shell not set": {
			image:        common.TestAlpineImage,
			script:       "ls -lR /builds/gitlab-org/ci-cd/gitlab-runner-pipeline-tests",
			runAsUser:    int64(1234),
			runAsGroup:   int64(5678),
			disableUmask: true,
			verifyFn: func(t *testing.T, out string) {
				assert.NotContains(t, out, "root")
				assert.NotContains(t, out, "drwxrwxrwx")
				assert.NotContains(t, out, "-rw-rw-rw-")
				assert.Regexp(t, regexp.MustCompile(`(?m)^.*1234\s*5678.*gitlab-test.*$`), out)
			},
		},
		"umask enabled with custom builds_dir": {
			image:      common.TestAlpineImage,
			shell:      "bash",
			buildDir:   customBuildDir,
			script:     "ls -lR $BUILDS_DIRECTORY/gitlab-org/ci-cd/gitlab-runner-pipeline-tests",
			runAsUser:  int64(1234),
			runAsGroup: int64(5678),
			envars: spec.Variables{
				spec.Variable{Key: "BUILDS_DIRECTORY", Value: customBuildDir},
			},
			verifyFn: func(t *testing.T, out string) {
				assert.NotContains(t, out, "1234")
				assert.NotContains(t, out, "5678")
				assert.NotContains(t, out, "drwxr-xr-x")
				assert.NotContains(t, out, "-rw-r--r--")
				assert.Regexp(t, regexp.MustCompile(`(?m)^.*root\s*root.*gitlab-test.*$`), out)
			},
		},
		"umask disabled with custom builds_dir": {
			image:        common.TestAlpineImage,
			shell:        "bash",
			buildDir:     customBuildDir,
			script:       "ls -lR $BUILDS_DIRECTORY/gitlab-org/ci-cd/gitlab-runner-pipeline-tests",
			runAsUser:    int64(1234),
			runAsGroup:   int64(5678),
			disableUmask: true,
			envars: spec.Variables{
				spec.Variable{Key: "BUILDS_DIRECTORY", Value: customBuildDir},
			},
			verifyFn: func(t *testing.T, out string) {
				assert.NotContains(t, out, "root")
				assert.NotContains(t, out, "drwxrwxrwx")
				assert.NotContains(t, out, "-rw-rw-rw-")
				assert.Regexp(t, regexp.MustCompile(`(?m)^.*1234\s*5678.*gitlab-test.*$`), out)
			},
		},
		"umask disabled with cache manipulation": {
			image:    common.TestAlpineImage,
			buildDir: customBuildDir,
			script:   "mkdir -p cache_files && touch cache_files/cache && ls -lR $BUILDS_DIRECTORY/",
			cache: spec.Caches{
				spec.Cache{
					Key:    "key",
					Paths:  spec.ArtifactPaths{"cache_files"},
					Policy: spec.CachePolicyPullPush,
					When:   spec.CacheWhenOnSuccess,
				},
			},
			runAsUser:    int64(1234),
			runAsGroup:   int64(5678),
			disableUmask: true,
			envars: spec.Variables{
				spec.Variable{Key: "BUILDS_DIRECTORY", Value: customBuildDir},
			},
			verifyFn: func(t *testing.T, out string) {
				assert.NotContains(t, out, "root")
				assert.NotContains(t, out, "drwxrwxrwx")
				assert.NotContains(t, out, "-rw-rw-rw-")
				assert.Regexp(t, regexp.MustCompile(`(?m)^.*1234\s*5678.*gitlab-test.*$`), out)
			},
		},
		"umask disabled with cache manipulation with custom cache_dir": {
			image:    common.TestAlpineImage,
			buildDir: customBuildDir,
			cacheDir: customCacheDir,
			script:   "mkdir -p cache_files && touch cache_files/cache && ls -lR $BUILDS_DIRECTORY/",
			cache: spec.Caches{
				spec.Cache{
					Key:    "key",
					Paths:  spec.ArtifactPaths{"cache_files"},
					Policy: spec.CachePolicyPullPush,
					When:   spec.CacheWhenOnSuccess,
				},
			},
			runAsUser:    int64(1234),
			runAsGroup:   int64(5678),
			disableUmask: true,
			envars: spec.Variables{
				spec.Variable{Key: "BUILDS_DIRECTORY", Value: customBuildDir},
				spec.Variable{Key: "CACHE_DIRECTORY", Value: customCacheDir},
			},
			verifyFn: func(t *testing.T, out string) {
				assert.NotContains(t, out, "root")
				assert.NotContains(t, out, "drwxrwxrwx")
				assert.NotContains(t, out, "-rw-rw-rw-")
				assert.Regexp(t, regexp.MustCompile(`(?m)^.*1234\s*5678.*gitlab-test.*$`), out)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse(tc.script)
			})

			build.Variables = append(build.Variables, tc.envars...)
			build.Runner.RunnerSettings.Shell = tc.shell
			build.Job.Image.Name = tc.image
			build.Job.Cache = tc.cache

			if tc.buildDir != "" {
				build.Runner.BuildsDir = tc.buildDir
				build.Runner.Kubernetes.Volumes = common.KubernetesVolumes{
					EmptyDirs: []common.KubernetesEmptyDir{
						{
							Name:      "repo",
							MountPath: "$BUILDS_DIRECTORY",
						},
					},
				}
			}

			if tc.cacheDir != "" {
				build.Runner.CacheDir = tc.cacheDir
				build.Runner.Kubernetes.Volumes = common.KubernetesVolumes{
					EmptyDirs: []common.KubernetesEmptyDir{
						{
							Name:      "cache",
							MountPath: "$CACHE_DIRECTORY",
						},
					},
				}
			}

			build.Runner.Kubernetes.BuildContainerSecurityContext = common.KubernetesContainerSecurityContext{
				RunAsUser:  &tc.runAsUser,
				RunAsGroup: &tc.runAsGroup,
			}
			build.Runner.Kubernetes.HelperImage = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest"

			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
			buildtest.SetBuildFeatureFlag(build, "FF_DISABLE_UMASK_FOR_KUBERNETES_EXECUTOR", tc.disableUmask)

			var buf bytes.Buffer
			err := build.Run(&common.Config{}, &common.Trace{Writer: &buf})
			assert.NoError(t, err)

			tc.verifyFn(t, buf.String())
		})
	}
}

func testKubernetesNoAdditionalNewLines(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (spec.Job, error) {
		return common.GetRemoteBuildResponse("for i in $(seq 1 120); do printf .; sleep 0.02; done; echo")
	})

	build.Runner.RunnerSettings.Shell = "bash"
	build.Job.Image.Name = common.TestAlpineImage
	build.Runner.Kubernetes.HelperImage = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest"

	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	var buf bytes.Buffer
	err := build.Run(&common.Config{}, &common.Trace{Writer: &buf})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "........................................................................................................................")
}

func TestBuildScriptSections(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		if shell != "bash" {
			t.Skip("TODO: fix this test for non-bash shells. This wasn't working before anyways because the image was never set correctly.")
		}

		t.Parallel()
		build := getTestBuild(t, func() (spec.Job, error) {
			return common.GetRemoteBuildResponse(`echo "Hello
World"`)
		})
		build.Runner.RunnerSettings.Shell = shell
		if shell != "bash" {
			build.Runner.Kubernetes.Image = common.TestPwshImage
		}

		buildtest.RunBuildWithSections(t, build)
	})
}

func TestEntrypointNotIgnored(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	buildTestJob := func() (spec.Job, error) {
		return common.GetRemoteBuildResponse(
			"if [ -f /tmp/debug.log ]; then",
			"cat /tmp/debug.log",
			"else",
			"echo 'file not found'",
			"fi",
			"echo \"I am now `whoami`\"",
		)
	}

	helperTestJob := func() (spec.Job, error) {
		return common.GetRemoteBuildResponse(
			"if [ -f /builds/debug.log ]; then",
			"cat /builds/debug.log",
			"else",
			"echo 'file not found'",
			"fi",
			"echo \"I am now `whoami`\"",
		)
	}

	testCases := map[string]struct {
		jobResponse          func() (spec.Job, error)
		buildImage           string
		helperImage          string
		useHonorEntrypointFF bool
		expectedOutputLines  []string
	}{
		"build image with entrypoint feature flag off": {
			jobResponse:          buildTestJob,
			buildImage:           common.TestAlpineEntrypointImage,
			useHonorEntrypointFF: false,
			expectedOutputLines:  []string{"I am now root", "file not found"},
		},
		"build image with entrypoint feature flag on": {
			jobResponse:          buildTestJob,
			buildImage:           common.TestAlpineEntrypointImage,
			useHonorEntrypointFF: true,
			expectedOutputLines:  []string{"I am now nobody", "this has been executed through a custom entrypoint"},
		},
		"helper image with entrypoint feature flag off": {
			jobResponse:          helperTestJob,
			helperImage:          common.TestHelperEntrypointImage,
			useHonorEntrypointFF: false,
			expectedOutputLines:  []string{"I am now root", "file not found"},
		},
		"helper image with entrypoint feature flag on": {
			jobResponse:          helperTestJob,
			helperImage:          common.TestHelperEntrypointImage,
			useHonorEntrypointFF: true,
			expectedOutputLines:  []string{"I am now nobody", "this has been executed through a custom entrypoint"},
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuildWithImage(t, common.TestAlpineEntrypointImage, func() (spec.Job, error) {
				jobResponse, err := tc.jobResponse()
				if err != nil {
					return spec.Job{}, err
				}

				jobResponse.Image = spec.Image{
					Name: common.TestAlpineEntrypointImage,
				}

				return jobResponse, nil
			})

			if tc.helperImage != "" {
				build.Runner.Kubernetes.HelperImage = common.TestHelperEntrypointImage
			}

			build.Variables = append(
				build.Variables,
				spec.Variable{Key: featureflags.KubernetesHonorEntrypoint, Value: strconv.FormatBool(tc.useHonorEntrypointFF)},
			)

			out, err := buildtest.RunBuildReturningOutput(t, build)
			require.NoError(t, err)

			t.Log(out)

			for _, expectedLine := range tc.expectedOutputLines {
				assert.Contains(t, out, expectedLine)
			}
		})
	}
}

func testKubernetesMultistepRunFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	successfulBuild, err := common.GetRemoteSuccessfulMultistepBuild()
	require.NoError(t, err)

	failingScriptBuild, err := common.GetRemoteFailingMultistepBuild(spec.StepNameScript)
	require.NoError(t, err)

	failingReleaseBuild, err := common.GetRemoteFailingMultistepBuild("release")
	require.NoError(t, err)

	successfulBuild.Image.Name = common.TestDockerGitImage
	failingScriptBuild.Image.Name = common.TestDockerGitImage
	failingReleaseBuild.Image.Name = common.TestDockerGitImage

	tests := map[string]struct {
		jobResponse    spec.Job
		expectedOutput []string
		unwantedOutput []string
		errExpected    bool
	}{
		"Successful build with release and after_script step": {
			jobResponse: successfulBuild,
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: false,
		},
		"Failure on script step. Release is skipped. After script runs.": {
			jobResponse: failingScriptBuild,
			expectedOutput: []string{
				"echo Hello World",
				"echo After Script",
			},
			unwantedOutput: []string{
				"echo Release",
			},
			errExpected: true,
		},
		"Failure on release step. After script runs.": {
			jobResponse: failingReleaseBuild,
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return tt.jobResponse, nil
			})
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			var buf bytes.Buffer
			err := build.Run(&common.Config{}, &common.Trace{Writer: &buf})

			out := buf.String()
			for _, output := range tt.expectedOutput {
				assert.Contains(t, out, output)
			}

			for _, output := range tt.unwantedOutput {
				assert.NotContains(t, out, output)
			}

			if tt.errExpected {
				var buildErr *common.BuildError
				assert.ErrorAs(t, err, &buildErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func testKubernetesTimeoutRunFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteLongRunningBuild)
	build.Image.Name = common.TestDockerGitImage
	build.RunnerInfo.Timeout = 10 // seconds
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	var buildError *common.BuildError
	assert.ErrorAs(t, err, &buildError)
	assert.Equal(t, common.JobExecutionTimeout, buildError.FailureReason)
}

func countWord(t *testing.T, text, word string) int {
	t.Helper()
	count := 0
	for w := range strings.FieldsSeq(text) {
		if w == word {
			count++
		}
	}
	return count
}

func testKubernetesLongLogsFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	timestampPattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z`)

	tests := map[string]struct {
		word          string
		log           string
		expectedCount int
	}{
		"short log": {
			word:          "Regular log",
			log:           "Regular log",
			expectedCount: 1,
		},
		"long log": {
			word:          "1",
			log:           strings.Repeat("1", common.DefaultReaderBufferSize),
			expectedCount: common.DefaultReaderBufferSize,
		},
		"really long log": {
			word:          "lorem ipsum",
			log:           strings.Repeat("lorem ipsum", common.DefaultReaderBufferSize),
			expectedCount: common.DefaultReaderBufferSize,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			line := tc.log
			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse(fmt.Sprintf(`echo "%s"`, line))
			})
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			outBuffer := new(bytes.Buffer)
			err := build.Run(&common.Config{}, &common.Trace{Writer: outBuffer})
			require.NoError(t, err)

			assert.GreaterOrEqual(t, tc.expectedCount, countWord(t, timestampPattern.ReplaceAllString(outBuffer.String(), ""), tc.word))
		})
	}
}

func testKubernetesHugeScriptAndAfterScriptFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	getAfterScript := func(featureFlag bool, script ...string) spec.Step {
		as := spec.Step{
			Name: "after_script",
			Script: spec.StepScript{
				"echo $CI_JOB_STATUS",
			},
			Timeout:      3600,
			When:         spec.StepWhenAlways,
			AllowFailure: true,
		}

		if !featureFlag {
			as.Script = append(as.Script, "ls -l /scripts-0-0/*")
		}

		as.Script = append(as.Script, script...)

		return as
	}

	tests := map[string]struct {
		image       string
		shell       string
		getScript   func() spec.StepScript
		afterScript []string
		verifyFn    func(t *testing.T, out string)
	}{
		"bash normal script": {
			image: common.TestAlpineImage,
			shell: "bash",
			getScript: func() spec.StepScript {
				return []string{
					`echo "My normal string"`,
				}
			},
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "success")
				assert.Contains(t, out, "My normal string")
			},
		},
		"pwsh unicode script": {
			image: common.TestPwshImage,
			shell: "pwsh",
			getScript: func() spec.StepScript {
				return []string{
					"echo \"`“ `“ `” `” `„ ‘ ’ ‚ ‛ ‘ ’ ; < ( ) & ^ # [ ] { } ' < > | @ % „",
				}
			},
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "success")
				assert.Contains(t, out, "“ “ ” ” „ ‘ ’ ‚ ‛ ‘ ’ ; < ( ) & ^ # [ ] { } ' < > | @ %")
			},
		},
		"bash nested here string": {
			image:       common.TestAlpineImage,
			shell:       "bash",
			afterScript: []string{"cat ./print.sh"},
			getScript: func() spec.StepScript {
				return []string{
					`cat <<EOF > ./print.sh
#!/bin/bash
echo "My nested here-string"
EOF
`,
				}
			},
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "success")
				assert.Contains(t, out, "echo \"My nested here-string\"")
			},
		},
		"pwsh nested here-string": {
			image: common.TestPwshImage,
			shell: "pwsh",
			getScript: func() spec.StepScript {
				return []string{
					`echo @'
My nested here-string
echo @"
My nested nested here-string
“ “ ” ” „ ‘ ’ ‚ ‛ ‘ ’ ; < ( ) & ^ # [ ] { } ' < > | @ %
"@
'@`,
				}
			},
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "success")
				assert.Contains(t, out, "My nested here-string")
				assert.Contains(t, out, "My nested nested here-string")
				assert.Contains(t, out, "“ “ ” ” „ ‘ ’ ‚ ‛ ‘ ’ ; < ( ) & ^ # [ ] { } ' < > | @ %")
			},
		},
		"bash huge script": {
			image: common.TestAlpineImage,
			shell: "bash",
			getScript: func() spec.StepScript {
				s := strings.Repeat(
					"echo \"Lorem ipsum dolor sit amet, consectetur adipiscing elit\"\n",
					10*1024,
				)
				return strings.Split(s, "\n")
			},
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "success")
				assert.Contains(t, out, "Lorem ipsum dolor sit amet, consectetur adipiscing elit")
			},
		},
		"pwsh special script with special character": {
			image: common.TestPwshImage,
			shell: "pwsh",
			getScript: func() spec.StepScript {
				return []string{
					`& {$Calendar = Get-Date; If ($Calendar.Month -eq '0') {"This is wrong"} Else {echo "not happening" > test.txt}; ls; Get-Content test.txt;}`,
				}
			},
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "test.txt")
				assert.Contains(t, out, "not happening")
			},
		},
		"pwsh multiple instructions in the script": {
			image: common.TestPwshImage,
			shell: "pwsh",
			getScript: func() spec.StepScript {
				return []string{
					`$Calendar = Get-Date`,
					`If ($Calendar.Month -eq '0') {"This is wrong"} Else {echo "not happening" > test.txt}`,
					`ls`,
					`Get-Content test.txt`,
					`&{ echo "Display special characters () {} <> [] \ | ;"}`,
				}
			},
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "test.txt")
				assert.Contains(t, out, "not happening")
				assert.Contains(t, out, "Display special characters () {} <> [] \\ | ;")
			},
		},
		"pwsh instruction with arrays": {
			image: common.TestPwshImage,
			shell: "pwsh",
			getScript: func() spec.StepScript {
				return []string{
					`$data = @('Zero','One')`,
					`$data | % {"$PSItem"}`,
					`$data = @{two = "Two"; three = "Three"; }`,
					`$data.values | % {"$PSItem"}`,
				}
			},
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "Zero")
				assert.Contains(t, out, "One")
				assert.Contains(t, out, "Two")
				assert.Contains(t, out, "Three")
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse("echo \"Hello World\"")
			})

			build.Runner.RunnerSettings.Shell = tc.shell
			build.Job.Image.Name = tc.image
			build.Job.Steps[0].Script = append(
				build.Job.Steps[0].Script,
				tc.getScript()...,
			)
			build.Job.Steps = append(
				build.Job.Steps,
				getAfterScript(featureFlagValue, tc.afterScript...),
			)

			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			outBuffer := new(bytes.Buffer)
			err := build.Run(&common.Config{}, &common.Trace{Writer: outBuffer})
			require.NoError(t, err)

			if !featureFlagValue {
				assert.Contains(t, outBuffer.String(), "echo $CI_JOB_STATUS")
				assert.Contains(t, outBuffer.String(), "/scripts-0-0/step_script")
				assert.Contains(t, outBuffer.String(), "/scripts-0-0/after_script")
			}

			if tc.verifyFn != nil {
				tc.verifyFn(t, outBuffer.String())
			}
		})
	}
}

func testKubernetesCustomPodSpec(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	ctxTimeout := time.Minute
	client := getTestKubeClusterClient(t)

	init := func(t *testing.T, _ *common.Build, client *k8s.Clientset) {
		credentials, err := getSecrets(client, ciNamespace, "")
		require.NoError(t, err)
		configMaps, err := getConfigMaps(client, ciNamespace, "")
		require.NoError(t, err)

		assert.Empty(t, credentials)
		assert.Empty(t, configMaps)
	}

	tests := map[string]struct {
		podSpec  []common.KubernetesPodSpec
		verifyFn func(*testing.T, v1.Pod)
	}{
		"change hostname with custom podSpec": {
			podSpec: []common.KubernetesPodSpec{
				{
					Patch: `
[
	{
		"op": "add",
		"path": "/hostname",
		"value": "my-custom-hostname"
	}
]
`,
					PatchType: common.PatchTypeJSONPatchType,
				},
			},
			verifyFn: func(t *testing.T, pod v1.Pod) {
				assert.Equal(t, "my-custom-hostname", pod.Spec.Hostname)
			},
		},
		"update build container with resources limit through custom podSpec using strategic patch type": {
			podSpec: []common.KubernetesPodSpec{
				{
					Patch: `
containers:
- name: "build"
  securityContext:
    runAsUser: 1010
`,
					PatchType: common.PatchTypeStrategicMergePatchType,
				},
			},
			verifyFn: func(t *testing.T, pod v1.Pod) {
				var buildContainer v1.Container
				var user int64 = 1010

				for _, c := range pod.Spec.Containers {
					if c.Name == "build" {
						buildContainer = c
						break
					}
				}
				assert.Equal(t, user, *buildContainer.SecurityContext.RunAsUser)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				jobResponse, err := common.GetRemoteBuildResponse(
					"sleep 5000",
				)
				require.NoError(t, err)

				return jobResponse, nil
			})
			build.Runner.Kubernetes.PodSpec = tc.podSpec
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
			buildtest.SetBuildFeatureFlag(build, featureflags.UseAdvancedPodSpecConfiguration, true)

			init(t, build, client)

			deletedPodNameCh := make(chan string)
			defer buildtest.OnUserStage(build, func() {
				ctx, cancel := context.WithTimeout(t.Context(), ctxTimeout)
				defer cancel()
				pods, err := client.CoreV1().Pods(ciNamespace).List(
					ctx,
					metav1.ListOptions{
						LabelSelector: labels.Set(build.Runner.Kubernetes.PodLabels).String(),
					},
				)
				require.NoError(t, err)
				require.NotEmpty(t, pods.Items)
				pod := pods.Items[0]

				tc.verifyFn(t, pod)

				err = client.
					CoreV1().
					Pods(ciNamespace).
					Delete(ctx, pod.Name, metav1.DeleteOptions{
						PropagationPolicy: &kubernetes.PropagationPolicy,
					})
				require.NoError(t, err)

				deletedPodNameCh <- pod.Name
			})()

			err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			assert.Error(t, err)

			<-deletedPodNameCh
		})
	}
}

func testKubernetesFailingBuildForBashAndPwshFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		image string
		shell string
	}{
		"bash failing script": {
			image: common.TestAlpineImage,
			shell: "bash",
		},
		"pwsh failing script": {
			image: common.TestPwshImage,
			shell: "pwsh",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()
			if tc.shell == "pwsh" {
				t.Skip("TODO: Fix pwsh fails")
			}

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse("invalid_command")
			})

			build.Runner.RunnerSettings.Shell = tc.shell
			build.Job.Image.Name = tc.image

			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
			err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			require.Error(t, err)
		})
	}
}

func testKubernetesBuildFailFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err, "error")
	var buildError *common.BuildError
	require.ErrorAs(t, err, &buildError)
	assert.Contains(t, err.Error(), "command terminated with exit code 1")
	assert.Equal(t, 1, buildError.ExitCode)
}

func testKubernetesBuildCancelFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	t.Skip("TODO: Flaky test https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/8628638315#L318")
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (spec.Job, error) {
		return spec.Job{}, nil
	})
	buildtest.RunBuildWithCancel(
		t,
		build.Runner,
		func(_ *testing.T, build *common.Build) {
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
		},
	)
}

func testKubernetesBuildLogLimitExceededFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (spec.Job, error) {
		return spec.Job{}, nil
	})
	buildtest.RunRemoteBuildWithJobOutputLimitExceeded(
		t,
		build.Runner,
		func(_ *testing.T, build *common.Build) {
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
		},
	)
}

func testKubernetesBuildMaskingFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (spec.Job, error) {
		return spec.Job{}, nil
	})
	buildtest.RunBuildWithMasking(
		t,
		build.Runner,
		func(_ *testing.T, build *common.Build) {
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
		},
	)
}

func testKubernetesCustomClonePathFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	const defaultGitClonePath = "$CI_BUILDS_DIR/go/src/gitlab.com/gitlab-org/repo"
	someTrue, someFalse := true, false

	tests := map[string]struct {
		customBuildDirConfig  common.CustomBuildDir
		buildsDirConfig       string
		gitClonePathOverwrite string
		expectedErr           string
	}{
		"defaults": {},
		"with builds_dir": {
			buildsDirConfig: "/foo/bar/baz",
		},
		"path has to be within CI_BUILDS_DIR": {
			gitClonePathOverwrite: "/nope/go/src/gitlab.com/gitlab-org/repo",
			expectedErr:           `prepare build and shell: the GIT_CLONE_PATH="/nope/go/src/gitlab.com/gitlab-org/repo" has to be within "/builds"`,
		},
		"custom_build_dir explicitly disabled": {
			customBuildDirConfig: common.CustomBuildDir{Enabled: &someFalse},
			expectedErr:          "prepare build and shell: setting GIT_CLONE_PATH is not allowed, enable `custom_build_dir` feature",
		},
		"custom_build_dir explicitly enabled": {
			customBuildDirConfig: common.CustomBuildDir{Enabled: &someTrue},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse("ls -la " + defaultGitClonePath)
			})
			build.Runner.CustomBuildDir = test.customBuildDirConfig
			build.Runner.BuildsDir += test.buildsDirConfig
			build.Variables = append(build.Variables,
				spec.Variable{Key: "GIT_CLONE_PATH", Value: cmp.Or(test.gitClonePathOverwrite, defaultGitClonePath)},
			)

			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			err := buildtest.RunBuild(t, build)
			if test.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedErr)
				var buildErr *common.BuildError
				assert.ErrorAs(t, err, &buildErr)
			}
		})
	}
}

func testKubernetesNoRootImageFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuildWithDumpedVariables)
	build.Image.Name = common.TestAlpineNoRootImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func testKubernetesMissingImageFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Image.Name = "some/non-existing/image"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.ErrorIs(t, err, &common.BuildError{FailureReason: common.ImagePullFailure})
	assert.Contains(t, err.Error(), "image pull failed")
}

func testKubernetesMissingTagFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Image.Name = "docker:missing-tag"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.ErrorIs(t, err, &common.BuildError{FailureReason: common.ImagePullFailure})
	assert.Contains(t, err.Error(), "image pull failed")
}

func testKubernetesFailingToPullImageTwiceFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Image.Name = "some/non-existing/image"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := runMultiPullPolicyBuild(t, build)

	var imagePullErr *pull.ImagePullError
	require.ErrorAs(t, err, &imagePullErr)
	assert.Equal(t, build.Image.Name, imagePullErr.Image)
}

func testKubernetesFailingToPullSvcImageTwiceFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Services = spec.Services{
		{
			Name: "some/non-existing/image",
		},
	}
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := runMultiPullPolicyBuild(t, build)

	var imagePullErr *pull.ImagePullError
	require.ErrorAs(t, err, &imagePullErr)
	assert.Equal(t, build.Services[0].Name, imagePullErr.Image)
}

func testKubernetesFailingToPullHelperTwiceFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Runner.Kubernetes.HelperImage = "some/non-existing/image"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := runMultiPullPolicyBuild(t, build)

	var imagePullErr *pull.ImagePullError
	require.ErrorAs(t, err, &imagePullErr)
	assert.Equal(t, build.Runner.Kubernetes.HelperImage, imagePullErr.Image)
}

func testOverwriteNamespaceNotMatchFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (spec.Job, error) {
		return spec.Job{
			GitInfo: spec.GitInfo{
				Sha: "1234567890",
			},
			Image: spec.Image{
				Name: "test-image",
			},
			Variables: []spec.Variable{
				{Key: kubernetes.NamespaceOverwriteVariableName, Value: "namespace"},
			},
		}, nil
	})
	build.Runner.Kubernetes.NamespaceOverwriteAllowed = "^not_a_match$"
	build.SystemInterrupt = make(chan os.Signal, 1)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func testOverwriteServiceAccountNotMatchFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (spec.Job, error) {
		return spec.Job{
			GitInfo: spec.GitInfo{
				Sha: "1234567890",
			},
			Image: spec.Image{
				Name: "test-image",
			},
			Variables: []spec.Variable{
				{Key: kubernetes.ServiceAccountOverwriteVariableName, Value: "service-account"},
			},
		}, nil
	})
	build.Runner.Kubernetes.ServiceAccountOverwriteAllowed = "^not_a_match$"
	build.SystemInterrupt = make(chan os.Signal, 1)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func testInteractiveTerminalFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	if os.Getenv("GITLAB_CI") == "true" {
		t.Skip("Skipping inside of GitLab CI check https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26421")
	}

	client := getTestKubeClusterClient(t)
	secrets, err := client.
		CoreV1().
		Secrets(ciNamespace).
		List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)

	build := getTestBuild(t, func() (spec.Job, error) {
		return common.GetRemoteBuildResponse("sleep 5")
	})
	build.Image.Name = "docker:git"
	build.Runner.Kubernetes.BearerToken = string(secrets.Items[0].Data["token"])
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	sess, err := session.NewSession(nil)
	build.Session = sess

	outBuffer := bytes.NewBuffer(nil)
	outCh := make(chan string)

	go func() {
		err = build.Run(
			&common.Config{
				SessionServer: common.SessionServer{
					SessionTimeout: 2,
				},
			},
			&common.Trace{Writer: outBuffer},
		)
		require.NoError(t, err)

		outCh <- outBuffer.String()
	}()

	srv := httptest.NewServer(build.Session.Handler())
	defer srv.Close()

	u := url.URL{
		Scheme: "ws",
		Host:   srv.Listener.Addr().String(),
		Path:   build.Session.Endpoint + "/exec",
	}
	headers := http.Header{
		"Authorization": []string{build.Session.Token},
	}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), headers)
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}
	}()
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusSwitchingProtocols)

	out := <-outCh
	t.Log(out)

	assert.Contains(t, out, "Terminal is connected, will time out in 2s...")
}

func testKubernetesReplaceEnvFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")
	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = "$IMAGE:$VERSION"
	build.Job.Variables = append(
		build.Job.Variables,
		spec.Variable{Key: "IMAGE", Value: "alpine"},
		spec.Variable{Key: "VERSION", Value: "latest"},
	)
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
	out, err := buildtest.RunBuildReturningOutput(t, build)
	require.NoError(t, err)
	assert.Contains(t, out, "alpine:latest")
}

func testKubernetesReplaceMissingEnvVarFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")
	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = "alpine:$NOT_EXISTING_VARIABLE"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image pull failed: Failed to apply default image tag \"alpine:\"")
}

func testBuildsDirDefaultVolumeFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = common.TestDockerGitImage
	build.Runner.BuildsDir = "/path/to/builds/dir"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)

	assert.Equal(t, "/path/to/builds/dir/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test", build.BuildDir)
}

func testBuildsDirVolumeMountEmptyDirFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		emptyDir   common.KubernetesEmptyDir
		hasWarning bool
	}{
		"emptyDir with empty size": {
			emptyDir: common.KubernetesEmptyDir{
				Name:      "repo",
				MountPath: "/path/to/builds/dir",
				Medium:    "Memory",
			},
		},
		"emptyDir with untrimed empty size": {
			emptyDir: common.KubernetesEmptyDir{
				Name:      "repo",
				MountPath: "/path/to/builds/dir",
				Medium:    "Memory",
				SizeLimit: "  ",
			},
		},
		"emptyDir with valid size": {
			emptyDir: common.KubernetesEmptyDir{
				Name:      "repo",
				MountPath: "/path/to/builds/dir",
				Medium:    "Memory",
				SizeLimit: "1G",
			},
		},
		"emptyDir with invalid emptyDir": {
			emptyDir: common.KubernetesEmptyDir{
				Name:      "repo",
				MountPath: "/path/to/builds/dir",
				Medium:    "Memory",
				SizeLimit: "invalid",
			},
			hasWarning: true,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
			build.Image.Name = common.TestDockerGitImage
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
			build.Runner.BuildsDir = "/path/to/builds/dir"
			build.Runner.Kubernetes.Volumes = common.KubernetesVolumes{
				EmptyDirs: []common.KubernetesEmptyDir{
					tc.emptyDir,
				},
			}

			outBuffer := bytes.NewBuffer(nil)
			err := build.Run(&common.Config{}, &common.Trace{Writer: outBuffer})
			assert.NoError(t, err)

			if tc.hasWarning {
				assert.Contains(t, outBuffer.String(), "invalid limit quantity")
			}

			assert.Equal(t, "/path/to/builds/dir/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test", build.BuildDir)
		})
	}
}

func testBuildsDirVolumeMountHostPathFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
	build.Runner.Kubernetes.Volumes = common.KubernetesVolumes{
		HostPaths: []common.KubernetesHostPath{
			{
				Name:      "repo-host",
				MountPath: "/builds",
				HostPath:  "/tmp/builds",
			},
		},
	}

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)

	allVariables := build.GetAllVariables()

	assert.Equal(t, fmt.Sprintf("/builds/%s/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test", allVariables.Value("CI_CONCURRENT_ID")), build.BuildDir)
}

// testKubernetesGarbageCollection tests the deletion of resources via garbage collector once the owning pod is deleted
func testKubernetesGarbageCollection(t *testing.T, featureFlagName string, featureFlagValue bool) {
	t.Skip("TODO: Fix flaky test expected error not always matches https://gitlab.com/gitlab-org/gitlab-runner/-/jobs/8543529098#L226")
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	ctxTimeout := time.Minute
	client := getTestKubeClusterClient(t)

	validateResourcesCreated := func(
		t *testing.T,
		client *k8s.Clientset,
		featureFlagValue bool,
		namespace string,
		podName string,
	) {
		credentials, err := getSecrets(client, namespace, podName)
		require.NoError(t, err)
		configMaps, err := getConfigMaps(client, namespace, podName)
		require.NoError(t, err)

		assert.NotEmpty(t, credentials)
		assert.Empty(t, configMaps)
	}

	validateResourcesDeleted := func(t *testing.T, client *k8s.Clientset, namespace string, podName string) {
		// The deletion propagation policy has been shifted to Background
		// in the MR https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4339
		// This means the dependant will be deleted in background and not immediately
		// A retry is needed to ensure the dependant is deleted at some point
		creds, err := retry.NewValue(retry.New(), func() ([]v1.Secret, error) {
			credentials, err := getSecrets(client, namespace, podName)
			require.NoError(t, err)

			if len(credentials) > 0 {
				return credentials, errors.New("secrets still exist")
			}

			return credentials, nil
		}).Run()
		require.NoError(t, err)

		cfgMaps, err := retry.NewValue(retry.New(), func() ([]v1.ConfigMap, error) {
			configMaps, err := getConfigMaps(client, namespace, podName)
			require.NoError(t, err)

			if len(configMaps) > 0 {
				return configMaps, errors.New("configMaps still exist")
			}

			return configMaps, nil
		}).Run()
		require.NoError(t, err)

		assert.Empty(t, creds)
		assert.Empty(t, cfgMaps)
	}

	tests := map[string]struct {
		init     func(t *testing.T, build *common.Build, client *k8s.Clientset)
		finalize func(t *testing.T, client *k8s.Clientset)
	}{
		"pod deletion during build step": {},
		"pod deletion during prepare stage in custom namespace": {
			init: func(t *testing.T, build *common.Build, client *k8s.Clientset) {
				credentials, err := getSecrets(client, ciNamespace, "")
				require.NoError(t, err)
				configMaps, err := getConfigMaps(client, ciNamespace, "")
				require.NoError(t, err)

				assert.Empty(t, credentials)
				assert.Empty(t, configMaps)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				jobResponse, err := common.GetRemoteBuildResponse(
					"sleep 5000",
				)
				require.NoError(t, err)

				jobResponse.Credentials = []spec.Credentials{
					{
						Type:     "registry",
						URL:      "http://example.com",
						Username: "user",
						Password: "password",
					},
				}

				return jobResponse, nil
			})
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			if tc.init != nil {
				tc.init(t, build, client)
			}

			deletedPodNameCh := make(chan string)
			defer buildtest.OnUserStage(build, func() {
				ctx, cancel := context.WithTimeout(t.Context(), ctxTimeout)
				defer cancel()
				pods, err := client.CoreV1().Pods(ciNamespace).List(
					ctx,
					metav1.ListOptions{
						LabelSelector: labels.Set(build.Runner.Kubernetes.PodLabels).String(),
					},
				)
				require.NoError(t, err)
				require.NotEmpty(t, pods.Items)
				pod := pods.Items[0]

				validateResourcesCreated(t, client, featureFlagValue, ciNamespace, pod.Name)

				err = client.
					CoreV1().
					Pods(ciNamespace).
					Delete(ctx, pod.Name, metav1.DeleteOptions{
						PropagationPolicy: &kubernetes.PropagationPolicy,
					})
				require.NoError(t, err)

				deletedPodNameCh <- pod.Name
			})()

			out, err := buildtest.RunBuildReturningOutput(t, build)

			podName := <-deletedPodNameCh

			if !featureFlagValue {
				assert.True(t, kubernetes.IsKubernetesPodNotFoundError(err), "expected err NotFound, but got %T", err)
				assert.Contains(
					t,
					out,
					"ERROR: Job failed (system failure):",
				)
			} else {
				assert.Errorf(t, err, "command terminated with exit code 137")
			}
			validateResourcesDeleted(t, client, ciNamespace, podName)
		})
	}
}

func testKubernetesNamespaceIsolation(t *testing.T, featureFlagName string, featureFlagValue bool) {
	t.Skip("TODO: skipping namespace isolation test to add metadata for better cleanup")
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	jobId := rand.Int()
	expectedNamespace := fmt.Sprintf("ci-job-%d", jobId)

	ctxTimeout := time.Minute
	client := getTestKubeClusterClient(t)

	validateNamespaceDeleted := func(t *testing.T, client *k8s.Clientset, namespace string) {
		ctx, cancel := context.WithTimeout(t.Context(), ctxTimeout)
		defer cancel()

		ns, err := client.CoreV1().Namespaces().Get(
			ctx,
			namespace,
			metav1.GetOptions{},
		)

		require.NoError(t, err)
		assert.Equal(t, v1.NamespaceTerminating, ns.Status.Phase)
	}

	tests := map[string]struct {
		init     func(t *testing.T, build *common.Build, client *k8s.Clientset, namespace string)
		finalize func(t *testing.T, client *k8s.Clientset, namespace string)
	}{
		"test with default values": {
			init: func(t *testing.T, build *common.Build, client *k8s.Clientset, namespace string) {
				credentials, err := getSecrets(client, namespace, "")
				require.NoError(t, err)
				configMaps, err := getConfigMaps(client, namespace, "")
				require.NoError(t, err)

				assert.Empty(t, credentials)
				assert.Empty(t, configMaps)
			},
			finalize: func(t *testing.T, client *k8s.Clientset, namespace string) {
				_, err := newNamespaceManager(client, deleteNamespace, namespace).Run()
				require.NoError(t, err)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				jobResponse, err := common.GetRemoteBuildResponse(
					"sleep 5000",
				)
				require.NoError(t, err)

				jobResponse.Credentials = []spec.Credentials{
					{
						Type:     "registry",
						URL:      "http://example.com",
						Username: "user",
						Password: "password",
					},
				}

				return jobResponse, nil
			})
			build.ID = int64(jobId)
			build.Runner.Kubernetes.Namespace = "default"
			build.Runner.Kubernetes.NamespacePerJob = true
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			if tc.init != nil {
				tc.init(t, build, client, expectedNamespace)
			}

			deletedPodNameCh := make(chan string)
			defer buildtest.OnUserStage(build, func() {
				ctx, cancel := context.WithTimeout(t.Context(), ctxTimeout)
				defer cancel()
				pods, err := client.CoreV1().Pods(expectedNamespace).List(
					ctx,
					metav1.ListOptions{
						LabelSelector: labels.Set(build.Runner.Kubernetes.PodLabels).String(),
					},
				)
				require.NoError(t, err)
				require.NotEmpty(t, pods.Items)
				pod := pods.Items[0]

				assert.Equal(t, pod.GetNamespace(), expectedNamespace)

				err = client.
					CoreV1().
					Pods(expectedNamespace).
					Delete(ctx, pod.Name, metav1.DeleteOptions{
						PropagationPolicy: &kubernetes.PropagationPolicy,
					})
				require.NoError(t, err)

				deletedPodNameCh <- pod.Name
			})()

			out, err := buildtest.RunBuildReturningOutput(t, build)

			<-deletedPodNameCh

			if !featureFlagValue {
				assert.Contains(
					t,
					out,
					"ERROR: Job failed (system failure):",
				)
			} else {
				assert.Errorf(t, err, "command terminated with exit code 137")
			}

			validateNamespaceDeleted(t, client, expectedNamespace)

			if tc.finalize != nil {
				tc.finalize(t, client, expectedNamespace)
			}
		})
	}
}

func testKubernetesPublicInternalVariables(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	ctxTimeout := time.Minute
	client := getTestKubeClusterClient(t)

	containsVerifyFn := func(t *testing.T, v spec.Variable, envNames []string, envValues []string) {
		assert.Contains(t, envNames, v.Key)
		assert.Contains(t, envValues, v.Value)
	}

	tests := map[string]struct {
		variable spec.Variable
		verifyFn func(*testing.T, spec.Variable, []string, []string)
	}{
		"internal variable": {
			variable: spec.Variable{
				Key:      "my_internal_variable",
				Value:    "my internal variable",
				Internal: true,
			},
			verifyFn: containsVerifyFn,
		},
		"public variable": {
			variable: spec.Variable{
				Key:    "my_public_variable",
				Value:  "my public variable",
				Public: true,
			},
			verifyFn: containsVerifyFn,
		},
		"regular variable": {
			variable: spec.Variable{
				Key:   "my_regular_variable",
				Value: "my regular variable",
			},
			verifyFn: func(t *testing.T, v spec.Variable, envNames []string, envValues []string) {
				assert.NotContains(t, envNames, v.Key)
				assert.NotContains(t, envValues, v.Value)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				jobResponse, err := common.GetRemoteBuildResponse(
					"sleep 15000",
				)
				require.NoError(t, err)

				jobResponse.Credentials = []spec.Credentials{
					{
						Type:     "registry",
						URL:      "http://example.com",
						Username: "user",
						Password: "password",
					},
				}

				jobResponse.Variables = []spec.Variable{
					tc.variable,
				}

				return jobResponse, nil
			})
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			deletedPodNameCh := make(chan string)
			defer buildtest.OnUserStage(build, func() {
				ctx, cancel := context.WithTimeout(t.Context(), ctxTimeout)
				defer cancel()
				pods, err := client.CoreV1().Pods(ciNamespace).List(
					ctx,
					metav1.ListOptions{
						LabelSelector: labels.Set(build.Runner.Kubernetes.PodLabels).String(),
					},
				)
				require.NoError(t, err)
				require.NotEmpty(t, pods.Items)
				pod := pods.Items[0]

				var c *v1.Container
				for _, container := range pod.Spec.Containers {
					if container.Name == "build" {
						c = &container
						break
					}
				}
				require.NotNil(t, c)

				envNames := make([]string, 0)
				envValues := make([]string, 0)
				for _, env := range c.Env {
					envNames = append(envNames, env.Name)
					envValues = append(envValues, env.Value)
				}
				require.NotEmpty(t, envNames)
				require.NotEmpty(t, envValues)

				tc.verifyFn(t, tc.variable, envNames, envValues)

				err = client.
					CoreV1().
					Pods(ciNamespace).
					Delete(ctx, pod.Name, metav1.DeleteOptions{
						PropagationPolicy: &kubernetes.PropagationPolicy,
					})
				require.NoError(t, err)

				deletedPodNameCh <- pod.Name
			})()

			_, err := buildtest.RunBuildReturningOutput(t, build)

			<-deletedPodNameCh

			assert.Errorf(t, err, "command terminated with exit code 137")
		})
	}
}

func testKubernetesWaitResources(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	secretName := func() string {
		return fmt.Sprintf("my-secret-1-%d", rand.Uint64())
	}
	saName := func() string {
		return fmt.Sprintf("my-serviceaccount-%d", rand.Uint64())
	}
	client := getTestKubeClusterClient(t)

	tests := map[string]struct {
		init             func(t *testing.T, secretName, saName string, build *common.Build, client *k8s.Clientset)
		finalize         func(t *testing.T, secretName, saName string, client *k8s.Clientset)
		checkMaxAttempts int
		imagePullSecret  []string
		serviceAccount   string
		expectedErr      bool
	}{
		"no resources available": {
			checkMaxAttempts: 1,
			imagePullSecret:  []string{secretName()},
			serviceAccount:   saName(),
			expectedErr:      true,
		},
		"only serviceaccount set": {
			serviceAccount: kubernetes.DefaultResourceIdentifier,
		},
		"secret not set but serviceaccount available": {
			checkMaxAttempts: 1,
			imagePullSecret:  []string{secretName()},
			serviceAccount:   kubernetes.DefaultResourceIdentifier,
			expectedErr:      true,
		},
		"secret made available while waiting for resources": {
			checkMaxAttempts: 10,
			imagePullSecret:  []string{secretName()},
			init: func(t *testing.T, secretName, saName string, build *common.Build, client *k8s.Clientset) {
				time.Sleep(time.Second * 3)
				ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
				defer cancel()

				s := &v1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind: "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: secretName,
					},
					Data: map[string][]byte{},
				}

				_, err := kubernetes.CreateTestKubernetesResource(ctx, client, ciNamespace, s)
				require.NoError(t, err)
			},
			finalize: func(t *testing.T, secretName, saName string, client *k8s.Clientset) {
				ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
				defer cancel()

				err := client.
					CoreV1().
					Secrets(ciNamespace).
					Delete(ctx, secretName, metav1.DeleteOptions{})
				require.NoError(t, err)
			},
		},
		"serviceaccount made available while waiting for resources": {
			init: func(t *testing.T, secretName, saName string, build *common.Build, client *k8s.Clientset) {
				time.Sleep(time.Second * 3)
				ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
				defer cancel()

				sa := &v1.ServiceAccount{
					TypeMeta: metav1.TypeMeta{
						Kind: "ServiceAccount",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: saName,
					},
				}

				_, err := kubernetes.CreateTestKubernetesResource(ctx, client, ciNamespace, sa)
				require.NoError(t, err)
			},
			checkMaxAttempts: 10,
			serviceAccount:   saName(),
			finalize: func(t *testing.T, secretName, saName string, client *k8s.Clientset) {
				ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
				defer cancel()

				err := client.
					CoreV1().
					ServiceAccounts(ciNamespace).
					Delete(ctx, saName, metav1.DeleteOptions{})
				require.NoError(t, err)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				jobResponse, err := common.GetRemoteBuildResponse(
					"echo Hello World",
				)
				require.NoError(t, err)

				jobResponse.Credentials = []spec.Credentials{
					{
						Type:     "registry",
						URL:      "http://example.com",
						Username: "user",
						Password: "password",
					},
				}

				return jobResponse, nil
			})
			build.Runner.Kubernetes.ResourceAvailabilityCheckMaxAttempts = tc.checkMaxAttempts
			build.Runner.Kubernetes.ImagePullSecrets = tc.imagePullSecret
			build.Runner.Kubernetes.ServiceAccount = tc.serviceAccount
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			var secretName string
			if len(tc.imagePullSecret) > 0 {
				secretName = tc.imagePullSecret[0]
			}

			saName := tc.serviceAccount

			if tc.init != nil {
				go tc.init(t, secretName, saName, build, client)
			}

			out, err := buildtest.RunBuildReturningOutput(t, build)

			if tc.finalize != nil {
				tc.finalize(t, secretName, saName, client)
			}

			if tc.expectedErr {
				assert.Error(t, err, "checking ImagePullSecret: couldn't find ImagePullSecret or ServiceAccount")
				return
			}

			assert.NoError(t, err)
			assert.Contains(t, out, "Hello World")
		})
	}
}

func testKubernetesClusterWarningEvent(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		image           string
		retrieveWarning bool
		verifyFn        func(*testing.T, string, error)
	}{
		"invalid image": {
			image:           "alpine:invalid-tag",
			retrieveWarning: true,
			verifyFn: func(t *testing.T, out string, err error) {
				assert.Error(t, err)
				assert.Contains(t, out, "WARNING: Event retrieved from the cluster:")
			},
		},
		"invalid image with configuration disabled": {
			image: "alpine:invalid-tag",
			verifyFn: func(t *testing.T, out string, err error) {
				assert.Error(t, err)
				assert.NotContains(t, out, "WARNING: Event retrieved from the cluster:")
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				jobResponse, err := common.GetRemoteBuildResponse(
					"echo Hello World",
				)
				require.NoError(t, err)

				return jobResponse, nil
			})
			build.Runner.Kubernetes.Image = tc.image
			build.Runner.Kubernetes.PrintPodWarningEvents = &tc.retrieveWarning
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
			build.Runner.Kubernetes.HelperImage = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest"

			out, err := buildtest.RunBuildReturningOutput(t, build)
			tc.verifyFn(t, out, err)
		})
	}
}

// TestLogDeletionAttach tests the outcome when the log files are all deleted
func TestLogDeletionAttach(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	t.Skip("Log deletion test temporary skipped: issue https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27755")

	tests := []struct {
		stage            string
		outputAssertions func(t *testing.T, out string, pod string)
	}{
		{
			stage: "step_", // Any script the user defined
			outputAssertions: func(t *testing.T, out string, pod string) {
				assert.Contains(
					t,
					out,
					"ERROR: Job failed: command terminated with exit code 100",
				)
			},
		},
		{
			stage: string(common.BuildStagePrepare),
			outputAssertions: func(t *testing.T, out string, pod string) {
				assert.Contains(
					t,
					out,
					"ERROR: Job failed: command terminated with exit code 100",
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse(
					"sleep 5000",
				)
			})
			buildtest.SetBuildFeatureFlag(build, featureflags.UseLegacyKubernetesExecutionStrategy, false)

			deletedPodNameCh := make(chan string)
			defer buildtest.OnUserStage(build, func() {
				client := getTestKubeClusterClient(t)
				pods, err := client.
					CoreV1().
					Pods(ciNamespace).
					List(t.Context(), metav1.ListOptions{
						LabelSelector: labels.Set(build.Runner.Kubernetes.PodLabels).String(),
					})
				require.NoError(t, err)
				require.NotEmpty(t, pods.Items)
				pod := pods.Items[0]
				config, err := kubernetes.GetKubeClientConfig(new(common.KubernetesConfig))
				require.NoError(t, err)
				logsPath := fmt.Sprintf("/logs-%d-%d", build.JobInfo.ProjectID, build.Job.ID)
				opts := kubernetes.ExecOptions{
					Namespace:  pod.Namespace,
					PodName:    pod.Name,
					KubeClient: client,
					Stdin:      true,
					In:         strings.NewReader(fmt.Sprintf("rm -rf %s/*", logsPath)),
					Out:        io.Discard,
					Command:    []string{"/bin/sh"},
					Config:     config,
					Executor:   &kubernetes.DefaultRemoteExecutor{},
				}
				err = opts.Run()
				require.NoError(t, err)

				deletedPodNameCh <- pod.Name
			})()

			out, err := buildtest.RunBuildReturningOutput(t, build)
			require.Error(t, err)
			assert.True(t, err != nil, "No error returned")

			tt.outputAssertions(t, out, <-deletedPodNameCh)
		})
	}
}

// This test reproduces the bug reported in https://gitlab.com/gitlab-org/gitlab-runner/issues/2583
// It checks that config overwrites are not persisted into shared state, and thus don't leak across executor instances.
func TestPrepareIssue2583(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	configuredNamespace := "configured-namespace"
	configuredServiceAccount := "configured-service-account"

	overwriteNamespace := ciNamespace
	overwriteServiceAccount := "some-other-service-account"

	build := getTestBuild(t, func() (spec.Job, error) {
		return spec.Job{
			Variables: []spec.Variable{
				{Key: kubernetes.NamespaceOverwriteVariableName, Value: overwriteNamespace},
				{Key: kubernetes.ServiceAccountOverwriteVariableName, Value: overwriteServiceAccount},
			},
		}, nil
	})
	build.Runner = &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Executor: common.ExecutorKubernetes,
			Kubernetes: &common.KubernetesConfig{
				Image:                          "an/image:latest",
				Namespace:                      configuredNamespace,
				NamespaceOverwriteAllowed:      ".*",
				ServiceAccount:                 configuredServiceAccount,
				ServiceAccountOverwriteAllowed: ".*",
			},
		},
	}

	e := common.NewExecutor(common.ExecutorKubernetes)

	mockTrace := buildlogger.NewMockTrace(t)
	mockTrace.EXPECT().IsStdout().Return(true).Once()
	mockTrace.EXPECT().Write(mock.Anything).Return(0, nil)

	// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
	prepareOptions := common.ExecutorPrepareOptions{
		Config:      build.Runner,
		Build:       build,
		Context:     t.Context(),
		BuildLogger: buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{}),
	}

	err := e.Prepare(prepareOptions)
	assert.NoError(t, err)
	assert.Equal(t, configuredNamespace, build.Runner.Kubernetes.Namespace)
	assert.Equal(t, configuredServiceAccount, build.Runner.Kubernetes.ServiceAccount)
}

func testDeletedPodSystemFailureDuringExecution(t *testing.T, ff string, ffValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	type terminator = func(client k8s.Interface, podName string) error

	deletePod := func(client k8s.Interface, podName string, delOpts metav1.DeleteOptions) error {
		return client.CoreV1().Pods(ciNamespace).Delete(t.Context(), podName, delOpts)
	}
	deletePodGracefully := func(client k8s.Interface, podName string) error {
		return deletePod(client, podName, metav1.DeleteOptions{})
	}
	deletePodNow := func(client k8s.Interface, podName string) error {
		return deletePod(client, podName, metav1.DeleteOptions{GracePeriodSeconds: common.Int64Ptr(0)})
	}
	evictPod := func(client k8s.Interface, podName string, delOpts metav1.DeleteOptions) error {
		return client.CoreV1().Pods(ciNamespace).EvictV1(t.Context(), &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name: podName,
			},
			DeleteOptions: &delOpts,
		})
	}
	evictPodGracefully := func(client k8s.Interface, podName string) error {
		return evictPod(client, podName, metav1.DeleteOptions{})
	}
	evictPodNow := func(client k8s.Interface, podName string) error {
		return evictPod(client, podName, metav1.DeleteOptions{GracePeriodSeconds: common.Int64Ptr(0)})
	}

	containsOneOf := func(heystack string, needles ...string) bool {
		for _, needle := range needles {
			if strings.Contains(heystack, needle) {
				return true
			}
		}

		return false
	}

	// Currently, with the introduction of the pod watcher, this is a bit racy in regards which actual error we get.
	// The pod watcher's informer is running concurrently to everything else that might produce an error (on-demand fetching or
	// polling of the pod's current state, e.g. when execing into or attaching to the pod).
	//
	// Therefore, this asserts
	// - that the error we observe is either one from the pod watcher or from the other checks
	// - regardless of which one it is, it results in a system failure
	//
	// If/once we only rely on the pod watcher and therefore the informer to tell us about the state of the pod instead of
	// doing on-demand checks / polling, this would go away and we'd receive a distinct error, and don't have to check if
	// it's either one or the other.
	assertSystemFailure := func(t *testing.T, err error, out string, errMsgs ...string) {
		t.Helper()
		assert.Contains(t, out, "ERROR: Job failed (system failure):")
		assert.True(t, containsOneOf(err.Error(), errMsgs...), "expected the error to contain one of %q, but didn't", errMsgs)
		assert.True(t, containsOneOf(out, errMsgs...), "expected the output to contain one of %q, but didn't", errMsgs)
	}

	tests := []struct {
		stage            string
		terminators      map[string]terminator
		outputAssertions func(t *testing.T, err error, out string, pod string)
	}{
		{
			stage: "step_", // Any script the user defined
			terminators: map[string]terminator{
				"delete gracefully": deletePodGracefully,
				"delete now":        deletePodNow,
			},
			outputAssertions: func(t *testing.T, err error, out string, pod string) {
				assertSystemFailure(t, err, out,
					fmt.Sprintf("pod %q is being deleted", ciNamespace+"/"+pod),
					fmt.Sprintf("pods %q not found", pod),
				)
			},
		},
		{
			stage: "step_", // Any script the user defined
			terminators: map[string]terminator{
				"evict gracefully": evictPodGracefully,
				"evict now":        evictPodNow,
			},
			outputAssertions: func(t *testing.T, err error, out string, pod string) {
				assertSystemFailure(t, err, out,
					fmt.Sprintf("pod %q is disrupted", ciNamespace+"/"+pod),
					fmt.Sprintf("pods %q not found", pod),
				)
			},
		},
		{
			stage: string(common.BuildStagePrepare),
			terminators: map[string]terminator{
				"delete gracefully": deletePodGracefully,
				"delete now":        deletePodNow,
				"evict gracefully":  evictPodGracefully,
				"evict now":         evictPodNow,
			},
			outputAssertions: func(t *testing.T, err error, out string, pod string) {
				assert.True(t, kubernetes.IsKubernetesPodNotFoundError(err), "expected err NotFound, but got %T", err)
				assert.Contains(t, out, "ERROR: Job failed (system failure):")
				assert.Contains(t, out, fmt.Sprintf("pods %q not found", pod))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			t.Parallel()

			for name, terminator := range tt.terminators {
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					ctx := t.Context()

					build := getTestBuild(t, common.GetRemoteLongRunningBuild)

					buildtest.SetBuildFeatureFlag(build, ff, ffValue)

					client := getTestKubeClusterClient(t)

					createdPodNameCh := make(chan string)
					deletedPodNameCh := make(chan string)

					go func() {
						watcher, err := client.CoreV1().Pods(ciNamespace).Watch(ctx, metav1.ListOptions{
							LabelSelector: labels.Set(build.Runner.Kubernetes.PodLabels).String(),
						})
						require.NoError(t, err, "setting up the pod watch")
						defer watcher.Stop()
						for event := range watcher.ResultChan() {
							if pod, ok := event.Object.(*v1.Pod); ok && event.Type == watch.Added {
								createdPodNameCh <- pod.GetName()
								break
							}
						}
					}()

					defer buildtest.OnStage(build, tt.stage, func() {
						podName := <-createdPodNameCh
						err := terminator(client, podName)
						require.NoError(t, err)
						deletedPodNameCh <- podName
					})()

					out, err := buildtest.RunBuildReturningOutput(t, build)

					tt.outputAssertions(t, err, out, <-deletedPodNameCh)
				})
			}
		})
	}
}

func testKubernetesWithNonRootSecurityContext(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (spec.Job, error) {
		return common.GetRemoteBuildResponse("id")
	})
	build.Image.Name = common.TestAlpineNoRootImage

	runAsNonRoot := true
	runAsUser := int64(1895034)
	build.Runner.Kubernetes.PodSecurityContext = common.KubernetesPodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
	}

	// We override the home directory, else we get this error from the git run:
	// 	```
	// 	Fetching changes...
	// 	error: could not lock config file //.gitconfig: Permission denied
	// 	ERROR: Job failed: command terminated with exit code 1
	// 	```
	build.Variables = append(build.Variables, spec.Variable{
		Key:   "HOME",
		Value: "/dev/shm",
	})

	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.Contains(t, out, fmt.Sprintf("uid=%d gid=0(root)", runAsUser))
}

func testKubernetesBashFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := []struct {
		script              string
		expectedContent     string
		expectedErrExitCode int
	}{
		{
			script:          "export hello=world; echo \"hello $hello\"",
			expectedContent: "hello world",
		},
		{
			script:              "return 129",
			expectedErrExitCode: 129,
		},
		{
			script:              "exit 128",
			expectedErrExitCode: 128,
		},
		{
			script:              "eco 'function error'",
			expectedErrExitCode: 127,
		},
		{
			script:              "{{}",
			expectedErrExitCode: 127,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			build.Image.Name = common.TestAlpineImage
			build.Runner.Shell = "bash"
			build.Job.Steps = spec.Steps{
				spec.Step{
					Name:   spec.StepNameScript,
					Script: []string{tc.script},
				},
			}

			out, err := buildtest.RunBuildReturningOutput(t, build)
			assert.Contains(t, out, tc.expectedContent)

			if tc.expectedErrExitCode != 0 {
				var buildError *common.BuildError
				if assert.ErrorAs(t, err, &buildError) {
					assert.Equal(t, tc.expectedErrExitCode, buildError.ExitCode)
				}
			}
		})
	}
}

func testKubernetesContainerHookFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		image           string
		shell           string
		lifecycleCfg    common.KubernetesContainerLifecyle
		steps           spec.Steps
		validateOutputs func(t *testing.T, out string, err error)
	}{
		"invalid hook configuration: more than one handler type": {
			lifecycleCfg: common.KubernetesContainerLifecyle{
				PreStop: &common.KubernetesLifecycleHandler{
					Exec: &common.KubernetesLifecycleExecAction{
						Command: []string{"touch", "/builds/postStart.txt"},
					},
					HTTPGet: &common.KubernetesLifecycleHTTPGet{
						Host: "localhost",
						Port: 8080,
					},
				},
			},
			validateOutputs: func(t *testing.T, out string, err error) {
				require.Error(t, err)
				assert.Contains(t, out, "ERROR: Job failed (system failure):")
				assert.Contains(t, out, "may not specify more than 1 handler type")
			},
		},
		"postStart exec hook bash": {
			steps: spec.Steps{
				spec.Step{
					Name: spec.StepNameScript,
					Script: []string{
						"ls -l /builds",
					},
				},
			},
			lifecycleCfg: common.KubernetesContainerLifecyle{
				PostStart: &common.KubernetesLifecycleHandler{
					Exec: &common.KubernetesLifecycleExecAction{
						Command: []string{"touch", "/builds/postStart.txt"},
					},
				},
			},
			validateOutputs: func(t *testing.T, out string, err error) {
				require.NoError(t, err)
				assert.Contains(t, out, "Job succeeded")
				assert.Contains(t, out, "postStart.txt")
			},
		},
		"postStart exec hook pwsh": {
			image: common.TestPwshImage,
			shell: shells.SNPwsh,
			steps: spec.Steps{
				spec.Step{
					Name: spec.StepNameScript,
					Script: []string{
						"Get-ChildItem /builds",
					},
				},
			},
			lifecycleCfg: common.KubernetesContainerLifecyle{
				PostStart: &common.KubernetesLifecycleHandler{
					Exec: &common.KubernetesLifecycleExecAction{
						Command: []string{"touch", "/builds/postStart.txt"},
					},
				},
			},
			validateOutputs: func(t *testing.T, out string, err error) {
				require.NoError(t, err)
				assert.Contains(t, out, "Job succeeded")
				assert.Contains(t, out, "postStart.txt")
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
			build.Image.Name = common.TestAlpineImage
			build.Runner.RunnerSettings.Kubernetes.ContainerLifecycle = tt.lifecycleCfg

			if tt.image != "" {
				build.Image.Name = tt.image
			}

			if tt.shell != "" {
				build.Runner.Shell = tt.shell
			}

			if tt.steps != nil {
				build.Job.Steps = tt.steps
			}

			out, err := buildtest.RunBuildReturningOutput(t, build)
			tt.validateOutputs(t, out, err)
		})
	}
}

func getTestBuildWithImage(t *testing.T, image string, getJobResponse func() (spec.Job, error)) *common.Build {
	jobResponse, err := getJobResponse()
	assert.NoError(t, err)

	podUUID, err := helpers.GenerateRandomUUID(8)
	require.NoError(t, err)

	tt := strings.Split(t.Name(), "/")
	slices.Reverse(tt)

	nodeSelector := map[string]string{}
	nodeTolerations := map[string]string{}
	if os.Getenv("GITLAB_CI") == "true" {
		nodeSelector["runner.gitlab.com/workload-type"] = "job"
		nodeTolerations["runner.gitlab.com/job="] = "NoExecute"
	}

	return &common.Build{
		Job: jobResponse,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: common.ExecutorKubernetes,
				Kubernetes: &common.KubernetesConfig{
					Image:      image,
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
					PodLabels: map[string]string{
						"test.k8s.gitlab.com/name":      podUUID,
						"test.k8s.gitlab.com/test-name": dns.MakeRFC1123Compatible(strings.Join(tt, ".")),
					},
					Namespace:                        ciNamespace,
					CleanupGracePeriodSeconds:        common.Int64Ptr(5),
					PodTerminationGracePeriodSeconds: common.Int64Ptr(5),
					PollTimeout:                      int((time.Minute * 10).Seconds()),
					NodeSelector:                     nodeSelector,
					NodeTolerations:                  nodeTolerations,

					CPULimit:            "0.3",
					MemoryRequest:       "150Mi",
					HelperCPULimit:      "0.2",
					HelperMemoryRequest: "150Mi",
				},
			},
		},
	}
}

func getTestBuild(t *testing.T, getJobResponse func() (spec.Job, error)) *common.Build {
	return getTestBuildWithImage(t, common.TestAlpineImage, getJobResponse)
}

func getTestBuildWithServices(
	t *testing.T,
	getJobResponse func() (spec.Job, error),
	services ...string,
) *common.Build {
	build := getTestBuild(t, getJobResponse)

	for _, service := range services {
		build.Services = append(build.Services, spec.Image{
			Name: service,
		})
	}

	return build
}

func getTestKubeClusterClient(t *testing.T) *k8s.Clientset {
	// Taken from the k8s client to create a config with a custom token
	// this token is linked to a separate service account that is not the one set as the
	// service account of the pod running the integration tests.
	// The service account set on the pod has all the permissions GitLab Runner needs to execute
	// builds in Kubernetes, but it doesn't have permissions needed for the integration tests to run,
	// such as listing pods or creating secrets. The admin service account is used specifically for that.
	const (
		tokenPath   = "/var/run/secrets/kubernetes.io/serviceaccount_admin/token"
		tokenCAPath = "/var/run/secrets/kubernetes.io/serviceaccount_admin/ca.crt"
	)

	var config *rest.Config
	if _, err := os.Stat(tokenPath); err != nil {
		config, err = kubernetes.GetKubeClientConfig(new(common.KubernetesConfig))
		require.NoError(t, err)
	} else {
		host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		if len(host) == 0 || len(port) == 0 {
			t.Fatal(rest.ErrNotInCluster)
		}

		token, err := os.ReadFile(tokenPath)
		require.NoError(t, err)

		tlsClientConfig := rest.TLSClientConfig{CAFile: tokenCAPath}

		config = &rest.Config{
			Host:            "https://" + net.JoinHostPort(host, port),
			TLSClientConfig: tlsClientConfig,
			BearerToken:     string(token),
			BearerTokenFile: tokenPath,
		}
	}

	client, err := k8s.NewForConfig(config)
	require.NoError(t, err)

	return client
}

// getSecrets retrieves all the secrets found in the given namespace
// with at least one ownerReference name matching the name given
// If ownerName is an empty string, all the secrets resources found are returned
func getSecrets(client *k8s.Clientset, namespace, ownerName string) ([]v1.Secret, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	credList, err := client.CoreV1().Secrets(namespace).List(
		ctx,
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, err
	}

	credentials := []v1.Secret{}
	for _, cred := range credList.Items {
		if len(cred.OwnerReferences) == 1 && cred.OwnerReferences[0].Name == ownerName {
			credentials = append(credentials, cred)
		}
	}

	return credentials, nil
}

// getConfigMaps retrieves all the configMaps found in the given namespace
// with at least one ownerReference name matching the name given
// If ownerName is an empty string, all the configMaps resources found are returned
func getConfigMaps(client *k8s.Clientset, namespace, ownerName string) ([]v1.ConfigMap, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	configMapList, err := client.CoreV1().ConfigMaps(namespace).List(
		ctx,
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, err
	}

	configMaps := []v1.ConfigMap{}
	for _, cfg := range configMapList.Items {
		if len(cfg.OwnerReferences) == 1 && cfg.OwnerReferences[0].Name == ownerName {
			configMaps = append(configMaps, cfg)
		}
	}

	return configMaps, nil
}

func runMultiPullPolicyBuild(t *testing.T, build *common.Build) error {
	build.Runner.Kubernetes.PullPolicy = common.StringOrArray{
		common.PullPolicyAlways,
		common.PullPolicyIfNotPresent,
	}

	outBuffer := bytes.NewBuffer(nil)

	err := build.Run(&common.Config{}, &common.Trace{Writer: outBuffer})
	require.Error(t, err)
	assert.ErrorIs(t, err, &common.BuildError{FailureReason: common.ImagePullFailure})

	quotedImage := regexp.QuoteMeta("some/non-existing/image")
	warningFmt := `WARNING: Failed to pull image "%s" for container "[^"]+" with policy "%s": image pull failed:`
	attemptFmt := `Attempt #%d: Trying "%s" pull policy for "%s" image for container "[^"]+"`

	// We expect
	//  - the warning for the 1st attempt with "Always", telling us about the pull issue
	//  - the log of the 2nd attempt with "IfNotPresent"
	//  - the warning for the 2. attempt with "IfNotPresent", telling us about the 2nd pull issue
	expectedLogRE := fmt.Sprintf(
		`(?s)%s.*%s.*%s`,
		fmt.Sprintf(warningFmt, quotedImage, "Always"),
		fmt.Sprintf(attemptFmt, 2, "IfNotPresent", quotedImage),
		fmt.Sprintf(warningFmt, quotedImage, "IfNotPresent"),
	)

	assert.Regexp(t, expectedLogRE, outBuffer.String())

	return err
}

func mustCreateResourceList(t *testing.T, cpu, memory string) v1.ResourceList {
	var rCPU, rMemory resource.Quantity
	var err error
	if cpu != "" {
		rCPU, err = resource.ParseQuantity(cpu)
	}
	require.NoError(t, err)

	if memory != "" {
		rMemory, err = resource.ParseQuantity(memory)
	}
	require.NoError(t, err)

	resources := make(v1.ResourceList)
	q := resource.Quantity{}

	if rCPU != q {
		resources[v1.ResourceCPU] = rCPU
	}
	if rMemory != q {
		resources[v1.ResourceMemory] = rMemory
	}

	return resources
}

func skipKubectlIntegrationTestsIfNotOnLinux(t *testing.T, client *k8s.Clientset) {
	nodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)

	os := nodes.Items[0].Status.NodeInfo.OperatingSystem

	// skip tests on windows cluster
	if os != "linux" {
		t.Skip("Non linux -- skipping tests")
	}
}

func skipKubectlIntegrationTestsIfOnOldCluster(t *testing.T, client *k8s.Clientset, minimalVersion string) {
	serverVersion, err := client.Discovery().ServerVersion()
	require.NoError(t, err)

	version, err := versionutil.Parse(serverVersion.String())
	require.NoError(t, err)

	res, err := version.Compare(minimalVersion)
	require.NoError(t, err)

	// skip tests if cluster is below minimalVersion
	if res == -1 {
		t.Skipf("Kubernetes server (%s) is older than %s -- skipping tests", serverVersion.String(), minimalVersion)
	}
}

func TestKubernetesBuildPodResources(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	client := getTestKubeClusterClient(t)

	// Pod Level Resources Graduated to Beta in kubernetes v1.34
	skipKubectlIntegrationTestsIfOnOldCluster(t, client, "1.34.0")
	// Pod-level resources are not supported for Windows pods
	skipKubectlIntegrationTestsIfNotOnLinux(t, client)

	ctxTimeout := time.Minute

	tests := map[string]struct {
		resources map[string]string
		verifyFn  func(*testing.T, v1.Pod)
	}{
		"set all pod-level resources": {
			resources: map[string]string{
				"PodCPURequest":    "1",
				"PodCPULimit":      "4",
				"PodMemoryRequest": "1Gi",
				"PodMemoryLimit":   "8Gi",
			},
			verifyFn: func(t *testing.T, pod v1.Pod) {
				resources := pod.Spec.Resources
				expectedRequests := mustCreateResourceList(t, "1", "1Gi")
				expectedLimits := mustCreateResourceList(t, "4", "8Gi")

				require.NotNil(t, resources)
				assert.Equal(t, expectedRequests, resources.Requests)
				assert.Equal(t, expectedLimits, resources.Limits)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)

			build.Runner.Kubernetes.PodCPURequest = test.resources["PodCPURequest"]
			build.Runner.Kubernetes.PodCPULimit = test.resources["PodCPULimit"]
			build.Runner.Kubernetes.PodMemoryRequest = test.resources["PodMemoryRequest"]
			build.Runner.Kubernetes.PodMemoryLimit = test.resources["PodMemoryLimit"]

			defer buildtest.OnUserStage(build, func() {
				ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
				defer cancel()
				pods, err := client.CoreV1().Pods(ciNamespace).List(
					ctx,
					metav1.ListOptions{
						LabelSelector: labels.Set(build.Runner.Kubernetes.PodLabels).String(),
					},
				)
				require.NoError(t, err)
				require.NotEmpty(t, pods.Items)
				pod := pods.Items[0]

				test.verifyFn(t, pod)
			})()

			err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			assert.NoError(t, err)
		})
	}
}

func TestKubernetesAllowedImages(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	type testDef struct {
		AllowedImages []string
		Image         string
		VerifyFn      func(*testing.T, error)
	}
	tests := map[string]testDef{
		// allowed image case
		"allowed image case": {
			AllowedImages: []string{"alpine"},
			Image:         "alpine",
			VerifyFn: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		// disallowed image case
		"disallowed image case": {
			AllowedImages: []string{"alpine"},
			Image:         "ubuntu",
			VerifyFn: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorIs(t, err, common.ErrDisallowedImage)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
			build.Runner.Kubernetes.AllowedImages = test.AllowedImages
			build.Image.Name = test.Image

			err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			test.VerifyFn(t, err)
		})
	}
}

func TestKubernetesAllowedServices(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	type testDef struct {
		AllowedServices []string
		Services        spec.Services
		VerifyFn        func(*testing.T, error)
	}
	tests := map[string]testDef{
		"allowed service case": {
			AllowedServices: []string{"alpine", "debian"},
			Services: spec.Services{
				spec.Image{Name: "alpine"},
			},
			VerifyFn: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		"disallowed service case": {
			AllowedServices: []string{"alpine", "debian"},
			Services: spec.Services{
				spec.Image{Name: "alpine"},
				spec.Image{Name: "ubuntu"},
			},
			VerifyFn: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorIs(t, err, common.ErrDisallowedImage)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
			build.Runner.Kubernetes.AllowedServices = test.AllowedServices
			build.Services = test.Services

			err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			test.VerifyFn(t, err)
		})
	}
}

func TestCleanupProjectGitClone(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	buildtest.RunBuildWithCleanupGitClone(t, getTestBuild(t, common.GetRemoteSuccessfulBuild))
}

func TestCleanupProjectGitFetch(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	untrackedFilename := "untracked"

	build := getTestBuild(t, func() (spec.Job, error) {
		return common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFilename, "", "")...,
		)
	})

	buildtest.RunBuildWithCleanupGitFetch(t, build, untrackedFilename)
}

func TestCleanupProjectGitSubmoduleNormal(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	untrackedFile := "untracked"
	untrackedSubmoduleFile := "untracked_submodule"

	build := getTestBuild(t, func() (spec.Job, error) {
		return common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFile, untrackedSubmoduleFile, "")...,
		)
	})

	buildtest.RunBuildWithCleanupNormalSubmoduleStrategy(t, build, untrackedFile, untrackedSubmoduleFile)
}

func TestCleanupProjectGitSubmoduleRecursive(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	untrackedFile := "untracked"
	untrackedSubmoduleFile := "untracked_submodule"
	untrackedSubSubmoduleFile := "untracked_submodule_submodule"

	build := getTestBuild(t, func() (spec.Job, error) {
		return common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(
				untrackedFile,
				untrackedSubmoduleFile,
				untrackedSubSubmoduleFile)...,
		)
	})

	buildtest.RunBuildWithCleanupRecursiveSubmoduleStrategy(
		t,
		build,
		untrackedFile,
		untrackedSubmoduleFile,
		untrackedSubSubmoduleFile,
	)
}

func TestKubernetesPwshFeatureFlag(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := []struct {
		script              string
		expectedRegexp      string
		expectedErrExitCode int
	}{
		{
			script:         "Write-Output $PSVersionTable",
			expectedRegexp: "PSEdition +Core",
		},
		{
			script:         "return 129",
			expectedRegexp: "Job succeeded",
		},
		{
			script:              "Write-Error 'should fail'",
			expectedErrExitCode: 1,
		},
		{
			script:              "Exit 128",
			expectedErrExitCode: 128,
		},
		{
			script:              "$host.SetShouldExit(130)",
			expectedErrExitCode: 130,
		},
		{
			script:              "eco 'function error'",
			expectedErrExitCode: 1,
		},
		{
			script:              "syntax error {{}",
			expectedErrExitCode: 1,
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)

			build.Image.Name = common.TestPwshImage
			build.Runner.Shell = shells.SNPwsh
			build.Job.Steps = spec.Steps{
				spec.Step{
					Name:   spec.StepNameScript,
					Script: []string{tc.script},
				},
			}

			out, err := buildtest.RunBuildReturningOutput(t, build)
			assert.Regexp(t, regexp.MustCompile(tc.expectedRegexp), out)

			if tc.expectedErrExitCode != 0 {
				var buildError *common.BuildError
				if assert.ErrorAs(t, err, &buildError) {
					assert.Equal(t, tc.expectedErrExitCode, buildError.ExitCode)
				}
			}
		})
	}
}

func TestKubernetesProcessesInBackground(t *testing.T) {
	t.Parallel()

	// Check fix for https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2880

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		shell  string
		image  string
		script []string
	}{
		"bash shell": {
			shell: "bash",
			image: common.TestAlpineImage,
		},
		"pwsh shell": {
			shell: shells.SNPwsh,
			image: common.TestPwshImage,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)

			build.Image.Name = tc.image
			build.Runner.Shell = tc.shell
			build.Job.Steps = spec.Steps{
				spec.Step{
					Name: spec.StepNameScript,
					Script: []string{
						`sleep infinity &`,
						`mkdir out && echo "Hello, world" > out/greeting`,
					},
				},
				spec.Step{
					Name: spec.StepNameAfterScript,
					Script: []string{
						`echo I should be running`,
					},
				},
			}

			out, err := buildtest.RunBuildReturningOutput(t, build)
			assert.Contains(t, out, "I should be running")
			assert.NoError(t, err)
		})
	}
}

func TestBuildExpandedFileVariable(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
		buildtest.RunBuildWithExpandedFileVariable(t, build.Runner, nil)
	})
}

func TestConflictingPullPolicies(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		imagePullPolicies   []common.DockerPullPolicy
		pullPolicy          common.StringOrArray
		allowedPullPolicies []common.DockerPullPolicy
		wantErrRegexp       string
	}{
		"allowed_pull_policies configured, default pull_policy": {
			imagePullPolicies:   nil,
			pullPolicy:          nil,
			allowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			wantErrRegexp:       `Runner config \(default\) .*IfNotPresent`,
		},
		"allowed_pull_policies and pull_policy configured": {
			imagePullPolicies:   nil,
			pullPolicy:          common.StringOrArray{common.PullPolicyNever},
			allowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			wantErrRegexp:       `Never.* Runner config .*IfNotPresent`,
		},
		"allowed_pull_policies and image pull_policy configured": {
			imagePullPolicies:   []common.DockerPullPolicy{common.PullPolicyAlways},
			pullPolicy:          nil,
			allowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			wantErrRegexp:       `Always.* GitLab pipeline config .*IfNotPresent`,
		},
		"all configured": {
			imagePullPolicies:   []common.DockerPullPolicy{common.PullPolicyAlways},
			pullPolicy:          common.StringOrArray{common.PullPolicyNever},
			allowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			wantErrRegexp:       `Always.* GitLab pipeline config .*IfNotPresent`,
		},
	}

	errorRE := regexp.MustCompile(`invalid pull policy for container "(build|helper|init-permissions)"`)

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
			build.Job.Image.PullPolicies = test.imagePullPolicies
			build.Runner.RunnerSettings.Kubernetes.PullPolicy = test.pullPolicy
			build.Runner.RunnerSettings.Kubernetes.AllowedPullPolicies = test.allowedPullPolicies

			gotErr := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})

			require.Error(t, gotErr)
			assert.Regexp(t, regexp.MustCompile(test.wantErrRegexp), gotErr.Error())
			assert.Regexp(t, errorRE, gotErr.Error())
		})
	}
}

func Test_CaptureServiceLogs(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		buildVars []spec.Variable
		assert    func(string, error)
	}{
		"enabled": {
			buildVars: []spec.Variable{
				{
					Key:    "CI_DEBUG_SERVICES",
					Value:  "1",
					Public: true,
				},
			},
			assert: func(out string, err error) {
				assert.NoError(t, err)
				assert.NotContains(t, out, "WARNING: invalid value '1' for CI_DEBUG_SERVICES variable")
				assert.Regexp(t, `\[service:postgres-db\] .* The files belonging to this database system will be owned by user "postgres"`, out)
				assert.Regexp(t, `\[service:postgres-db\] .* database system is ready to accept connections`, out)
				assert.Regexp(t, `\[service:redis-cache\] .* oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0O`, out)
				assert.Regexp(t, `\[service:redis-cache\] .* Ready to accept connections`, out)
			},
		},
		"not enabled": {
			assert: func(out string, err error) {
				assert.NoError(t, err)
				assert.NotContains(t, out, "WARNING: invalid value '1' for CI_DEBUG_SERVICES variable")
				assert.NotRegexp(t, `\[service:postgres-db\] .* The files belonging to this database system will be owned by user "postgres"`, out)
				assert.NotRegexp(t, `\[service:postgres-db\] .* database system is ready to accept connections`, out)
				assert.NotRegexp(t, `\[service:redis-cache\] .* oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0O`, out)
				assert.NotRegexp(t, `\[service:redis-cache\] .* Ready to accept connections`, out)
			},
		},
		"bogus value": {
			buildVars: []spec.Variable{{
				Key:    "CI_DEBUG_SERVICES",
				Value:  "blammo",
				Public: true,
			}},
			assert: func(out string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, out, `WARNING: CI_DEBUG_SERVICES: expected bool got "blammo", using default value: false`)
				assert.NotRegexp(t, `\[service:postgres-db\] .* The files belonging to this database system will be owned by user "postgres"`, out)
				assert.NotRegexp(t, `\[service:postgres-db\] .* database system is ready to accept connections`, out)
				assert.NotRegexp(t, `\[service:redis-cache\] .* oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0O`, out)
				assert.NotRegexp(t, `\[service:redis-cache\] .* Ready to accept connections`, out)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			build := getTestBuildWithServices(t, common.GetRemoteSuccessfulBuild, "postgres:14.4", "redis:7.0")
			build.Services[0].Alias = "db"
			build.Services[1].Alias = "cache"
			build.Variables = tt.buildVars
			build.Variables = append(build.Variables, spec.Variable{
				Key:    "POSTGRES_PASSWORD",
				Value:  "password",
				Public: true,
			})

			out, err := buildtest.RunBuildReturningOutput(t, build)
			tt.assert(out, err)
		})
	}
}

// When testing with minikube, the following commands may be used to
// properly configure the cluster:
//
// minikube config set container-runtime containerd
// minikube config set feature-gates "ProcMountType=true"
//
// Note that the cluster must be re-initialized after making these changes:
//
// minikube delete
// minikube start
func TestKubernetesProcMount(t *testing.T) {
	t.Parallel()
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	setBuildWithProcMount := func(build *common.Build, procMountType v1.ProcMountType) {
		build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "none"})
		build.Runner.RunnerSettings.Kubernetes.BuildContainerSecurityContext = common.KubernetesContainerSecurityContext{
			ProcMount:  procMountType,
			Privileged: &[]bool{false}[0], // unpriv'ed
		}
		if strings.EqualFold(strings.TrimSpace(string(procMountType)), "unmasked") {
			// when we set "unmsked", we also need to set .spec.hostUsers to false explicitly (since: 1.33?)
			// for that we need to leverage the pod spec patch feature
			build.Runner.RunnerSettings.FeatureFlags = map[string]bool{
				featureflags.UseAdvancedPodSpecConfiguration: true,
			}
			build.Runner.RunnerSettings.Kubernetes.PodSpec = append(build.Runner.RunnerSettings.Kubernetes.PodSpec, common.KubernetesPodSpec{
				Name:      "disable_host_usersns",
				PatchType: common.PatchTypeJSONPatchType,
				Patch: `[{
					"op": "add",
					"path": "/hostUsers",
					"value": false
				}]`,
			})
		}
	}

	// Generate a temporary Pod with procMount set to Unmasked.
	// If the cluster supports the ProcMount feature, then this will be reflected
	// in the PodSpec. If the cluster does not support this feature, the API server
	// will return DefaultProcMount.
	tmpPod := getTestBuild(t, func() (spec.Job, error) {
		return common.GetRemoteBuildResponse("cat")
	})

	setBuildWithProcMount(tmpPod, v1.UnmaskedProcMount)

	shouldSkipCh := make(chan bool)
	cleanup := buildtest.OnUserStage(tmpPod, func() {
		client := getTestKubeClusterClient(t)

		pods, err := client.
			CoreV1().
			Pods(ciNamespace).
			List(t.Context(), metav1.ListOptions{
				LabelSelector: labels.Set(tmpPod.Runner.Kubernetes.PodLabels).String(),
			})

		require.NoError(t, err)
		require.NotEmpty(t, pods.Items)

		pod := pods.Items[0]

		require.NotEmpty(t, pod.Spec.Containers)

		container := pod.Spec.Containers[0]

		procMount := container.SecurityContext.ProcMount
		shouldSkipCh <- procMount == nil || *procMount != v1.UnmaskedProcMount
	})
	defer cleanup()

	buildtest.RunBuildReturningOutput(t, tmpPod)

	shouldSkip := <-shouldSkipCh
	if shouldSkip {
		t.Skip("ProcMountType feature not supported on cluster -- skipping tests")
		return
	}

	// If we get here, then we have validated that the cluster does indeed support the
	// ProcMount feature, and we can proceed with a more thorough set of tests.

	var buildErr *common.BuildError

	tests := map[string]struct {
		procMount v1.ProcMountType
		validate  func(*testing.T, string, error)
	}{
		"Default": {
			procMount: v1.DefaultProcMount,
			validate: func(t *testing.T, out string, err error) {
				assert.ErrorAs(t, err, &buildErr)
				assert.Contains(t, out, "Job failed")
				assert.Contains(t, out, "[masked]")
			},
		},
		"default": {
			procMount: v1.ProcMountType("default"),
			validate: func(t *testing.T, out string, err error) {
				assert.ErrorAs(t, err, &buildErr)
				assert.Contains(t, out, "Job failed")
				assert.Contains(t, out, "[masked]")
			},
		},
		"Unmasked": {
			procMount: v1.UnmaskedProcMount,
			validate: func(t *testing.T, out string, err error) {
				require.NoError(t, err)
				assert.Contains(t, out, "Job succeeded")
				assert.Contains(t, out, "[unmasked]")
			},
		},
		"unmasked": {
			procMount: v1.ProcMountType("unmasked"),
			validate: func(t *testing.T, out string, err error) {
				require.NoError(t, err)
				assert.Contains(t, out, "Job succeeded")
				assert.Contains(t, out, "[unmasked]")
			},
		},
		"empty": {
			procMount: v1.ProcMountType("   "),
			validate: func(t *testing.T, out string, err error) {
				assert.ErrorAs(t, err, &buildErr)
				assert.Contains(t, out, "Job failed")
				assert.Contains(t, out, "[masked]")
			},
		},
		"invalid": {
			procMount: v1.ProcMountType("invalid"),
			validate: func(t *testing.T, out string, err error) {
				assert.ErrorAs(t, err, &buildErr)
				assert.Contains(t, out, "Job failed")
				assert.Contains(t, out, "[masked]")
			},
		},
	}

	const testScript = `
		if mount | grep 'proc on /proc' | grep -q 'ro,'
		then
			echo '[masked] masked /proc paths found, some paths have ro mount overwrites'
			exit 1
		fi
		echo '[unmasked] /proc is unmasked, no ro mount overwrites'
	`

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse(testScript)
			})

			setBuildWithProcMount(build, test.procMount)

			out, err := buildtest.RunBuildReturningOutput(t, build)
			test.validate(t, out, err)
		})
	}
}

func Test_ContainerOptionsExpansion(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)

	jobVars := spec.Variables{
		{Key: "CI_DEBUG_SERVICES", Value: "true", Public: true},
		{Key: "POSTGRES_PASSWORD", Value: "password", Public: true},
		{Key: "JOB_IMAGE", Value: "alpine:latest"},
		{Key: "HELPER_IMAGE", Value: "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest"},
		{Key: "HELPER_IMAGE_FLAVOR", Value: "alpine"},
		{Key: "SRVS_IMAGE", Value: "postgres:latest"},
		{Key: "SRVS_IMAGE_ALIAS", Value: "db"},
	}
	build.Variables = append(build.Variables, jobVars...)

	build.Runner.Kubernetes.Image = "$JOB_IMAGE"
	build.Runner.Kubernetes.HelperImage = "$HELPER_IMAGE"
	build.Runner.Kubernetes.HelperImageFlavor = "$HELPER_IMAGE_FLAVOR"
	build.Services = []spec.Image{
		{Name: "$SRVS_IMAGE", Alias: "$SRVS_IMAGE_ALIAS"},
	}

	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	// the helper image name does not appeart in the logs, but the build will fail if the option was not expanded.
	assert.Contains(t, out, "Using Kubernetes executor with image alpine:latest")
	assert.Regexp(t, `\[service:postgres-db\]`, out)
}

func testJobRunningAndPassingWhenServiceStops(t *testing.T, featureFlagName string, featureFlagValue bool) {
	build := getTestBuild(t, func() (spec.Job, error) {
		jobResponse, err := common.GetRemoteBuildResponse("sleep 12")
		if err != nil {
			return spec.Job{}, err
		}

		jobResponse.Steps = append(
			jobResponse.Steps,
			spec.Step{
				Name:   spec.StepNameAfterScript,
				Script: []string{"echo after script"},
			},
		)

		return jobResponse, nil
	})

	build.Runner.Kubernetes.Services = []common.Service{{
		Name: counterServiceImage,
	}}

	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := buildtest.RunBuild(t, build)
	require.NoError(t, err)
}

func testKubernetesServiceContainerAlias(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	ctxTimeout := time.Minute
	client := getTestKubeClusterClient(t)

	tests := map[string]struct {
		services   spec.Services
		lookupName []string
	}{
		"service container without alias": {
			services: spec.Services{
				{
					Name: common.TestAlpineImage,
				},
			},
			lookupName: []string{"svc-0"},
		},
		"service container with alias": {
			services: spec.Services{
				{
					Name:  common.TestAlpineImage,
					Alias: "alpine-service",
				},
			},
			lookupName: []string{"alpine-service"},
		},
		"service container with multiple different aliases": {
			services: spec.Services{
				{
					Name:  common.TestAlpineImage,
					Alias: "alpine-service-1",
				},
				{
					Name:  common.TestAlpineImage,
					Alias: "alpine-service-2",
				},
			},
			lookupName: []string{"alpine-service-1", "alpine-service-2"},
		},
		"service container with multiple similar aliases": {
			services: spec.Services{
				{
					Name:  common.TestAlpineImage,
					Alias: "alpine-service",
				},
				{
					Name:  common.TestAlpineImage,
					Alias: "alpine-service",
				},
			},
			lookupName: []string{"alpine-service", "svc-0"},
		},
		"service container with multiple similar aliases 2": {
			services: spec.Services{
				{
					Name:  common.TestAlpineImage,
					Alias: "alpine,foo,bar",
				},
				{
					Name:  common.TestAlpineImage,
					Alias: "alpine,foo,bar",
				},
				{
					Name:  common.TestAlpineImage,
					Alias: "alpine,foo,bar",
				},
				{
					Name:  common.TestAlpineImage,
					Alias: "alpine,foo,bar",
				},
			},
			lookupName: []string{"alpine", "foo", "bar", "svc-0"},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse(
					"sleep 60",
				)
			})
			build.Services = tc.services
			buildtest.SetBuildFeatureFlag(build, featureflags.UseLegacyKubernetesExecutionStrategy, false)
			buildtest.SetBuildFeatureFlag(build, featureflags.PrintPodEvents, true)

			ctx, cancel := context.WithTimeout(t.Context(), ctxTimeout)
			deletedPodNameCh := make(chan string)
			defer buildtest.OnUserStage(build, func() {
				defer cancel()
				pods, err := client.CoreV1().
					Pods(ciNamespace).
					List(
						ctx,
						metav1.ListOptions{
							LabelSelector: labels.Set(build.Runner.Kubernetes.PodLabels).String(),
						},
					)
				require.NoError(t, err)
				require.NotEmpty(t, pods.Items)
				pod := pods.Items[0]

				names := make([]string, 0)
				for _, container := range pod.Spec.Containers {
					names = append(names, container.Name)
				}

				for _, lookup := range tc.lookupName {
					assert.Contains(t, names, lookup)
				}

				err = client.
					CoreV1().
					Pods(ciNamespace).
					Delete(ctx, pod.Name, metav1.DeleteOptions{
						PropagationPolicy: &kubernetes.PropagationPolicy,
					})
				require.NoError(t, err)

				deletedPodNameCh <- pod.Name
			})()

			err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			require.Error(t, err)

			select {
			case <-deletedPodNameCh:
				err = nil
			case <-ctx.Done():
				err = fmt.Errorf("test terminated through context expiration")
			}

			require.NoError(t, err)
		})
	}
}

func testKubernetesOptionsUserAndGroup(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		ciUserId  string
		cfgUserId func() *int64
		verifyFn  func(t *testing.T, out string)
	}{
		"no user set": {
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "uid and gid is set to: 0:0")
			},
		},
		"user set to 1002": {
			ciUserId: "1002",
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "uid and gid is set to: 1002:0")
			},
		},
		"uid set to 1002 and gid set to 1002": {
			ciUserId: "1002:1002",
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "uid and gid is set to: 1002:1002")
			},
		},
		"user set to 1002 in gitlab-ci and 1003 in config.toml": {
			ciUserId: "1002",
			cfgUserId: func() *int64 {
				id := int64(1003)
				return &id
			},
			verifyFn: func(t *testing.T, out string) {
				assert.Contains(t, out, "uid and gid is set to: 1003:0")
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			build := getTestBuild(t, func() (spec.Job, error) {
				jobResponse, err := common.GetRemoteBuildResponse(`echo "uid and gid is set to: $(id -u):$(id -g)"`)
				if err != nil {
					return spec.Job{}, err
				}

				jobResponse.Image.ExecutorOptions.Kubernetes = spec.ImageKubernetesOptions{
					User: spec.StringOrInt64(tc.ciUserId),
				}

				return jobResponse, nil
			})

			if tc.cfgUserId != nil {
				build.Runner.Kubernetes.BuildContainerSecurityContext.RunAsUser = tc.cfgUserId()
			}

			buildtest.SetBuildFeatureFlag(build, featureflags.UseLegacyKubernetesExecutionStrategy, false)
			buildtest.SetBuildFeatureFlag(build, featureflags.PrintPodEvents, true)

			var buf bytes.Buffer
			err := build.Run(&common.Config{}, &common.Trace{Writer: &buf})
			require.NoError(t, err)

			if tc.verifyFn != nil {
				tc.verifyFn(t, buf.String())
			}
		})
	}
}

func TestEntrypointLogging(t *testing.T) {
	t.Skip("TODO: Flaky, fix with https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5175/diffs?commit_id=d424ae620f90db86bacc3696f3b8727886e1f85b")

	t.Run("succeed", testEntrypointLoggingSuccesses)
	t.Run("fail", testEntrypointLoggingFailures)
}

func testEntrypointLoggingFailures(t *testing.T) {
	t.Parallel()

	// When the pollTimeout is smaller than the time it takes for the entrypoint to start the shell, and thus resolve the
	// startupProbe (roughly 1sec * iterations), then the build should fail but still show _some_ of the entrypoint logs (until
	// the pod gets killed because of the timeout)
	// Note: We only use a startup probe in exec mode
	t.Run("startupProbe does not resolve in time", func(t *testing.T) {
		t.Parallel()

		const pollTimeout = 4
		const iterations = 8

		build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
		build.Runner.Kubernetes.Image = "registry.gitlab.com/gitlab-org/gitlab-runner/alpine-entrypoint-pre-post-trap"
		build.Runner.Kubernetes.PollTimeout = pollTimeout
		build.Runner.FeatureFlags = mapFromKeySlices(true, []string{
			featureflags.KubernetesHonorEntrypoint,
			featureflags.UseLegacyKubernetesExecutionStrategy,
		})
		build.Runner.Environment = []string{
			fmt.Sprintf("LOOP_ITERATIONS=%d", iterations),
		}

		out, err := buildtest.RunBuildReturningOutput(t, build)
		if assert.Error(t, err, "expected build to fail, but did not") {
			assert.Contains(t, err.Error(), "timed out")
		}

		// we see some entrypoint logs
		assert.Contains(t, out, "some pre message")
	})
}

func testEntrypointLoggingSuccesses(t *testing.T) {
	t.Parallel()

	const pollTimeout = 12
	const loopIterations = 5
	defaultFeatureFlags := []string{featureflags.ScriptSections, featureflags.PrintPodEvents}

	expectedLogs := func(phase string, count int) []string {
		expectedLogs := make([]string, count*2)
		for idx := 0; idx < count; idx++ {
			// produces something like: "[entrypoint][post][stdout][5/10] "
			expectedLogs[idx] = fmt.Sprintf("[entrypoint][%s][stdout][%d/%d] ", phase, idx, count)
			expectedLogs[idx+count] = fmt.Sprintf("[entrypoint][%s][stderr][%d/%d] ", phase, idx, count)
		}
		return expectedLogs
	}

	runtimeEnvs := map[string]struct {
		shell string
		image string
	}{
		"bash": {shell: shells.Bash, image: "registry.gitlab.com/gitlab-org/gitlab-runner/alpine-entrypoint-pre-post-trap"},
		"pwsh": {shell: shells.SNPwsh, image: "registry.gitlab.com/gitlab-org/gitlab-runner/powershell-entrypoint-pre-post-trap"},
	}
	modes := map[string][]string{
		"attach mode": { /* attach mode is the default, no additional FFs needed */ },
		"exec mode":   {featureflags.UseLegacyKubernetesExecutionStrategy},
	}
	tests := map[string]struct {
		featureFlags  []string
		expectLogs    []string
		notExpectLogs []string
	}{
		"not honoring entrypoint": {
			notExpectLogs: append(expectedLogs("pre", loopIterations), expectedLogs("post", loopIterations)...),
		},
		"honoring entrypoint": {
			featureFlags: []string{featureflags.KubernetesHonorEntrypoint},
			expectLogs:   expectedLogs("pre", loopIterations),
		},
	}

	for runtimeName, runtimeEnv := range runtimeEnvs {
		t.Run(runtimeName, func(t *testing.T) {
			t.Parallel()
			if runtimeName == "pwsh" {
				t.Skip("TODO: pwsh doesn't work")
			}

			for mode, modeFeatureFlags := range modes {
				t.Run(mode, func(t *testing.T) {
					t.Parallel()

					for testName, testConfig := range tests {
						t.Run(testName, func(t *testing.T) {
							t.Parallel()

							build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
							build.Runner.Kubernetes.Image = runtimeEnv.image
							build.Runner.Kubernetes.PollTimeout = pollTimeout
							build.Runner.FeatureFlags = mapFromKeySlices(true, defaultFeatureFlags, modeFeatureFlags, testConfig.featureFlags)
							build.Runner.Environment = []string{
								fmt.Sprintf("LOOP_ITERATIONS=%d", loopIterations),
							}
							build.Runner.Shell = runtimeEnv.shell

							out, err := buildtest.RunBuildReturningOutput(t, build)
							assert.NoError(t, err)

							for _, s := range testConfig.expectLogs {
								occurrences := strings.Count(out, s)
								assert.Equal(t, 1, occurrences, "expected to find %q exactly once, found it %d times", s, occurrences)
							}
							for _, s := range testConfig.notExpectLogs {
								assert.NotContains(t, out, s, "expected output not to contain %q, but does", s)
							}
						}) // tests
					}
				}) // modes
			}
		}) // runtimeEnvs
	}
}

// mapFromKeySlices gives you a new map, with some keys already set to the value provided.
func mapFromKeySlices[K comparable, V any](value V, keySlices ...[]K) map[K]V {
	m := map[K]V{}

	for _, keySlice := range keySlices {
		for _, key := range keySlice {
			m[key] = value
		}
	}

	return m
}

func TestKubernetesScriptsBaseDir(t *testing.T) {
	t.Parallel()
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		image    string
		shell    string
		script   string
		baseDir  string
		verifyFn func(t *testing.T, out string)
	}{
		"scripts_base_dir enabled": {
			image:   common.TestAlpineImage,
			shell:   "bash",
			script:  "find /tmp",
			baseDir: "/tmp",
			verifyFn: func(t *testing.T, out string) {
				assert.Regexp(t, regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s+\w+\s+/tmp/scripts-0-0$`), out)
			},
		},
		"scripts_base_dir trailing slash": {
			image:   common.TestAlpineImage,
			shell:   "bash",
			script:  "find /tmp",
			baseDir: "/tmp/",
			verifyFn: func(t *testing.T, out string) {
				assert.Regexp(t, regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s+\w+\s+/tmp/scripts-0-0$`), out)
			},
		},
		"scripts_base_dir disabled": {
			image:   common.TestAlpineImage,
			shell:   "bash",
			script:  "find / -maxdepth 1",
			baseDir: "",
			verifyFn: func(t *testing.T, out string) {
				assert.Regexp(t, regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s+\w+\s+/scripts-0-0$`), out)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse(tc.script)
			})

			build.Runner.RunnerSettings.Shell = tc.shell
			build.Runner.RunnerSettings.Kubernetes.ScriptsBaseDir = tc.baseDir
			build.Job.Image.Name = tc.image
			build.Runner.Kubernetes.HelperImage = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest"

			var buf bytes.Buffer
			err := build.Run(&common.Config{}, &common.Trace{Writer: &buf})
			assert.NoError(t, err)

			tc.verifyFn(t, buf.String())
		})
	}
}

func TestKubernetesLogsBaseDir(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		image    string
		shell    string
		script   string
		baseDir  string
		envVars  spec.Variables
		verifyFn func(t *testing.T, out string)
	}{
		"logs_base_dir enabled": {
			image:   common.TestAlpineImage,
			shell:   "bash",
			script:  "find /tmp",
			baseDir: "/tmp",
			verifyFn: func(t *testing.T, out string) {
				assert.Regexp(t, regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s+\w+\s+/tmp/logs-0-0$`), out)
			},
		},
		"logs_base_dir trailing slash": {
			image:   common.TestAlpineImage,
			shell:   "bash",
			script:  "find /tmp",
			baseDir: "/tmp/",
			verifyFn: func(t *testing.T, out string) {
				assert.Regexp(t, regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s+\w+\s+/tmp/logs-0-0$`), out)
			},
		},
		"logs_base_dir disabled": {
			image:   common.TestAlpineImage,
			shell:   "bash",
			script:  "find / -maxdepth 1",
			baseDir: "",
			verifyFn: func(t *testing.T, out string) {
				assert.Regexp(t, regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s+\w+\s+/logs-0-0$`), out)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse(tc.script)
			})

			build.Runner.RunnerSettings.Shell = tc.shell
			build.Runner.RunnerSettings.Kubernetes.LogsBaseDir = tc.baseDir
			build.Job.Image.Name = tc.image
			build.Runner.Kubernetes.HelperImage = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest"

			var buf bytes.Buffer
			err := build.Run(&common.Config{}, &common.Trace{Writer: &buf})
			assert.NoError(t, err)

			tc.verifyFn(t, buf.String())
		})
	}
}

func testJobAgainstServiceContainerBehaviour(t *testing.T, featureFlagName string, featureFlagValue bool) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		services spec.Services
		verifyFn func(t *testing.T, err error)
	}{
		"job fails when waiting for service port readiness and service fails": {
			services: spec.Services{
				{
					Name: "postgres:12.17-alpine3.19",
					Variables: spec.Variables{
						spec.Variable{Key: "HEALTHCHECK_TCP_PORT", Value: "5432"},
					},
				},
			},
			verifyFn: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		// Postgres service container will fail because the password and database variables are not provided
		"job passes when service fails": {
			services: spec.Services{
				{
					Name: "postgres:12.17-alpine3.19",
				},
			},
			verifyFn: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse("sleep 5s")
			})

			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
			build.Job.Image.Name = common.TestAlpineImage
			build.Job.Services = append(build.Job.Services, tc.services...)
			build.Runner.Kubernetes.HelperImage = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest"

			err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			tc.verifyFn(t, err)
		})
	}
}

func TestKubernetesUserAndGroupConstraints(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	type testDef struct {
		name          string
		runnerUID     *int64
		runnerGID     *int64
		allowedUsers  []string
		allowedGroups []string
		imageUser     string
		expectError   string
	}

	tests := []testDef{
		{
			name:          "no constraints - should succeed",
			allowedUsers:  nil,
			allowedGroups: nil,
			imageUser:     "1000:1000",
			expectError:   "",
		},
		{
			name:          "user allowed - should succeed",
			allowedUsers:  []string{"1000"},
			allowedGroups: nil,
			imageUser:     "1000:1000",
			expectError:   "",
		},
		{
			name:          "user not allowed - should succeed with warning",
			allowedUsers:  []string{"1001"},
			allowedGroups: nil,
			imageUser:     "1000:1000",
			expectError:   "", // Should succeed, not fail
		},
		{
			name:          "group not allowed - should succeed with warning",
			allowedUsers:  nil,
			allowedGroups: []string{"1001"},
			imageUser:     "1000:1000",
			expectError:   "", // Should succeed, not fail
		},
		{
			name:          "runner user takes precedence - should succeed",
			runnerUID:     common.Int64Ptr(2000),
			runnerGID:     common.Int64Ptr(2000),
			allowedUsers:  []string{"2000"},
			allowedGroups: []string{"2000"},
			imageUser:     "1000:1000",
			expectError:   "",
		},
		{
			name:          "runner user takes precedence - bypasses allowlist validation",
			runnerUID:     common.Int64Ptr(2000),
			runnerGID:     common.Int64Ptr(2000),
			allowedUsers:  []string{"1000"},
			allowedGroups: []string{"1000"},
			imageUser:     "1000:1000",
			expectError:   "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse(`
					echo "Testing Kubernetes user and group constraints"
					echo "Current user ID: $(id -u)"
					echo "Current group ID: $(id -g)"
					echo "Build completed"
				`)
			})

			build.Image.ExecutorOptions.Kubernetes.User = spec.StringOrInt64(test.imageUser)
			build.Runner.Kubernetes.AllowedUsers = test.allowedUsers
			build.Runner.Kubernetes.AllowedGroups = test.allowedGroups

			// Configure security context for admin override
			if test.runnerUID != nil {
				build.Runner.Kubernetes.BuildContainerSecurityContext.RunAsUser = test.runnerUID
			}
			if test.runnerGID != nil {
				build.Runner.Kubernetes.BuildContainerSecurityContext.RunAsGroup = test.runnerGID
			}

			var buffer bytes.Buffer
			err := build.Run(&common.Config{}, &common.Trace{Writer: &buffer})

			if test.expectError != "" {
				assert.Error(t, err, "Expected build to fail but it succeeded")
				if err != nil {
					assert.Contains(t, err.Error(), test.expectError, "Expected error message not found")
				}
			} else {
				assert.NoError(t, err, "Expected build to succeed but got error: %v", err)
			}
		})
	}
}

func TestBuildContainerOOMKilled(t *testing.T) {
	t.Parallel()

	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	tests := map[string]struct {
		script      string
		verifyFn    func(t *testing.T, out string, err error)
		memoryLimit string
	}{
		"job fails because build container is OOMKilled": {
			script: `echo "Starting memory allocation to trigger OOM..."

allocate_memory() {
	while true; do
		# Allocate a large block of memory by creating a large variable
		data=$(printf 'A%.0s' $(seq 1 1000000)) # Adjust this number for more memory allocation
		sleep 1  # Optional: add a small delay to control the speed of allocation
	done
}

allocate_memory
`,
			memoryLimit: "6Mi",
			verifyFn: func(t *testing.T, out string, err error) {
				assert.Contains(t, out, "Error in container build: exit code: 137, reason: 'OOMKilled'")
				assert.Error(t, err)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			build := getTestBuild(t, func() (spec.Job, error) {
				return common.GetRemoteBuildResponse(tc.script)
			})

			buildtest.SetBuildFeatureFlag(build, featureflags.UseLegacyKubernetesExecutionStrategy, false)
			build.Job.Image.Name = common.TestAlpineImage
			build.Runner.Kubernetes.HelperImage = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-latest"
			build.Runner.Kubernetes.MemoryLimit = tc.memoryLimit
			build.Runner.Kubernetes.MemoryRequest = tc.memoryLimit

			var buf bytes.Buffer
			err := build.Run(&common.Config{}, &common.Trace{Writer: &buf})
			tc.verifyFn(t, buf.String(), err)
		})
	}
}

// withDevHelperImage reads the artifacts from the "(development|bleeding|stable) docker images" job, extracts the
// helper image ref from there, and sets it as the build's helper image.
func withDevHelperImage(t *testing.T, build *common.Build, imageRefRE string) {
	t.Helper()

	const (
		artifactType    = "Docker image"
		artifactBaseDir = "out"
		// out/release_artifacts/helper-images_json-registry_gitlab_com_gitlab-org_gitlab-runner_gitlab-runner-helper-dev-dfb8eda29.json
		//	-> registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper-dev:x86_64-dfb8ed29
		// out/release_artifacts/helper-images_json-registry_gitlab_com_gitlab-org_gitlab-runner_gitlab-runner-helper-a2f2305f-v18_1_3.json
		//	-> registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-a2f2305f
		artifactGlob = "release_artifacts/helper-images_json-registry_gitlab_com_gitlab-org_gitlab-runner_gitlab-runner-helper-*.json"
	)

	projectDir, ok := os.LookupEnv("CI_PROJECT_DIR")
	if !ok {
		// for local runs, don't fail but warn.
		t.Logf(
			`You asked me to set the helper image based on references in %q, to an image matching %q.\n`+
				`But I am not running in CI (CI_PROJECT_DIR env var is not set), so I can't do that, sorry.\n`+
				`I will still continue, but without setting a helper image.`,
			artifactGlob, imageRefRE,
		)
		return
	}

	if imageRefRE == "" {
		imageRefRE = ":x86_64-[a-f0-9]+$"
	}

	re, err := regexp.Compile(imageRefRE)
	require.NoError(t, err, "compiling imageRefRE %q", imageRefRE)

	t.Logf("trying to find helper image with RE %q", imageRefRE)

	searchPath := filepath.Join(projectDir, artifactBaseDir)
	fullGlob := filepath.Join(searchPath, artifactGlob)

	matches, err := filepath.Glob(fullGlob)
	require.NoError(t, err, "globbing for artifact file")

	if l := len(matches); l != 1 {
		var files []string
		err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(searchPath, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
			return nil
		})
		assert.NoError(t, err, "walking dir %q", searchPath)

		t.Errorf(
			"expected to find 1 file for glob %q, but found: %d\navailable files in %q:\n%q",
			fullGlob, l, searchPath, files,
		)
		t.FailNow()
	}

	f := matches[0]
	b, err := os.ReadFile(f)
	require.NoError(t, err, "reading %q", f)

	var artifacts []struct {
		Type  string
		Value string
	}
	err = json.Unmarshal(b, &artifacts)
	require.NoError(t, err, "parsing %q", f)

	for _, artifact := range artifacts {
		if artifact.Type != artifactType {
			continue
		}
		if re.MatchString(artifact.Value) {
			build.Runner.Kubernetes.HelperImage = artifact.Value
			return
		}
	}

	require.FailNow(t, "helper image not found", "could not find image ref matching %q in %q", imageRefRE, f)
}

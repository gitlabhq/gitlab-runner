package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/rand"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/docker/cli/cli/config/types"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Register all available authentication methods
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/exec"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/watchers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/auth"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	os_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/os"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/pull_policies"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

const (
	buildContainerName          = "build"
	helperContainerName         = "helper"
	initPermissionContainerName = "init-permissions"

	detectShellScriptName         = "detect_shell_script"
	pwshJSONTerminationScriptName = "terminate_with_json_script"

	waitLogFileTimeout = time.Minute

	outputLogFileNotExistsExitCode = 100
	unknownLogProcessorExitCode    = 1000

	// nodeSelectorWindowsBuildLabel is the label used to reference a specific Windows Version.
	// https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesiowindows-build
	nodeSelectorWindowsBuildLabel = "node.kubernetes.io/windows-build"

	apiVersion         = "v1"
	ownerReferenceKind = "Pod"

	// Polling time between each attempt to check serviceAccount and imagePullSecret (in seconds)
	resourceAvailabilityCheckMaxPollInterval = 5 * time.Second

	serviceContainerPrefix = "svc-"

	// runnerLabelNamespace is used to build the k8s objects' labels and annotations, and for checking if user-defined
	// labels and label overwrites are allowed; i.e. labels within this namespace cannot be set or overwritten by
	// users, though other labels can.
	runnerLabelNamespace = "runner.gitlab.com"

	// The suffix is built using alphanumeric character
	// that means there is 34^8 possibilities for a resource name using the same pattern
	// Considering that the k8s resources are deleted after they run,
	k8sResourcesNameSuffixLength = 8
	k8sResourcesNameMaxLength    = 63

	k8sEventWarningType = "Warning"

	// errorAlreadyExistsMessage is an error message that is encountered when
	// we fail to create a resource because it already exists.
	// Because of a connectivity issue, an attempt to create a resource can fail while the request itself
	// was successfully executed. We then monitor the conflict error message to retrieve the already create resource
	errorAlreadyExistsMessage = "the server was not able to generate a unique name for the object"

	// Memory usage estimate per cache entry:
	//   key (UID:count) ≈ 40 B + string header 16 B + time.Time 24 B + LRU links 16 B ≈ 96 B
	//   For 100k entries, worst‑case RAM ≈ 10 MB.
	podEventLRUCapacity = 100000
	podEventStateTTL    = 2 * time.Hour
)

var (
	runnerLabelNamespacePattern = regexp.MustCompile(`(?i)(^|.*\.)` + regexp.QuoteMeta(runnerLabelNamespace) + `(\/.*|$)`)

	PropagationPolicy = metav1.DeletePropagationBackground

	executorOptions = executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: true,
		DefaultSafeDirectoryCheckout:  true,
		DefaultBuildsDir:              "/builds",
		DefaultCacheDir:               "/cache",
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.NormalShell,
			RunnerCommand: "/usr/bin/gitlab-runner-helper",
		},
		ShowHostname: true,
	}

	errIncorrectShellType = fmt.Errorf("kubernetes executor incorrect shell type")

	DefaultResourceIdentifier  = "default"
	resourceTypeServiceAccount = "ServiceAccount"
	resourceTypePullSecret     = "ImagePullSecret"

	defaultLogsBaseDir    = "/logs"
	defaultScriptsBaseDir = "/scripts"

	chars = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	// network errors to retry on
	// make sure to update the documentation in docs/executors/kubernetes/_index.md to keep it in sync
	retryNetworkErrorsGroup = []string{
		"error dialing backend",
		"TLS handshake timeout",
		"read: connection timed out",
		"connect: connection timed out",
		"Timeout occurred",
		"etcdserver: request timed out",
		"http2: client connection lost",
		"connection refused",
		"tls: internal error",

		io.ErrUnexpectedEOF.Error(),
		syscall.ECONNRESET.Error(),
		syscall.ECONNREFUSED.Error(),
		syscall.ECONNABORTED.Error(),
		syscall.EPIPE.Error(),
	}
)

type commandTerminatedError struct {
	exitCode int
}

func (c *commandTerminatedError) Error() string {
	return fmt.Sprintf("command terminated with exit code %d", c.exitCode)
}

func (c *commandTerminatedError) Is(err error) bool {
	_, ok := err.(*commandTerminatedError)
	return ok
}

func (s *executor) NewRetry() *retry.Retry {
	retryLimits := s.Config.Kubernetes.RequestRetryLimits
	retryBackoffConfig := s.getRetryBackoffConfig()

	return retry.New().
		WithCheck(func(_ int, err error) bool {
			for key := range retryLimits {
				if err != nil && strings.Contains(err.Error(), key) {
					return true
				}
			}

			return slices.ContainsFunc(retryNetworkErrorsGroup, func(v string) bool {
				return err != nil && strings.Contains(err.Error(), v)
			})
		}).
		WithMaxTriesFunc(func(err error) int {
			for key, limit := range retryLimits {
				if err != nil && strings.Contains(err.Error(), key) {
					return limit
				}
			}

			return s.Config.Kubernetes.RequestRetryLimit.Get()
		}).
		WithBackoff(retryBackoffConfig.min, retryBackoffConfig.max)
}

type retryBackoffConfig struct {
	min time.Duration
	max time.Duration
}

func (s *executor) getRetryBackoffConfig() retryBackoffConfig {
	return retryBackoffConfig{min: common.RequestRetryBackoffMin, max: s.Config.Kubernetes.RequestRetryBackoffMax.Get()}
}

type podPhaseError struct {
	name  string
	phase api.PodPhase
}

func (p *podPhaseError) Error() string {
	return fmt.Sprintf("pod %q status is %q", p.name, p.phase)
}

type resourceCheckError struct {
	resourceType string
	resourceName string
}

func (r *resourceCheckError) Error() string {
	return fmt.Sprintf(
		"Timed out while waiting for %s/%s to be present in the cluster",
		r.resourceType,
		r.resourceName,
	)
}

func (r *resourceCheckError) Is(err error) bool {
	_, ok := err.(*resourceCheckError)
	return ok
}

type podContainerError struct {
	containerName string
	exitCode      int
	reason        string
}

func (p *podContainerError) Error() string {
	return fmt.Sprintf("Error in container %s: exit code: %d, reason: '%s'", p.containerName, p.exitCode, p.reason)
}

type kubernetesOptions struct {
	Image    spec.Image
	Services map[string]*spec.Image
}

func (kOpts kubernetesOptions) servicesList() spec.Services {
	services := make(spec.Services, len(kOpts.Services))
	for _, name := range slices.Sorted(maps.Keys(kOpts.Services)) {
		services = append(services, *kOpts.Services[name])
	}

	return services
}

func (kOpts kubernetesOptions) getSortedServiceNames() []string {
	return slices.Sorted(maps.Keys(kOpts.Services))
}

type containerBuildOpts struct {
	name               string
	image              string
	imageDefinition    spec.Image
	isServiceContainer bool
	requests           api.ResourceList
	limits             api.ResourceList
	securityContext    *api.SecurityContext
	command            []string
}

type podConfigPrepareOpts struct {
	labels           map[string]string
	annotations      map[string]string
	services         []api.Container
	initContainers   []api.Container
	imagePullSecrets []api.LocalObjectReference
	hostAliases      []api.HostAlias
}

type executor struct {
	executors.AbstractExecutor

	newKubeClient func(config *restclient.Config) (kubernetes.Interface, error)
	kubeClient    kubernetes.Interface

	getKubeConfig func(conf *common.KubernetesConfig, overwrites *overwrites) (*restclient.Config, error)
	kubeConfig    *restclient.Config

	windowsKernelVersion func() string

	pod         *api.Pod
	credentials *api.Secret
	options     *kubernetesOptions
	services    []api.Service

	configurationOverwrites *overwrites
	pullManager             pull.Manager

	helperImageInfo helperimage.Info

	featureChecker featureChecker

	newLogProcessor func() logProcessor

	remoteProcessTerminated chan shells.StageCommandStatus

	requireSharedBuildsDir *bool

	// Flag if a repo mount and emptyDir volume are needed
	requireDefaultBuildsDirVolume *bool

	remoteStageStatusMutex sync.Mutex
	remoteStageStatus      shells.StageCommandStatus

	eventsStream watch.Interface

	podWatcher    podWatcher
	newPodWatcher func(podWatcherConfig) podWatcher

	podEventState *podEventState
}

type podEventState struct {
	lastFetched time.Time
	seen        *expirable.LRU[string, time.Time]
}

type podWatcher interface {
	Start() error
	UpdatePodName(string)
	Stop()
	Errors() <-chan error
}

// podWatcherConfig is configuration for setup of a new pod watcher
type podWatcherConfig struct {
	ctx             context.Context
	logger          *buildlogger.Logger
	kubeClient      kubernetes.Interface
	featureChecker  featureChecker
	namespace       string
	labels          map[string]string
	maxSyncDuration time.Duration
	retryProvider   retry.Provider
}

type serviceCreateResponse struct {
	service *api.Service
	err     error
}

func (s *executor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	s.AbstractExecutor.PrepareConfiguration(options)

	if err = s.prepareOverwrites(options.Build.GetAllVariables()); err != nil {
		return fmt.Errorf("couldn't prepare overwrites: %w", err)
	}

	s.prepareOptions(options.Build)

	if err = s.prepareServiceOverwrites(s.options.Services); err != nil {
		return fmt.Errorf("couldn't prepare explicit service overwrites: %w", err)
	}

	// Dynamically configure use of shared build dir allowing
	// for static build dir when isolated volume is in use.
	s.SharedBuildsDir = s.isSharedBuildsDirRequired()

	if err = s.checkDefaults(); err != nil {
		return fmt.Errorf("check defaults error: %w", err)
	}

	s.kubeConfig, err = s.getKubeConfig(s.Config.Kubernetes, s.configurationOverwrites)
	if err != nil {
		return fmt.Errorf("getting Kubernetes config: %w", err)
	}

	s.kubeClient, err = s.newKubeClient(s.kubeConfig)
	if err != nil {
		return fmt.Errorf("connecting to Kubernetes: %w", err)
	}

	s.helperImageInfo, err = s.prepareHelperImage()
	if err != nil {
		return fmt.Errorf("prepare helper image: %w", err)
	}

	// setup default executor options based on OS type
	s.setupDefaultExecutorOptions(s.helperImageInfo.OSType)

	s.featureChecker = &kubeClientFeatureChecker{s.kubeClient}

	imageName := s.options.Image.Name

	s.BuildLogger.Println("Using Kubernetes executor with image", imageName, "...")
	if !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		s.BuildLogger.Println("Using attach strategy to execute scripts...")
	}

	// pull manager can be prepared once s.options.Image & s.options.Services is set up
	s.pullManager, err = s.preparePullManager()
	if err != nil {
		return err
	}

	if imageNameFromConfig := s.ExpandValue(s.Config.Kubernetes.HelperImage); imageNameFromConfig != "" {
		s.BuildLogger.Debugln(fmt.Sprintf("Using helper image: %s (overridden, default would be %s:%s)", imageNameFromConfig, s.helperImageInfo.Name, s.helperImageInfo.Tag))
	} else {
		s.BuildLogger.Debugln(fmt.Sprintf("Using helper image: %s:%s", s.helperImageInfo.Name, s.helperImageInfo.Tag))
	}

	if err = s.AbstractExecutor.PrepareBuildAndShell(); err != nil {
		return fmt.Errorf("prepare build and shell: %w", err)
	}

	if s.BuildShell.PassFile {
		return fmt.Errorf("kubernetes doesn't support shells that require script file")
	}

	s.podWatcher = s.newPodWatcher(podWatcherConfig{
		ctx:             options.Context,
		logger:          &s.BuildLogger,
		kubeClient:      s.kubeClient,
		featureChecker:  s.featureChecker,
		namespace:       s.configurationOverwrites.namespace,
		labels:          s.buildLabels(),
		maxSyncDuration: s.Config.Kubernetes.RequestRetryBackoffMax.Get(),
		retryProvider:   s,
	})
	if err := s.podWatcher.Start(); err != nil {
		return fmt.Errorf("starting pod watcher: %w", err)
	}

	return s.waitForServices(options.Context)
}

func (s *executor) preparePullManager() (pull.Manager, error) {
	allowedPullPolicies, err := s.Config.Kubernetes.GetAllowedPullPolicies()
	if err != nil {
		return nil, err
	}

	dockerPullPoliciesPerContainer := map[string][]common.DockerPullPolicy{
		buildContainerName:          s.options.Image.PullPolicies,
		helperContainerName:         s.options.Image.PullPolicies,
		initPermissionContainerName: s.options.Image.PullPolicies,
	}
	for containerName, service := range s.options.Services {
		dockerPullPoliciesPerContainer[containerName] = service.PullPolicies
	}

	k8sPullPoliciesPerContainer := map[string][]api.PullPolicy{}
	for containerName, pullPolicies := range dockerPullPoliciesPerContainer {
		k8sPullPolicies, err := s.getPullPolicies(pullPolicies)
		if err != nil {
			return nil, fmt.Errorf("converting pull policy for container %q: %w", containerName, err)
		}

		k8sPullPolicies, err = pull_policies.ComputeEffectivePullPolicies(
			k8sPullPolicies, allowedPullPolicies, pullPolicies, s.Config.Kubernetes.PullPolicy)
		if err != nil {
			return nil, &common.BuildError{
				Inner:         fmt.Errorf("invalid pull policy for container %q: %w", containerName, err),
				FailureReason: common.ConfigurationError,
			}
		}

		s.BuildLogger.Println(fmt.Sprintf("Using effective pull policy of %s for container %s", k8sPullPolicies, containerName))

		k8sPullPoliciesPerContainer[containerName] = k8sPullPolicies
	}

	return pull.NewPullManager(k8sPullPoliciesPerContainer, &s.BuildLogger), nil
}

// getPullPolicies selects the pull_policy configurations originating from
// either gitlab-ci.yaml or config.toml. If present, the pull_policies in
// gitlab-ci.yaml take precedence over those in config.toml.
func (s *executor) getPullPolicies(imagePullPolicies []common.DockerPullPolicy) ([]api.PullPolicy, error) {
	k8sImagePullPolicies, err := s.Config.Kubernetes.ConvertFromDockerPullPolicy(imagePullPolicies)
	if err != nil {
		return nil, fmt.Errorf("conversion to Kubernetes policy: %w", err)
	}

	if len(k8sImagePullPolicies) != 0 {
		return k8sImagePullPolicies, nil
	}

	return s.Config.Kubernetes.GetPullPolicies()
}

func (s *executor) setupDefaultExecutorOptions(os string) {
	if os == helperimage.OSTypeWindows {
		s.DefaultBuildsDir = `C:\builds`
		s.DefaultCacheDir = `C:\cache`

		s.ExecutorOptions.Shell.Shell = shells.SNPowershell
		s.ExecutorOptions.Shell.RunnerCommand = "gitlab-runner-helper"
	}
}

func (s *executor) prepareHelperImage() (helperimage.Info, error) {
	config := s.retrieveHelperImageConfig()

	// use node selector labels to better select the correct image
	if s.Config.Kubernetes.NodeSelector != nil {
		for label, option := range map[string]*string{
			api.LabelArchStable:           &config.Architecture,
			api.LabelOSStable:             &config.OSType,
			nodeSelectorWindowsBuildLabel: &config.KernelVersion,
		} {
			value := s.Config.Kubernetes.NodeSelector[label]
			if value != "" {
				*option = value
			}
		}
	}

	// Also consider node selector overwrites as they may change arch or os
	if s.configurationOverwrites.nodeSelector != nil {
		for label, option := range map[string]*string{
			api.LabelArchStable:           &config.Architecture,
			api.LabelOSStable:             &config.OSType,
			nodeSelectorWindowsBuildLabel: &config.KernelVersion,
		} {
			value := s.configurationOverwrites.nodeSelector[label]
			if value != "" {
				*option = value
			}
		}
	}

	return helperimage.Get(common.AppVersion.Version, config)
}

func (s *executor) retrieveHelperImageConfig() helperimage.Config {
	cfg := helperimage.Config{
		OSType:       helperimage.OSTypeLinux,
		Architecture: "amd64",
		Shell:        s.Config.Shell,
		Flavor:       s.ExpandValue(s.Config.Kubernetes.HelperImageFlavor),
		ProxyExec:    s.Config.IsProxyExec(),
		DisableUmask: s.Build.IsFeatureFlagOn(featureflags.DisableUmaskForKubernetesExecutor),
	}

	if !s.Config.Kubernetes.HelperImageAutosetArchAndOS {
		return cfg
	}

	cfg.Architecture = common.AppVersion.Architecture
	if helperimage.OSTypeWindows == common.AppVersion.OS {
		cfg.OSType = helperimage.OSTypeWindows
		cfg.KernelVersion = s.windowsKernelVersion()
	}

	return cfg
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	for attempt := 1; ; attempt++ {
		var err error

		if s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
			s.BuildLogger.Debugln("Starting Kubernetes command...")
			err = s.runWithExecLegacy(cmd)
		} else {
			s.BuildLogger.Debugln("Starting Kubernetes command with attach...")
			err = s.runWithAttach(cmd)
		}

		if err != nil && s.Config.Kubernetes.GetPrintPodWarningEvents() {
			s.logPodWarningEvents(cmd.Context, k8sEventWarningType)
		}

		var imagePullErr *pull.ImagePullError
		if errors.As(err, &imagePullErr) {
			if s.pullManager.UpdatePolicyForContainer(attempt, imagePullErr) {
				s.cleanupResources()
				s.pod = nil
				continue
			}
		}
		return err
	}
}

func (s *executor) handlePodEvents() error {
	// This will run the watcher only for the first call
	// which is when the pod is being initialized
	if s.eventsStream != nil {
		return nil
	}

	if err := s.watchPodEvents(); err != nil {
		return err
	}

	go s.printPodEvents()
	return nil
}

func (s *executor) watchPodEvents() error {
	s.BuildLogger.Println("Subscribing to Kubernetes Pod events...")
	// Continue polling for the status of the pod as that feels more straightforward than
	// checking for each individual container's status in the events.
	// It also makes it less likely to break something existing since we get the status of the Pod
	// when it's already failed.
	// This strategy can be revised in the future if needed.
	var err error
	s.eventsStream, err = retry.WithValueFn(s, func() (watch.Interface, error) {
		// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
		// kubeAPI: events, watch, FF_PRINT_POD_EVENTS=true
		return s.kubeClient.CoreV1().Events(s.pod.Namespace).Watch(context.Background(), metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", s.pod.Name),
		})
	}).Run()

	return err
}

func (s *executor) printPodEvents() {
	wc := s.BuildLogger.Stream(buildlogger.StreamExecutorLevel, buildlogger.Stderr)
	defer wc.Close()

	w := tabwriter.NewWriter(wc, 3, 1, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "Type\tReason\tMessage")

	// The s.eventsStream.Stop method will be called by the caller
	// that's how we'll exit from this loop
	for result := range s.eventsStream.ResultChan() {
		ev, ok := result.Object.(*api.Event)
		if !ok {
			continue
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", ev.Type, ev.Reason, ev.Message)
		_ = w.Flush()
	}
}

//nolint:gocognit,unparam
func (s *executor) logPodWarningEvents(ctx context.Context, eventType string) {
	if s.pod == nil {
		return
	}

	s.initPodEventState()

	options := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,type=%s", s.pod.Name, eventType),
		Limit:         200,
	}

	var newEvents []*api.Event
	seenDuringFetch := make(map[string]struct{})

	for {
		events, err := retry.WithValueFn(s, func() (*api.EventList, error) {
			//nolint:gocritic
			// kubeAPI: events, list, print_pod_warning_events=true
			return s.kubeClient.CoreV1().Events(s.pod.Namespace).
				List(ctx, options)
		}).Run()
		if err != nil {
			s.BuildLogger.Debugln(fmt.Sprintf("Error retrieving events list: %s", err.Error()))
			return
		}

		for i := range events.Items {
			ev := &events.Items[i]
			timestamp := eventLastOccurredTimestamp(ev)

			// We're filtering events by timestamp to skip older ones.
			// Heads up: there's a small race condition risk here.
			//
			// Under heavy load or K8s leader changes, List() might miss an old event
			// initially, then show it later—skipping it here.
			//
			// Trade-off between performance and full correctness.
			// Misses are super rare, so we prioritize speed:
			//   - Keeps LRU cache small, fewer duplicates
			//
			// Alternative (no filter): 100% complete, but might contain duplicates,
			// might need a bigger cache, and will be slower under high event volume.
			if !s.podEventState.lastFetched.IsZero() && timestamp.Before(s.podEventState.lastFetched) {
				continue
			}

			key := eventKey(ev)

			if s.podEventState.seen.Contains(key) {
				continue
			}

			if _, exists := seenDuringFetch[key]; exists {
				continue
			}
			seenDuringFetch[key] = struct{}{}

			newEvents = append(newEvents, ev)
		}

		if events.Continue == "" {
			break
		}

		options.Continue = events.Continue
	}

	if len(newEvents) == 0 {
		return
	}

	s.logNewPodEvents(newEvents)
}

func (s *executor) initPodEventState() {
	if s.podEventState != nil {
		return
	}

	cache := expirable.NewLRU[string, time.Time](podEventLRUCapacity, nil, podEventStateTTL)

	s.podEventState = &podEventState{seen: cache}
}

func eventLastOccurredTimestamp(e *api.Event) time.Time {
	if e.Series != nil && !e.Series.LastObservedTime.IsZero() {
		return e.Series.LastObservedTime.Time
	}

	// All other below values are for backwards compatibility.
	// Not all objects in Kubernetes currently support Event.Series
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	if !e.EventTime.IsZero() {
		return e.EventTime.Time
	}
	if !e.FirstTimestamp.IsZero() {
		return e.FirstTimestamp.Time
	}
	return e.ObjectMeta.CreationTimestamp.Time
}

func eventKey(e *api.Event) string {
	return fmt.Sprintf("%s:%d", e.UID, e.Count)
}

func (s *executor) logNewPodEvents(events []*api.Event) {
	if len(events) == 0 {
		return
	}

	sort.Slice(events, func(i, j int) bool {
		return eventLastOccurredTimestamp(events[i]).Before(eventLastOccurredTimestamp(events[j]))
	})

	for _, ev := range events {
		s.BuildLogger.Warningln(fmt.Sprintf("Event retrieved from the cluster: %s", ev.Message))

		timestamp := eventLastOccurredTimestamp(ev)
		if timestamp.After(s.podEventState.lastFetched) {
			s.podEventState.lastFetched = timestamp
		}

		s.podEventState.seen.Add(eventKey(ev), timestamp)
	}
}

func (s *executor) runWithExecLegacy(cmd common.ExecutorCommand) error {
	ctx := cmd.Context

	if err := s.setupPodLegacy(ctx); err != nil {
		return err
	}

	containerName := buildContainerName
	containerCommand := s.BuildShell.DockerCommand
	if cmd.Predefined {
		containerName = helperContainerName
		containerCommand = s.helperImageInfo.Cmd
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.BuildLogger.Debugln(fmt.Sprintf(
		"Starting in container %q the command %q with script: %s",
		containerName,
		containerCommand,
		cmd.Script,
	))

	stdout, stderr := s.getExecutorIoWriters()
	defer stdout.Close()
	defer stderr.Close()

	select {
	case err := <-s.runInContainerWithExec(ctx, containerName, containerCommand, cmd.Script, stdout, stderr):
		s.BuildLogger.Debugln(fmt.Sprintf("Container %q exited with error: %v", containerName, err))
		var exitError exec.CodeExitError
		if err != nil && errors.As(err, &exitError) {
			return &common.BuildError{Inner: err, ExitCode: exitError.ExitStatus()}
		}
		return err

	case err := <-s.podWatcher.Errors():
		// if we observe terminal pod errors via the pod watcher, we can exit immediately
		return err

	case <-ctx.Done():
		return fmt.Errorf("build aborted")
	}
}

//nolint:gocognit
func (s *executor) setupPodLegacy(ctx context.Context) error {
	if s.pod != nil {
		return nil
	}

	err := s.setupBuildNamespace(ctx)
	if err != nil {
		return err
	}

	err = s.setupCredentials(ctx)
	if err != nil {
		return err
	}

	initContainers, err := s.buildInitContainers()
	if err != nil {
		return err
	}

	err = s.setupBuildPod(ctx, initContainers)
	if err != nil {
		return err
	}

	if s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
		if err := s.handlePodEvents(); err != nil {
			return err
		}
	}

	if s.Build.IsFeatureFlagOn(featureflags.KubernetesHonorEntrypoint) {
		err := s.captureContainerLogs(ctx, buildContainerName, &entrypointLogForwarder{
			Sink: s.BuildLogger.Stream(buildlogger.StreamExecutorLevel, buildlogger.Stdout),
		})
		if err != nil {
			return err
		}
	}

	var out io.WriteCloser = buildlogger.NewNopCloser(io.Discard)
	if !s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
		out = s.BuildLogger.Stream(buildlogger.StreamExecutorLevel, buildlogger.Stderr)
		defer out.Close()
	}

	if err := s.waitForPod(ctx, out); err != nil {
		return err
	}

	return nil
}

func (s *executor) runWithAttach(cmd common.ExecutorCommand) error {
	ctx := cmd.Context

	err := s.ensurePodsConfigured(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	containerName, containerCommand := s.getContainerInfo(cmd)

	err = s.saveScriptOnEmptyDir(ctx, s.scriptName(string(cmd.Stage)), containerName, cmd.Script)
	if err != nil {
		return err
	}

	s.BuildLogger.Debugln(fmt.Sprintf(
		"Starting in container %q the command %q with script: %s",
		containerName,
		containerCommand,
		cmd.Script,
	))

	podStatusCh := s.watchPodStatus(ctx, &podContainerStatusChecker{
		shouldCheckContainerFilter: func(cs api.ContainerStatus) bool {
			return isNotServiceContainerName(cs.Name)
		},
	})

	select {
	case err := <-s.runInContainer(ctx, cmd.Stage, containerName, containerCommand):
		s.BuildLogger.Debugln(fmt.Sprintf("Container %q exited with error: %v", containerName, err))
		var terminatedError *commandTerminatedError
		if err != nil && errors.As(err, &terminatedError) {
			return &common.BuildError{Inner: err, ExitCode: terminatedError.exitCode}
		}

		return err
	case err := <-podStatusCh:
		if IsKubernetesPodNotFoundError(err) || IsKubernetesPodFailedError(err) || IsKubernetesPodContainerError(err) {
			return err
		}
		return &common.BuildError{Inner: err}

	case err := <-s.podWatcher.Errors():
		// if we observe terminal pod errors via the pod watcher, we can exit immediately
		return err

	case <-ctx.Done():
		s.remoteStageStatusMutex.Lock()
		defer s.remoteStageStatusMutex.Unlock()
		script := s.stageCancellationScript(string(cmd.Stage))
		s.BuildLogger.Debugln("Running job cancellation script:", script)
		if !s.remoteStageStatus.IsExited() {
			err := <-s.runInContainerWithExec(
				s.Context,
				containerName,
				s.BuildShell.DockerCommand,
				script,
				nil, nil,
			)

			s.BuildLogger.Debugln("Job cancellation script exited with error:", err)
		}
		return fmt.Errorf("build aborted")
	}
}

func (s *executor) stageCancellationScript(stage string) string {
	switch s.Shell().Shell {
	case shells.SNPwsh, shells.SNPowershell:
		processIdRetrievalCmd := fmt.Sprintf(
			"(Get-CIMInstance Win32_Process -Filter \"CommandLine LIKE '%%%s%%'\").ProcessId",
			s.scriptName(stage),
		)
		return shells.PowershellStageProcessesKillerScript(processIdRetrievalCmd)
	default:
		// ps command is not available on all unix-like OS
		// To bypass this limitation, we use the following command to search for existing PIDs in the /proc directory
		// Some post processing are then made to only display the process PID and the command line executed.
		searchPIDs := "for prc in /proc/[0-9]*/cmdline; do (printf \"$prc \"; cat -A \"$prc\") | " +
			"sed 's/\\^@/ /g;s|/proc/||;s|/cmdline||'; echo; done"

		// a filtration is made to only keep those related to the ongoing stage script name
		// The subprocess of each PIDs is also retrieve if any.
		return "kill -TERM $(for item in $(" + searchPIDs + " | grep -e '" +
			s.scriptName(stage) +
			" $' | awk '{print $1}'); do test -f /proc/${item}/task/${item}/children" +
			" && cat /proc/${item}/task/${item}/children && echo; done) 2> /dev/null"
	}
}

func (s *executor) ensurePodsConfigured(ctx context.Context) error {
	if s.pod != nil {
		return nil
	}

	if err := s.setupBuildNamespace(ctx); err != nil {
		return fmt.Errorf("setting up build namespace: %w", err)
	}

	if err := s.setupCredentials(ctx); err != nil {
		return fmt.Errorf("setting up credentials: %w", err)
	}

	initContainers, err := s.buildInitContainers()
	if err != nil {
		return err
	}

	if err := s.setupBuildPod(ctx, initContainers); err != nil {
		return fmt.Errorf("setting up build pod: %w", err)
	}

	if s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
		if err := s.handlePodEvents(); err != nil {
			return err
		}
	}

	var out io.WriteCloser = buildlogger.NewNopCloser(io.Discard)
	if !s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
		out = s.BuildLogger.Stream(buildlogger.StreamExecutorLevel, buildlogger.Stderr)
	}
	defer out.Close()

	if err := s.waitForPod(ctx, out); err != nil {
		return err
	}
	out.Close()

	if err := s.setupTrappingScripts(ctx); err != nil {
		return fmt.Errorf("setting up trapping scripts on emptyDir: %w", err)
	}

	// start pulling in logs from the build container, to capture entrypoint logs
	if s.Build.IsFeatureFlagOn(featureflags.KubernetesHonorEntrypoint) {
		err := s.captureContainerLogs(ctx, buildContainerName, &entrypointLogForwarder{
			Sink: s.BuildLogger.Stream(buildlogger.StreamExecutorLevel, buildlogger.Stdout),
		})
		if err != nil {
			return err
		}
	}

	// This starts the log processing, where we run the helper bin (in the helper container) to pull logs from the
	// logfile.
	go s.processLogs(ctx)

	// This pulls the services containers logs directly from the kubeapi and pushes them into the buildlogger.
	s.captureServiceContainersLogs(ctx, s.pod.Spec.Containers)

	return nil
}

func (s *executor) buildInitContainers() ([]api.Container, error) {
	var initContainers []api.Container

	if s.Build.IsFeatureFlagOn(featureflags.DisableUmaskForKubernetesExecutor) {
		uidGidCollectorInitContainer, err := s.buildUiGidCollector(s.helperImageInfo.OSType)
		if err != nil {
			return nil, fmt.Errorf("building umask init container: %w", err)
		}

		initContainers = append(initContainers, uidGidCollectorInitContainer)
	}

	if !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) ||
		(s.Build.IsFeatureFlagOn(featureflags.UseDumbInitWithKubernetesExecutor) &&
			!s.isWindowsJob()) {
		permissionsInitContainer, err := s.buildPermissionsInitContainer(s.helperImageInfo.OSType)
		if err != nil {
			return nil, fmt.Errorf("building permissions init container: %w", err)
		}

		initContainers = append(initContainers, permissionsInitContainer)
	}

	return initContainers, nil
}

func (s *executor) waitForPod(ctx context.Context, writer io.WriteCloser) error {
	status, err := waitForPodRunning(ctx, s.kubeClient, s.pod, writer, s.Config.Kubernetes, buildContainerName)
	if err != nil {
		return fmt.Errorf("waiting for pod running: %w", err)
	}

	if status != api.PodRunning {
		return fmt.Errorf("pod failed to enter running state: %s", status)
	}

	if !s.Build.IsFeatureFlagOn(featureflags.WaitForPodReachable) {
		return nil
	}

	if err := WaitForPodReachable(ctx, s.kubeClient, s.pod, s.Config.Kubernetes); err != nil {
		return fmt.Errorf("pod failed to become attachable %w", err)
	}

	return nil
}

func (s *executor) getContainerInfo(cmd common.ExecutorCommand) (string, []string) {
	var containerCommand []string

	containerName := buildContainerName
	if cmd.Predefined {
		containerName = helperContainerName
	}

	shell := s.Shell().Shell

	switch shell {
	case shells.SNPwsh, shells.SNPowershell:
		// Translates to roughly "/path/to/parse_pwsh_script.ps1 /path/to/stage_script"
		containerCommand = []string{
			s.scriptPath(pwshJSONTerminationScriptName),
			s.scriptPath(cmd.Stage),
			s.buildRedirectionCmd(shell),
		}
	default:
		// Translates to roughly "sh -c '(/detect/shell/path.sh /stage/script/path.sh 2>&1 | tee) &'"
		// which when the detect shell exits becomes something like "bash /stage/script/path.sh".
		// This works unlike "gitlab-runner-build" since the detect shell passes arguments with "$@"
		containerCommand = []string{
			"sh",

			// We have to run the command in a background subshell. Unfortunately,
			// explaining why in a comment fails the code quality check of
			// function length not exceeding 60 lines, so `git blame` this instead.
			"-c",
			fmt.Sprintf("'(%s %s %s) &'",
				s.scriptPath(detectShellScriptName),
				s.scriptPath(cmd.Stage),
				s.buildRedirectionCmd(shell),
			),
		}
		if cmd.Predefined {
			// We use redirection here since the "gitlab-runner-build" helper doesn't pass input args
			// to the shell it executes, so we technically pass the script to the stdin of the underlying shell
			// translates roughly to "gitlab-runner-build <<< /stage/script/path.sh"
			containerCommand = append( //nolint:gocritic
				s.helperImageInfo.Cmd,
				"<<<",
				s.scriptPath(cmd.Stage),
				s.buildRedirectionCmd(shell),
			)
		}
	}

	return containerName, containerCommand
}

func (s *executor) initContainerResources() api.ResourceRequirements {
	resources := api.ResourceRequirements{}

	if s.configurationOverwrites != nil {
		resources.Limits = s.configurationOverwrites.helperLimits
		resources.Requests = s.configurationOverwrites.helperRequests
	}

	return resources
}

func (s *executor) podResourcesReference() *api.ResourceRequirements {
	resources := api.ResourceRequirements{}

	if s.configurationOverwrites != nil {
		resources.Limits = s.configurationOverwrites.podLimits
		resources.Requests = s.configurationOverwrites.podRequests
	}

	return &resources
}

func (s *executor) buildPermissionsInitContainer(os string) (api.Container, error) {
	pullPolicy, err := s.pullManager.GetPullPolicyFor(helperContainerName)
	if err != nil {
		return api.Container{}, fmt.Errorf("getting pull policy for permissions init container: %w", err)
	}

	container := api.Container{
		Name:            initPermissionContainerName,
		Image:           s.getHelperImage(),
		VolumeMounts:    s.getVolumeMounts(),
		ImagePullPolicy: pullPolicy,
		// let's use build container resources
		Resources: s.initContainerResources(),
		SecurityContext: s.Config.Kubernetes.GetContainerSecurityContext(
			s.Config.Kubernetes.InitPermissionsContainerSecurityContext,
			s.defaultCapDrop()...,
		),
	}

	// The kubernetes executor uses both a helper container (for predefined stages) and a build
	// container (for user defined steps). When accessing files on a shared volume, permissions
	// are resolved within the context of the individual container.
	//
	// For Linux, the helper container and build container can occasionally have the same user IDs
	// and access is not a problem. This can occur when:
	// - the image defines a user ID that is identical across both images
	// - PodSecurityContext is used and the UIDs is set manually
	// - Openshift is used and each pod is assigned a different user ID
	// Due to UIDs being different in other scenarios, we explicitly open the permissions on the
	// log shared volume so both containers have access.
	//
	// For Windows, the security identifiers are larger. Unlike Linux, its not likely to have
	// containers share the same identifier. The Windows Security Access Manager is not shared
	// between containers, so we need to open up permissions across more than just the logging
	// shared volume. Fortunately, Windows allows us to set permissions that recursively affect
	// future folders and files.
	switch os {
	case helperimage.OSTypeWindows:
		chmod := "icacls $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%q) /grant 'Everyone:(OI)(CI)F' /q | out-null"
		commands := []string{
			fmt.Sprintf(chmod, s.logsDir()),
			fmt.Sprintf(chmod, s.Build.RootDir),
		}
		container.Command = []string{s.Shell().Shell, "-c", strings.Join(commands, ";\n")}

	default:
		var initCommand []string
		if !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
			initCommand = append(initCommand, fmt.Sprintf("touch %[1]s && (chmod 777 %[1]s || exit 0)", s.logFile()))
		}
		if s.Build.IsFeatureFlagOn(featureflags.UseDumbInitWithKubernetesExecutor) {
			initCommand = append(initCommand, fmt.Sprintf("cp /usr/bin/dumb-init %s", s.scriptsDir()))
		}
		container.Command = []string{"sh", "-c", strings.Join(initCommand, ";\n")}
	}

	return container, nil
}

func (s *executor) buildUiGidCollector(os string) (api.Container, error) {
	const containerName = "init-build-uid-gid-collector"

	kubernetesOptions := s.options.Image.ExecutorOptions.Kubernetes.Expand(s.Build.GetAllVariables())
	securityContext := s.getSecurityContextWithUIDGID(
		string(kubernetesOptions.User),
		containerName,
		s.Config.Kubernetes.BuildContainerSecurityContext,
	)

	opts := containerBuildOpts{
		name:            containerName,
		image:           s.options.Image.Name,
		imageDefinition: s.options.Image,
		requests:        s.configurationOverwrites.buildRequests,
		limits:          s.configurationOverwrites.buildLimits,
		securityContext: securityContext,
	}

	if err := s.verifyAllowedImages(opts); err != nil {
		return api.Container{}, err
	}

	pullPolicy, err := s.pullManager.GetPullPolicyFor(opts.name)
	if err != nil {
		return api.Container{}, err
	}

	container := api.Container{
		Name:            opts.name,
		Image:           opts.image,
		VolumeMounts:    s.getVolumeMounts(),
		ImagePullPolicy: pullPolicy,
		// let's use build container resources
		Resources:       api.ResourceRequirements{Limits: opts.limits, Requests: opts.requests},
		SecurityContext: opts.securityContext,
	}

	if os == helperimage.OSTypeLinux {
		container.Command = []string{"sh", "-c", fmt.Sprintf("> %s/%s", s.RootDir(), shells.BuildUidGidFile)}
	}

	return container, nil
}

func (s *executor) buildRedirectionCmd(shell string) string {
	if shell == shells.SNPowershell {
		// powershell outputs utf16, so we re-encode the output to utf8
		// this is important because our json decoder that detects the exit status
		// of a job requires utf8.Converting command output to strings with %{"$_"}
		// prevents a powershell complaint about native command output on stderr.
		return fmt.Sprintf("2>&1 | %%{ \"$_\" } | Out-File -Append -Encoding UTF8 %s", s.logFile())
	}

	return fmt.Sprintf("2>&1 | tee -a %s", s.logFile())
}

func (s *executor) processLogs(ctx context.Context) {
	processor := s.newLogProcessor()
	logsCh, errCh := processor.Process(ctx)

	// todo: update kubernetes log processor to support separate stdout/stderr streams
	logger := s.BuildLogger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stdout)
	defer logger.Close()

	for {
		select {
		case line, ok := <-logsCh:
			if !ok {
				return
			}
			s.forwardLogLine(logger, line)
		case err, ok := <-errCh:
			if !ok {
				continue
			}

			if err != nil {
				s.BuildLogger.Warningln(fmt.Sprintf("Error processing the log file: %v", err))
			}

			exitCode := getExitCode(err)
			// Script can be kept to nil as not being used after the exitStatus is received
			s.remoteProcessTerminated <- shells.StageCommandStatus{CommandExitCode: &exitCode}
		}
	}
}

func (s *executor) forwardLogLine(w io.Writer, line string) {
	var status shells.StageCommandStatus
	if !status.TryUnmarshal(line) {
		if _, err := w.Write([]byte(line)); err != nil {
			s.BuildLogger.Warningln(fmt.Sprintf("Error writing log line to trace: %v", err))
		}

		return
	}

	s.BuildLogger.Debugln(fmt.Sprintf("Setting remote stage status: %s", status))
	s.remoteStageStatusMutex.Lock()
	s.remoteStageStatus = status
	s.remoteStageStatusMutex.Unlock()

	if status.IsExited() {
		s.remoteProcessTerminated <- status
	}
}

// getExitCode tries to extract the exit code from an inner exec.CodeExitError
// This error may be returned by the underlying kubernetes connection stream
// however it's not guaranteed to be.
// getExitCode would return unknownLogProcessorExitCode if err isn't of type exec.CodeExitError
// or if it's nil
func getExitCode(err error) int {
	var exitErr exec.CodeExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return unknownLogProcessorExitCode
}

func (s *executor) setupTrappingScripts(ctx context.Context) error {
	s.BuildLogger.Debugln("Setting up trapping scripts on emptyDir ...")

	scriptName, script := "", ""
	shellName := s.Shell().Shell
	switch shellName {
	case shells.SNPwsh, shells.SNPowershell:
		scriptName, script = s.scriptName(pwshJSONTerminationScriptName), shells.PwshJSONTerminationScript(shellName)
	default:
		scriptName, script = s.scriptName(detectShellScriptName), shells.BashDetectShellScript
	}

	return s.saveScriptOnEmptyDir(ctx, scriptName, buildContainerName, script)
}

func (s *executor) saveScriptOnEmptyDir(ctx context.Context, scriptName, containerName, script string) error {
	shell, err := s.retrieveShell()
	if err != nil {
		return err
	}

	scriptPath := fmt.Sprintf("%s/%s", s.scriptsDir(), scriptName)
	saveScript, err := shell.GenerateSaveScript(*s.Shell(), scriptPath, script)
	if err != nil {
		return err
	}
	s.BuildLogger.Debugln(fmt.Sprintf("Saving stage script %s on Container %q", saveScript, containerName))

	stdout, stderr := s.getExecutorIoWriters()
	defer stdout.Close()
	defer stderr.Close()

	select {
	case err := <-s.runInContainerWithExec(
		ctx,
		containerName,
		s.BuildShell.DockerCommand,
		saveScript,
		stdout,
		stderr,
	):
		s.BuildLogger.Debugln(fmt.Sprintf("Container %q exited with error: %v", containerName, err))
		var exitError exec.CodeExitError
		if err != nil && errors.As(err, &exitError) {
			return &common.BuildError{Inner: err, ExitCode: exitError.ExitStatus()}
		}
		return err

	case <-ctx.Done():
		return fmt.Errorf("build aborted")
	}
}

func (s *executor) retrieveShell() (common.Shell, error) {
	bashShell, ok := common.GetShell(s.Shell().Shell).(*shells.BashShell)
	if ok {
		return bashShell, nil
	}

	shell := common.GetShell(s.Shell().Shell)
	if shell == nil {
		return nil, errIncorrectShellType
	}

	return shell, nil
}

func (s *executor) Finish(err error) {
	s.podWatcher.Stop()

	if IsKubernetesPodNotFoundError(err) {
		// Avoid an additional error message when trying to
		// cleanup a pod that we know no longer exists
		s.pod = nil
	}

	s.AbstractExecutor.Finish(err)
}

func (s *executor) Cleanup() {
	if s.eventsStream != nil {
		s.eventsStream.Stop()
	}

	s.cleanupResources()
	closeKubeClient(s.kubeClient)
	s.AbstractExecutor.Cleanup()
}

// cleanupResources deletes the resources used during the runner job
// Having a pod does not mean that the owner-dependent relationship exists as an error may occur during setting
// We therefore explicitly delete the resources if no ownerReference is found on it
// This does not apply for services as they are created with the owner from the start
// thus deletion of the pod automatically means deletion of the services if any
func (s *executor) cleanupResources() {
	// Here we don't use the build context as its timeout will prevent a successful cleanup of the resources.
	// The solution used here is inspired from the one used for the docker executor.
	// We give a configurable timeout to complete the resources cleanup.
	ctx, cancel := context.WithTimeout(context.Background(), s.Config.Kubernetes.GetCleanupResourcesTimeout())
	defer cancel()

	if s.pod != nil {
		kubeRequest := retry.WithFn(s, func() error {
			// kubeAPI: pods, delete
			return s.kubeClient.CoreV1().
				Pods(s.pod.Namespace).
				Delete(ctx, s.pod.Name, metav1.DeleteOptions{
					GracePeriodSeconds: s.Config.Kubernetes.CleanupGracePeriodSeconds,
					PropagationPolicy:  &PropagationPolicy,
				})
		})

		if err := kubeRequest.Run(); err != nil {
			s.BuildLogger.Errorln(fmt.Sprintf("Error cleaning up pod: %s", err.Error()))
		}
	}

	if s.credentials != nil && len(s.credentials.OwnerReferences) == 0 {
		kubeRequest := retry.WithFn(s, func() error {
			// kubeAPI: secrets, delete
			return s.kubeClient.CoreV1().
				Secrets(s.configurationOverwrites.namespace).
				Delete(ctx, s.credentials.Name, metav1.DeleteOptions{
					GracePeriodSeconds: s.Config.Kubernetes.CleanupGracePeriodSeconds,
				})
		})
		if err := kubeRequest.Run(); err != nil {
			s.BuildLogger.Errorln(fmt.Sprintf("Error cleaning up secrets: %s", err.Error()))
		}
	}

	err := s.teardownBuildNamespace(ctx)
	if err != nil {
		s.BuildLogger.Errorln(fmt.Sprintf("Error tearing down namespace: %s", err.Error()))
	}
}

func (s *executor) buildContainer(opts containerBuildOpts) (api.Container, error) {
	var envVars []spec.Variable

	if opts.isServiceContainer {
		envVars = s.getServiceVariables(opts.imageDefinition)
	} else if opts.name == buildContainerName {
		envVars = s.Build.GetAllVariables().PublicOrInternal()
	}

	err := s.verifyAllowedImages(opts)
	if err != nil {
		return api.Container{}, err
	}

	containerPorts := make([]api.ContainerPort, len(opts.imageDefinition.Ports))
	proxyPorts := make([]proxy.Port, len(opts.imageDefinition.Ports))

	for i, port := range opts.imageDefinition.Ports {
		proxyPorts[i] = proxy.Port{Name: port.Name, Number: port.Number, Protocol: port.Protocol}
		containerPorts[i] = api.ContainerPort{ContainerPort: int32(port.Number)}
	}

	if len(proxyPorts) > 0 {
		aliases := opts.imageDefinition.Aliases()
		if len(aliases) == 0 {
			if opts.name != buildContainerName {
				aliases = []string{fmt.Sprintf("proxy-%s", opts.name)}
			} else {
				aliases = []string{opts.name}
			}
		}

		for _, serviceName := range aliases {
			s.ProxyPool[serviceName] = s.newProxy(serviceName, proxyPorts)
		}
	}

	pullPolicy, err := s.pullManager.GetPullPolicyFor(opts.name)
	if err != nil {
		return api.Container{}, err
	}

	command, args := s.getCommandAndArgs(opts.imageDefinition, opts.command...)

	container := api.Container{
		Name:            opts.name,
		Image:           opts.image,
		ImagePullPolicy: pullPolicy,
		Command:         command,
		Args:            args,
		Env:             buildVariables(envVars),
		Resources:       api.ResourceRequirements{Limits: opts.limits, Requests: opts.requests},
		Ports:           containerPorts,
		VolumeMounts:    s.getVolumeMounts(),
		SecurityContext: opts.securityContext,
		Lifecycle:       s.prepareLifecycleHooks(),
		Stdin:           true,
	}

	return container, nil
}

func (s *executor) verifyAllowedImages(opts containerBuildOpts) error {
	// check if the image/service is allowed
	internalImages := []string{
		s.ExpandValue(s.Config.Kubernetes.Image),
		s.ExpandValue(s.helperImageInfo.Name),
	}

	var (
		optionName    string
		allowedImages []string
	)
	if opts.isServiceContainer {
		optionName = "services"
		allowedImages = s.Config.Kubernetes.AllowedServices
	} else if opts.name == buildContainerName {
		optionName = "images"
		allowedImages = s.Config.Kubernetes.AllowedImages
	}

	verifyAllowedImageOptions := common.VerifyAllowedImageOptions{
		Image:          opts.image,
		OptionName:     optionName,
		AllowedImages:  allowedImages,
		InternalImages: internalImages,
	}

	return common.VerifyAllowedImage(verifyAllowedImageOptions, s.BuildLogger)
}

func (s *executor) shouldUseStartupProbe() bool {
	honorEntrypoint := s.Build.IsFeatureFlagOn(featureflags.KubernetesHonorEntrypoint)
	legacyExecMode := s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy)

	// For attach mode, the system waits until the entrypoint ran and the shell is spawned anyway, regardless if there is
	// something running as part of the entrypoint or not. For legacy exec mode, we don't wait for the entrypoint (we run
	// a separate process as opposed to attaching to the "main" process), thus we need a signal when the entrypoint is
	// done and the shell is spawned. However, when in exec mode and ignoring the image's entrypoint (thus only starting
	// the shell), we don't really have to wait, either.
	// Thus, in summary: We only need a startupProbe if we run in exec mode *and* honor the image's entrypoint.
	return honorEntrypoint && legacyExecMode
}

func (s *executor) getCommandAndArgs(imageDefinition spec.Image, command ...string) (retCommand []string, retArgs []string) {
	if s.Build.IsFeatureFlagOn(featureflags.KubernetesHonorEntrypoint) {
		return []string{}, command
	}

	if len(command) == 0 && len(imageDefinition.Entrypoint) > 0 {
		command = imageDefinition.Entrypoint
	}

	var args []string
	if len(imageDefinition.Command) > 0 {
		args = imageDefinition.Command
	}

	return command, args
}

func (s *executor) logFile() string {
	return path.Join(s.logsDir(), "output.log")
}

func (s *executor) logsDir() string {
	return s.baseDir(defaultLogsBaseDir,
		s.Config.Kubernetes.LogsBaseDir, s.Build.JobInfo.ProjectID, s.Build.Job.ID)
}

func (s *executor) scriptsDir() string {
	return s.baseDir(defaultScriptsBaseDir,
		s.Config.Kubernetes.ScriptsBaseDir, s.Build.JobInfo.ProjectID, s.Build.Job.ID)
}

func (s *executor) baseDir(defaultBaseDir, configDir string, projectId, jobId int64) string {
	baseDir := defaultBaseDir
	if configDir != "" {
		// if path ends with one or more / or \, drop it
		configDir = strings.TrimRight(configDir, "/\\")
		baseDir = configDir + defaultBaseDir
	}
	return fmt.Sprintf("%s-%d-%d", baseDir, projectId, jobId)
}

func (s *executor) scriptPath(stage common.BuildStage) string {
	return path.Join(s.scriptsDir(), s.scriptName(string(stage)))
}

func (s *executor) scriptName(name string) string {
	shell := s.Shell()
	conf, err := common.GetShell(shell.Shell).GetConfiguration(*shell)
	if err != nil || conf.Extension == "" {
		return name
	}

	return name + "." + conf.Extension
}

func (s *executor) getVolumeMounts() []api.VolumeMount {
	var mounts []api.VolumeMount

	// scripts volumes are needed when using the Kubernetes executor in attach mode
	// FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY = false
	// or when the dumb init is used as it is copied from the helper to this volume
	if s.Build.IsFeatureFlagOn(featureflags.UseDumbInitWithKubernetesExecutor) ||
		!s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		mounts = append(
			mounts,
			api.VolumeMount{
				Name:      "scripts",
				MountPath: s.scriptsDir(),
			})
	}

	if !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		// These volume mounts **MUST NOT** be mounted inside another volume mount.
		// E.g. mounting them inside the "repo" volume mount will cause the whole volume
		// to be owned by root instead of the current user of the image. Something similar
		// is explained here https://github.com/kubernetes/kubernetes/issues/2630#issuecomment-64679120
		// where the first container determines the ownership of a volume. However, it seems like
		// when mounting a volume inside another volume the first container or the first point of contact
		// becomes root, regardless of SecurityContext or Image settings changing the user ID of the container.
		// This causes builds to stop working in environments such as OpenShift where there's no root access
		// resulting in an inability to modify anything inside the parent volume.
		mounts = append(
			mounts,
			api.VolumeMount{
				Name:      "logs",
				MountPath: s.logsDir(),
			})
	}

	mounts = append(mounts, s.getVolumeMountsForConfig()...)

	if s.isDefaultBuildsDirVolumeRequired() {
		mounts = append(mounts, api.VolumeMount{
			Name:      "repo",
			MountPath: s.AbstractExecutor.RootDir(),
		})
	}

	return mounts
}

func (s *executor) getVolumeMountsForConfig() []api.VolumeMount {
	var mounts []api.VolumeMount

	for _, mount := range s.Config.Kubernetes.Volumes.HostPaths {
		mounts = append(mounts, api.VolumeMount{
			Name:             mount.Name,
			MountPath:        s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:          s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:         mount.ReadOnly,
			MountPropagation: (*api.MountPropagationMode)(mount.MountPropagation),
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.Secrets {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.PVCs {
		mounts = append(mounts, api.VolumeMount{
			Name:             s.Build.GetAllVariables().ExpandValue(mount.Name),
			MountPath:        s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:          s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:         mount.ReadOnly,
			MountPropagation: (*api.MountPropagationMode)(mount.MountPropagation),
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.ConfigMaps {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.EmptyDirs {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.CSIs {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: s.Build.GetAllVariables().ExpandValue(mount.MountPath),
			SubPath:   s.Build.GetAllVariables().ExpandValue(mount.SubPath),
			ReadOnly:  mount.ReadOnly,
		})
	}

	return mounts
}

func (s *executor) getVolumes() []api.Volume {
	volumes := s.getVolumesForConfig()

	if s.isDefaultBuildsDirVolumeRequired() {
		volumes = append(volumes, api.Volume{
			Name: "repo",
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{},
			},
		})
	}

	// scripts volumes are needed when using the Kubernetes executor in attach mode
	// FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY = false
	// or when the dumb init is used as it is copied from the helper to this volume
	if s.Build.IsFeatureFlagOn(featureflags.UseDumbInitWithKubernetesExecutor) ||
		!s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		volumes = append(volumes, api.Volume{
			Name: "scripts",
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{},
			},
		})
	}

	if s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		return volumes
	}

	volumes = append(
		volumes,
		api.Volume{
			Name: "logs",
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{},
			},
		})

	return volumes
}

func (s *executor) getVolumesForConfig() []api.Volume {
	var volumes []api.Volume

	volumes = append(volumes, s.getVolumesForHostPaths()...)
	volumes = append(volumes, s.getVolumesForSecrets()...)
	volumes = append(volumes, s.getVolumesForPVCs()...)
	volumes = append(volumes, s.getVolumesForConfigMaps()...)
	volumes = append(volumes, s.getVolumesForEmptyDirs()...)
	volumes = append(volumes, s.getVolumesForCSIs()...)

	return volumes
}

func (s *executor) getVolumesForHostPaths() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.HostPaths {
		path := volume.HostPath
		// Make backward compatible with syntax introduced in version 9.3.0
		if path == "" {
			path = volume.MountPath
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: path,
				},
			},
		})
	}

	return volumes
}

func (s *executor) getVolumesForSecrets() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.Secrets {
		var items []api.KeyToPath
		for key, path := range volume.Items {
			items = append(items, api.KeyToPath{Key: key, Path: path})
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				Secret: &api.SecretVolumeSource{
					SecretName: volume.Name,
					Items:      items,
				},
			},
		})
	}

	return volumes
}

func (s *executor) getVolumesForPVCs() []api.Volume {
	var volumes []api.Volume

	store := make(map[string]api.Volume)

	for _, volume := range s.Config.Kubernetes.Volumes.PVCs {
		if _, found := store[volume.Name]; found {
			continue
		}

		// Resolve the runtime name by injecting variable references.
		resolvedName := s.Build.GetAllVariables().ExpandValue(volume.Name)

		apiVolume := api.Volume{
			Name: resolvedName,
			VolumeSource: api.VolumeSource{
				PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{
					ClaimName: resolvedName,
				},
			},
		}

		volumes = append(volumes, apiVolume)
		store[volume.Name] = apiVolume
	}

	return volumes
}

func (s *executor) getVolumesForConfigMaps() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.ConfigMaps {
		var items []api.KeyToPath
		for key, path := range volume.Items {
			items = append(items, api.KeyToPath{Key: key, Path: path})
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				ConfigMap: &api.ConfigMapVolumeSource{
					LocalObjectReference: api.LocalObjectReference{
						Name: volume.Name,
					},
					Items: items,
				},
			},
		})
	}

	return volumes
}

func (s *executor) getVolumesForEmptyDirs() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.EmptyDirs {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{
					Medium:    api.StorageMedium(volume.Medium),
					SizeLimit: s.parseVolumeSizeLimit(volume),
				},
			},
		})
	}
	return volumes
}

func (s *executor) parseVolumeSizeLimit(volume common.KubernetesEmptyDir) *resource.Quantity {
	if strings.Trim(volume.SizeLimit, " ") == "" {
		return nil
	}

	quantity, err := resource.ParseQuantity(volume.SizeLimit)
	if err != nil {
		s.BuildLogger.Warningln(fmt.Sprintf("invalid limit quantity %q for empty volume %q: %v", volume.SizeLimit, volume.Name, err))
		return nil
	}
	return &quantity
}

func (s *executor) getVolumesForCSIs() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.CSIs {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				CSI: &api.CSIVolumeSource{
					Driver:           volume.Driver,
					FSType:           &volume.FSType,
					ReadOnly:         &volume.ReadOnly,
					VolumeAttributes: volume.VolumeAttributes,
				},
			},
		})
	}
	return volumes
}

func (s *executor) isDefaultBuildsDirVolumeRequired() bool {
	if s.requireDefaultBuildsDirVolume != nil {
		return *s.requireDefaultBuildsDirVolume
	}

	required := true
	for _, mount := range s.getVolumeMountsForConfig() {
		if mount.MountPath == s.AbstractExecutor.RootDir() {
			required = false
			break
		}
	}

	s.requireDefaultBuildsDirVolume = &required

	return required
}

func (s *executor) isSharedBuildsDirRequired() bool {
	// Return quickly when default builds dir is used as job is
	// isolated to pod, so no need for SharedBuildsDir behavior
	if s.isDefaultBuildsDirVolumeRequired() {
		return false
	}

	required := true
	if s.requireSharedBuildsDir != nil {
		return *s.requireSharedBuildsDir
	}

	// Fetch name of the volume backing the builds volume mount
	buildVolumeName := "repo"
	for _, mount := range s.getVolumeMountsForConfig() {
		if mount.MountPath == s.AbstractExecutor.RootDir() {
			buildVolumeName = mount.Name
			break
		}
	}

	// Require shared builds dir when builds dir volume is anything except an emptyDir
	for _, volume := range s.getVolumes() {
		if volume.Name == buildVolumeName && volume.VolumeSource.EmptyDir != nil {
			required = false
			break
		}
	}

	s.requireSharedBuildsDir = &required
	return required
}

func (s *executor) setupCredentials(ctx context.Context) error {
	s.BuildLogger.Debugln("Setting up secrets")

	authConfigs, err := auth.Resolver{}.AllConfigs(s.Build.GetDockerAuthConfig(), s.Shell().User, s.Build.Credentials, &s.BuildLogger)
	if err != nil {
		return err
	}

	if len(authConfigs) == 0 {
		return nil
	}

	dockerCfgs := make(map[string]types.AuthConfig)
	for _, config := range authConfigs {
		dockerCfgs[config.AuthConfig.ServerAddress] = config.AuthConfig
	}

	dockerCfgContent, err := json.Marshal(dockerCfgs)
	if err != nil {
		return err
	}

	secret := api.Secret{}
	secret.Name = generateNameForK8sResources(s.Build.ProjectUniqueName())
	secret.Namespace = s.configurationOverwrites.namespace
	secret.Type = api.SecretTypeDockercfg
	secret.Data = map[string][]byte{}
	secret.Data[api.DockerConfigKey] = dockerCfgContent

	s.credentials, err = retry.WithValueFn(s, func() (*api.Secret, error) {
		return s.requestSecretCreation(ctx, &secret, s.configurationOverwrites.namespace)
	}).Run()
	return err
}

func (s *executor) requestSecretCreation(
	ctx context.Context,
	secret *api.Secret,
	namespace string,
) (*api.Secret, error) {
	// kubeAPI: secrets, create
	creds, err := s.kubeClient.CoreV1().
		Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if isConflict(err) {
		s.BuildLogger.Debugln(
			fmt.Sprintf(
				"Conflict while trying to create the secret  %s ... Retrieving the existing resource",
				secret.Name,
			),
		)

		// kubeAPI: secrets, get
		creds, err = s.kubeClient.CoreV1().
			Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	}

	return creds, err
}

func (s *executor) getHostAliases() ([]api.HostAlias, error) {
	supportsHostAliases, err := s.featureChecker.IsHostAliasSupported()
	switch {
	case errors.Is(err, &badVersionError{}):
		s.BuildLogger.Warningln("Checking for host alias support. Host aliases will be disabled.", err)
		return nil, nil
	case err != nil:
		return nil, err
	case !supportsHostAliases:
		return nil, nil
	}

	return createHostAliases(s.options.servicesList(), s.Config.Kubernetes.GetHostAliases())
}

func (s *executor) setupBuildNamespace(ctx context.Context) error {
	if !s.Config.Kubernetes.NamespacePerJob {
		return nil
	}
	s.BuildLogger.Debugln("Setting up build namespace")

	nsconfig := api.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.configurationOverwrites.namespace,
		},
	}

	//nolint:gocritic
	// kubeAPI: namespaces, create, kubernetes.NamespacePerJob=true
	_, err := s.kubeClient.CoreV1().Namespaces().Create(ctx, &nsconfig, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	return err
}

func (s *executor) teardownBuildNamespace(ctx context.Context) error {
	if !s.Config.Kubernetes.NamespacePerJob {
		return nil
	}

	s.BuildLogger.Debugln("Tearing down build namespace")

	//nolint:gocritic
	// kubeAPI: namespaces, delete, kubernetes.NamespacePerJob=true
	err := s.kubeClient.CoreV1().Namespaces().Delete(ctx, s.configurationOverwrites.namespace, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}
	return nil
}

func (s *executor) setupBuildPod(ctx context.Context, initContainers []api.Container) error {
	s.BuildLogger.Debugln("Setting up build pod")

	prepareOpts, err := s.createPodConfigPrepareOpts(initContainers)
	if err != nil {
		return err
	}

	podConfig, err := s.preparePodConfig(prepareOpts)
	if err != nil {
		return err
	}

	s.BuildLogger.Debugln("Checking for ImagePullSecrets or ServiceAccount existence")
	err = s.checkDependantResources(ctx)
	if err != nil {
		return err
	}

	if s.Build.IsFeatureFlagOn(featureflags.UseAdvancedPodSpecConfiguration) {
		s.BuildLogger.Warningln("Advanced Pod Spec configuration enabled, merging the provided PodSpec to the generated one. " +
			"This is a beta feature and is subject to change. Feedback is collected in this issue: " +
			"https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29659 ...")
		podConfig.Spec, err = s.applyPodSpecMerge(&podConfig.Spec)
		if err != nil {
			return err
		}
	}

	// if we need to retry on pull issues, we need to set the new pod name, so that we don't track terminating pods.
	s.podWatcher.UpdatePodName(podConfig.GetName())

	s.BuildLogger.Debugln("Creating build pod")

	s.pod, err = retry.WithValueFn(s, func() (*api.Pod, error) {
		return s.requestPodCreation(ctx, &podConfig, s.configurationOverwrites.namespace)
	}).Run()
	if err != nil {
		return err
	}

	if data, ok := s.Build.ExecutorData.(*executorData); ok {
		data.PodName = s.pod.GetName()
	}

	ownerReferences := s.buildPodReferences()
	err = s.setOwnerReferencesForResources(ctx, ownerReferences)
	if err != nil {
		return fmt.Errorf("error setting ownerReferences: %w", err)
	}

	s.services, err = s.makePodProxyServices(ctx, ownerReferences)
	return err
}

func (s *executor) requestPodCreation(ctx context.Context, pod *api.Pod, namespace string) (*api.Pod, error) {
	// kubeAPI: pods, create
	p, err := s.kubeClient.CoreV1().
		Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if isConflict(err) {
		s.BuildLogger.Debugln(
			fmt.Sprintf(
				"Conflict while trying to create the pod  %s ... Retrieving the existing resource",
				pod.Name,
			),
		)

		// kubeAPI: pods, get
		p, err = s.kubeClient.CoreV1().
			Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	}

	return p, err
}

func (s *executor) checkDependantResources(ctx context.Context) error {
	if s.Config.Kubernetes.GetResourceAvailabilityCheckMaxAttempts() == 0 {
		s.BuildLogger.Debugln("Resources check has been disabled")
		return nil
	}

	err := s.waitForResource(
		ctx,
		resourceTypeServiceAccount,
		s.Config.Kubernetes.ServiceAccount,
		s.serviceAccountExists(),
	)
	if err != nil {
		return err
	}

	for _, secretName := range s.Config.Kubernetes.ImagePullSecrets {
		err = s.waitForResource(
			ctx,
			resourceTypePullSecret,
			secretName,
			s.secretExists(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *executor) buildLabels() map[string]string {
	// We set default pod labels. These are not allowed to be overwritten.
	labels := map[string]string{
		// Retained for backwards compatibility, may be removed in future release!
		"pod": sanitizeLabel(s.Build.ProjectUniqueName()),

		"project." + runnerLabelNamespace + "/id":             strconv.FormatInt(s.Build.JobInfo.ProjectID, 10),
		"project." + runnerLabelNamespace + "/namespace-id":   sanitizeLabel(s.Build.Variables.Value("CI_PROJECT_NAMESPACE_ID")),
		"project." + runnerLabelNamespace + "/name":           sanitizeLabel(s.Build.JobInfo.ProjectName),
		"project." + runnerLabelNamespace + "/namespace":      sanitizeLabel(s.Build.Variables.Value("CI_PROJECT_NAMESPACE")),
		"project." + runnerLabelNamespace + "/root-namespace": sanitizeLabel(s.Build.Variables.Value("CI_PROJECT_ROOT_NAMESPACE")),

		// Used for setting up services for the build pod
		"job." + runnerLabelNamespace + "/pod": sanitizeLabel(s.Build.ProjectUniqueName()),

		"manager." + runnerLabelNamespace + "/name":     sanitizeLabel(s.Config.Name),
		"manager." + runnerLabelNamespace + "/id-short": sanitizeLabel(s.Config.ShortDescription()),
	}

	safeLabelSetter := func(key, val string) {
		if runnerLabelNamespacePattern.MatchString(key) {
			s.BuildLogger.Debugln(fmt.Sprintf("not setting pod label %q, overwrite of labels in the %q namespace is not allowed", key, runnerLabelNamespace))
			return
		}
		labels[key] = sanitizeLabel(s.Build.Variables.ExpandValue(val))
	}

	for key, val := range s.Build.Runner.Kubernetes.PodLabels {
		safeLabelSetter(key, val)
	}
	for key, val := range s.configurationOverwrites.podLabels {
		safeLabelSetter(key, val)
	}

	return labels
}

func (s *executor) createPodConfigPrepareOpts(initContainers []api.Container) (podConfigPrepareOpts, error) {
	podServices, err := s.preparePodServices()
	if err != nil {
		return podConfigPrepareOpts{}, err
	}

	labels := s.buildLabels()

	annotations := map[string]string{
		"job." + runnerLabelNamespace + "/id":         strconv.FormatInt(s.Build.ID, 10),
		"job." + runnerLabelNamespace + "/url":        s.Build.JobURL(),
		"job." + runnerLabelNamespace + "/sha":        s.Build.GitInfo.Sha,
		"job." + runnerLabelNamespace + "/before_sha": s.Build.GitInfo.BeforeSha,
		"job." + runnerLabelNamespace + "/ref":        s.Build.GitInfo.Ref,
		"job." + runnerLabelNamespace + "/name":       s.Build.JobInfo.Name,
		"job." + runnerLabelNamespace + "/timeout":    s.Build.GetBuildTimeout().String(),
		"project." + runnerLabelNamespace + "/id":     strconv.FormatInt(s.Build.JobInfo.ProjectID, 10),
	}
	for key, val := range s.configurationOverwrites.podAnnotations {
		annotations[key] = s.Build.Variables.ExpandValue(val)
	}

	imagePullSecrets := s.prepareImagePullSecrets()
	hostAliases, err := s.getHostAliases()
	if err != nil {
		return podConfigPrepareOpts{}, err
	}

	return podConfigPrepareOpts{
		labels:           labels,
		annotations:      annotations,
		services:         podServices,
		imagePullSecrets: imagePullSecrets,
		hostAliases:      hostAliases,
		initContainers:   initContainers,
	}, nil
}

func (s *executor) isWindowsJob() bool {
	return s.helperImageInfo.OSType == helperimage.OSTypeWindows
}

func (s *executor) defaultCapDrop() []string {
	if s.isWindowsJob() {
		return nil
	}

	return []string{
		// Reasons for disabling NET_RAW by default were
		// discussed in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26833
		"NET_RAW",
	}
}

func (s *executor) prepareImagePullSecrets() []api.LocalObjectReference {
	if s.Config.Kubernetes.UseServiceAccountImagePullSecrets {
		return nil
	}

	var imagePullSecrets []api.LocalObjectReference
	for _, imagePullSecret := range s.Config.Kubernetes.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: imagePullSecret})
	}

	if s.credentials != nil {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: s.credentials.Name})
	}

	return imagePullSecrets
}

func (s *executor) preparePodServices() ([]api.Container, error) {
	podServices := make([]api.Container, len(s.options.Services))

	for i, name := range s.options.getSortedServiceNames() {
		service := s.options.Services[name]
		kubernetesOptions := service.ExecutorOptions.Kubernetes.Expand(s.Build.GetAllVariables())
		securityContext := s.getSecurityContextWithUIDGID(
			string(kubernetesOptions.User),
			name,
			s.Config.Kubernetes.ServiceContainerSecurityContext,
		)

		var err error
		podServices[i], err = s.buildContainer(containerBuildOpts{
			name:               name,
			image:              service.Name,
			imageDefinition:    *service,
			isServiceContainer: true,
			requests:           s.configurationOverwrites.getServiceResourceRequests(name),
			limits:             s.configurationOverwrites.getServiceResourceLimits(name),
			securityContext:    securityContext,
		})
		if err != nil {
			return nil, err
		}
	}

	return podServices, nil
}

func (s *executor) preparePodConfig(opts podConfigPrepareOpts) (api.Pod, error) {
	buildContainer, helperContainer, err := s.createBuildAndHelperContainers()
	if err != nil {
		return api.Pod{}, err
	}

	pod := api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        generateNameForK8sResources(s.Build.ProjectUniqueName()),
			Namespace:   s.configurationOverwrites.namespace,
			Labels:      opts.labels,
			Annotations: opts.annotations,
		},
		Spec: api.PodSpec{
			Volumes:                      s.getVolumes(),
			SchedulerName:                s.Config.Kubernetes.SchedulerName,
			ServiceAccountName:           s.configurationOverwrites.serviceAccount,
			AutomountServiceAccountToken: s.Config.Kubernetes.AutomountServiceAccountToken,
			RestartPolicy:                api.RestartPolicyNever,
			NodeSelector:                 s.configurationOverwrites.nodeSelector,
			Tolerations:                  s.getPodTolerations(),
			InitContainers:               opts.initContainers,
			Containers: append([]api.Container{
				buildContainer,
				helperContainer,
			}, opts.services...),
			TerminationGracePeriodSeconds: s.Config.Kubernetes.PodTerminationGracePeriodSeconds,
			ActiveDeadlineSeconds:         s.getPodActiveDeadlineSeconds(),
			ImagePullSecrets:              opts.imagePullSecrets,
			SecurityContext:               s.Config.Kubernetes.GetPodSecurityContext(),
			HostAliases:                   opts.hostAliases,
			Affinity:                      s.Config.Kubernetes.GetAffinity(),
			DNSPolicy:                     s.getDNSPolicy(),
			DNSConfig:                     s.Config.Kubernetes.GetDNSConfig(),
			RuntimeClassName:              s.Config.Kubernetes.RuntimeClassName,
			PriorityClassName:             s.Config.Kubernetes.PriorityClassName,
			Resources:                     s.podResourcesReference(),
		},
	}

	return pod, nil
}

// getPodTolerations returns a list of pod tolerations converted from map
// entries in the kubernetes configuration, and possibly from map entries
// generated from job variables, if overwrite is allowed.
func (s *executor) getPodTolerations() []api.Toleration {
	var tolerations []api.Toleration

	for keyvalue, effect := range s.configurationOverwrites.nodeTolerations {
		newToleration := api.Toleration{
			Key:      keyvalue,
			Operator: api.TolerationOpExists,
			Effect:   api.TaintEffect(effect),
		}

		if strings.Contains(keyvalue, "=") {
			parts := strings.SplitN(keyvalue, "=", 2)
			newToleration.Key = parts[0]
			if len(parts) > 1 {
				newToleration.Value = parts[1]
			}
			newToleration.Operator = api.TolerationOpEqual
		}

		tolerations = append(tolerations, newToleration)
	}

	return tolerations
}

// getPodActiveDeadlineSeconds returns the effective build/job timeout
// The feature is behind a FF and return a nil pointer when
// FF_POD_ACTIVE_DEADLINE_SECONDS is disabled
// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29279.
func (s *executor) getPodActiveDeadlineSeconds() *int64 {
	if !s.Build.IsFeatureFlagOn(featureflags.UsePodActiveDeadlineSeconds) {
		return nil
	}

	s.BuildLogger.Println(fmt.Sprintf(
		"Using FF_USE_POD_ACTIVE_DEADLINE_SECONDS, the Pod activeDeadlineSeconds will be set to the job timeout: %v...",
		time.Duration(s.Build.RunnerInfo.Timeout)*time.Second,
	))

	// We do not set the exact timeout as activeDeadlineSeconds
	// 1 second is added to allow the job to timeout on GitLab side
	// before the pod can be marked as failed and the container killed
	timeout := int64(s.Build.RunnerInfo.Timeout + 1)

	return &timeout
}

func (s *executor) createBuildAndHelperContainers() (api.Container, api.Container, error) {
	buildCmd, err := s.getContainerCommand(buildContainerName)
	if err != nil {
		return api.Container{}, api.Container{}, err
	}

	kubernetesOptions := s.options.Image.ExecutorOptions.Kubernetes.Expand(s.Build.GetAllVariables())
	securityContext := s.getSecurityContextWithUIDGID(
		string(kubernetesOptions.User),
		buildContainerName,
		s.Config.Kubernetes.BuildContainerSecurityContext,
	)

	buildContainer, err := s.buildContainer(containerBuildOpts{
		name:            buildContainerName,
		image:           s.options.Image.Name,
		imageDefinition: s.options.Image,
		requests:        s.configurationOverwrites.buildRequests,
		limits:          s.configurationOverwrites.buildLimits,
		securityContext: securityContext,
		command:         buildCmd,
	})
	if err != nil {
		return api.Container{}, api.Container{}, fmt.Errorf("building build container: %w", err)
	}

	helperCmd, err := s.getContainerCommand(helperContainerName)
	if err != nil {
		return api.Container{}, api.Container{}, err
	}
	helperSecurityContext := s.getSecurityContextWithUIDGID(
		"", // Empty user - helper doesn't inherit job user config
		helperContainerName,
		s.Config.Kubernetes.HelperContainerSecurityContext,
	)

	helperContainer, err := s.buildContainer(containerBuildOpts{
		name:            helperContainerName,
		image:           s.getHelperImage(),
		requests:        s.configurationOverwrites.helperRequests,
		limits:          s.configurationOverwrites.helperLimits,
		securityContext: helperSecurityContext,
		command:         helperCmd,
	})
	if err != nil {
		return api.Container{}, api.Container{}, fmt.Errorf("building helper container: %w", err)
	}

	if s.shouldUseStartupProbe() {
		buildContainer.StartupProbe = s.buildContainerStartupProbe()
	}

	return buildContainer, helperContainer, nil
}

func (s *executor) parseAndValidateID(kind, idStr string, validator func(string) error) (int64, error) {
	if idStr == "" {
		return -1, nil
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse %s: %w", kind, err)
	}
	if id < 0 {
		return -1, fmt.Errorf("failed to parse %s: negative values not allowed", kind)
	}
	if err := validator(idStr); err != nil {
		return -1, fmt.Errorf("validating %s: %w", kind, err)
	}

	return id, nil
}

func (s *executor) parseAndValidateUID(uidStr string) (int64, error) {
	return s.parseAndValidateID("UID", uidStr, s.Config.Kubernetes.IsUserAllowed)
}

func (s *executor) parseAndValidateGID(gidStr string) (int64, error) {
	return s.parseAndValidateID("GID", gidStr, s.Config.Kubernetes.IsGroupAllowed)
}

type securityContextIDSource string

const (
	securityContextIDSourceContainer securityContextIDSource = "container security context"
	securityContextIDSourcePod       securityContextIDSource = "pod security context"
	securityContextIDSourceJob       securityContextIDSource = "job configuration"
)

func (s *executor) pickSecurityContextID(containerRunAs, podRunAs *int64, jobRunAs func() int64) (int64, securityContextIDSource) {
	if containerRunAs != nil {
		return *containerRunAs, securityContextIDSourceContainer
	}

	if podRunAs != nil {
		return *podRunAs, securityContextIDSourcePod
	}

	return jobRunAs(), securityContextIDSourceJob
}

func (s *executor) getContainerUIDGID(jobUser, containerName string, containerSecurityContext common.KubernetesContainerSecurityContext) (int64, int64) {
	containerContext := s.Config.Kubernetes.GetContainerSecurityContext(
		containerSecurityContext,
		s.defaultCapDrop()...,
	)
	podContext := s.Config.Kubernetes.GetPodSecurityContext()

	jobUIDStr, jobGIDStr, _ := strings.Cut(jobUser, ":")

	var podRunAsUser, podRunAsGroup *int64
	if podContext != nil {
		podRunAsUser = podContext.RunAsUser
		podRunAsGroup = podContext.RunAsGroup
	}

	uid, uidSource := s.pickSecurityContextID(containerContext.RunAsUser, podRunAsUser, func() int64 {
		uid, err := s.parseAndValidateUID(jobUIDStr)
		if err != nil {
			s.BuildLogger.Warningln(fmt.Sprintf("Error parsing 'uid' from image options for container %q, using the configured security context: %v",
				containerName,
				err,
			))
		}

		return uid
	})

	gid, gidSource := s.pickSecurityContextID(containerContext.RunAsGroup, podRunAsGroup, func() int64 {
		gid, err := s.parseAndValidateGID(jobGIDStr)
		if err != nil {
			s.BuildLogger.Warningln(fmt.Sprintf("Error parsing 'gid' from image options for container %q, using the configured security context: %v",
				containerName,
				err,
			))
		}

		return gid
	})

	if uidSource != securityContextIDSourceJob && jobUIDStr != "" {
		s.BuildLogger.Println(fmt.Sprintf("Overriding user for container %q to %q is not allowed: user is set to %d in %s", containerName, jobUIDStr, uid, uidSource))
	}

	if gidSource != securityContextIDSourceJob && jobGIDStr != "" {
		s.BuildLogger.Println(fmt.Sprintf("Overriding group for container %q to %q is not allowed: group is set to %d in %s", containerName, jobGIDStr, gid, gidSource))
	}

	return uid, gid
}

// getSecurityContextWithUIDGID returns a container security context, where the runAsUser & runAsGroup are set to user provided ones if
//   - the user provided UID/GID is in the allowed list of IDs configured by the admin
//   - the admin hasn't set specific a UID/GID via either the pod security context or the container security context
func (s *executor) getSecurityContextWithUIDGID(jobUser, containerName string, containerSecurityContext common.KubernetesContainerSecurityContext) *api.SecurityContext {
	context := s.Config.Kubernetes.GetContainerSecurityContext(
		containerSecurityContext,
		s.defaultCapDrop()...,
	)

	uid, gid := s.getContainerUIDGID(jobUser, containerName, containerSecurityContext)

	if uid > -1 {
		context.RunAsUser = &uid
	}

	if gid > -1 {
		context.RunAsGroup = &gid
	}

	return context
}

func (s *executor) buildContainerStartupProbe() *api.Probe {
	notUpLog := "gitlab-runner shell not up yet"
	startupProbeFile := s.getStartupProbeFile()
	var probeCommand []string

	switch shell := s.Shell().Shell; shell {
	case shells.SNPwsh, shells.SNPowershell:
		probeCommand = []string{
			shell, "-CommandWithArgs", "if (-Not (Test-Path $args[0] -PathType Leaf)) { $args[1] ; exit 1 }", startupProbeFile, notUpLog,
		}
	default:
		probeCommand = []string{
			shell, "-c", `test -e "$1" || { echo -n "$2"; exit 1; }`, "--", startupProbeFile, notUpLog,
		}
	}

	pollInterval := s.Config.Kubernetes.GetPollInterval()
	pollAttempts := s.Config.Kubernetes.GetPollAttempts()

	return &api.Probe{
		ProbeHandler: api.ProbeHandler{
			Exec: &api.ExecAction{Command: probeCommand},
		},
		SuccessThreshold:    1,
		InitialDelaySeconds: 1,
		PeriodSeconds:       int32(pollInterval),
		FailureThreshold:    int32(pollAttempts),
	}
}

func (s *executor) getStartupProbeFile() string {
	return filepath.Join(s.RootDir(), shells.StartupProbeFile)
}

func (s *executor) getContainerCommand(containerName string) ([]string, error) {
	command := s.BuildShell.DockerCommand

	if s.shouldUseStartupProbe() && containerName == buildContainerName {
		shell, err := s.retrieveShell()
		if err != nil {
			return nil, fmt.Errorf("retrieving shell: %w", err)
		}

		return shell.GetEntrypointCommand(*s.Shell(), s.getStartupProbeFile()), nil
	}

	if !s.Build.IsFeatureFlagOn(featureflags.UseDumbInitWithKubernetesExecutor) {
		return command, nil
	}

	switch s.Shell().Shell {
	case shells.SNPowershell:
		return command, nil
	default:
		return append([]string{fmt.Sprintf("%s/dumb-init", s.scriptsDir()), "--"}, command...), nil
	}
}

// Inspired by
// https://github.com/kubernetes/kubernetes/blob/cde45fb161c5a4bfa7cfe45dfd814f6cc95433f7/cmd/kubeadm/app/util/patches/patches.go#L171
func (s *executor) applyPodSpecMerge(podSpec *api.PodSpec) (api.PodSpec, error) {
	patchedData, err := json.Marshal(podSpec)
	if err != nil {
		return api.PodSpec{}, err
	}

	for _, spec := range s.Config.Kubernetes.PodSpec {
		patchedData, err = doPodSpecMerge(patchedData, spec)
		if err != nil {
			return api.PodSpec{}, err
		}
	}

	var patchedPodSpec api.PodSpec
	err = json.Unmarshal(patchedData, &patchedPodSpec)
	return patchedPodSpec, err
}

func doPodSpecMerge(original []byte, spec common.KubernetesPodSpec) ([]byte, error) {
	var data []byte

	patchBytes, patchType, err := spec.PodSpecPatch()
	if err != nil {
		return nil, err
	}

	switch patchType {
	case common.PatchTypeJSONPatchType:
		var patchObj jsonpatch.Patch
		patchObj, err = jsonpatch.DecodePatch(patchBytes)
		if err == nil {
			data, err = patchObj.Apply(original)
		}

		if err != nil {
			return nil, err
		}
	case common.PatchTypeMergePatchType:
		data, err = jsonpatch.MergePatch(original, patchBytes)
		if err != nil {
			return nil, err
		}
	case common.PatchTypeStrategicMergePatchType:
		data, err = strategicpatch.StrategicMergePatch(
			original,
			patchBytes,
			api.PodSpec{},
		)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported patch type %v", patchType)
	}

	return data, nil
}

func (s *executor) setOwnerReferencesForResources(ctx context.Context, ownerReferences []metav1.OwnerReference) error {
	if s.credentials == nil {
		return nil
	}

	var err error
	s.credentials, err = retry.WithValueFn(s, func() (*api.Secret, error) {
		credentials := s.credentials.DeepCopy()
		credentials.SetOwnerReferences(ownerReferences)

		// kubeAPI: secrets, update
		return s.kubeClient.CoreV1().
			Secrets(s.configurationOverwrites.namespace).
			Update(ctx, credentials, metav1.UpdateOptions{})
	}).Run()

	return err
}

func (s *executor) buildPodReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: apiVersion,
			Kind:       ownerReferenceKind,
			Name:       s.pod.GetName(),
			UID:        s.pod.GetUID(),
		},
	}
}

func (s *executor) waitForResource(
	ctx context.Context,
	resourceType string,
	resourceName string,
	checkExists func(context.Context, string) bool,
) error {
	attempt := -1

	s.BuildLogger.Debugln(fmt.Sprintf("Checking for %s existence", resourceType))

	for attempt < s.Config.Kubernetes.GetResourceAvailabilityCheckMaxAttempts() {
		if checkExists(ctx, resourceName) {
			return nil
		}

		attempt++
		if attempt > 0 {
			s.BuildLogger.Debugln(fmt.Sprintf(
				"Pausing check of the %s availability for %d (attempt %d)",
				resourceType,
				resourceAvailabilityCheckMaxPollInterval,
				attempt,
			))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(resourceAvailabilityCheckMaxPollInterval):
		}
	}

	return &resourceCheckError{
		resourceType: resourceType,
		resourceName: resourceName,
	}
}

func (s *executor) serviceAccountExists() func(context.Context, string) bool {
	return func(ctx context.Context, saName string) bool {
		if saName == "" {
			return true
		}

		return retry.WithFn(s, func() error {
			// NOTE: Casing is important here
			// kubeAPI: serviceaccounts, get
			_, err := s.kubeClient.CoreV1().
				ServiceAccounts(s.configurationOverwrites.namespace).Get(ctx, saName, metav1.GetOptions{})
			return err
		}).Run() == nil
	}
}

func (s *executor) secretExists() func(context.Context, string) bool {
	return func(ctx context.Context, secretName string) bool {
		return retry.WithFn(s, func() error {
			// kubeAPI: secrets, get
			_, err := s.kubeClient.CoreV1().
				Secrets(s.configurationOverwrites.namespace).Get(ctx, secretName, metav1.GetOptions{})
			return err
		}).Run() == nil
	}
}

func (s *executor) getDNSPolicy() api.DNSPolicy {
	dnsPolicy, err := s.Config.Kubernetes.DNSPolicy.Get()
	if err != nil {
		s.BuildLogger.Warningln(fmt.Sprintf("falling back to cluster's default policy: %v", err))
	}
	return dnsPolicy
}

func (s *executor) getHelperImage() string {
	if s.Config.Kubernetes.HelperImage != "" {
		return s.ExpandValue(s.Config.Kubernetes.HelperImage)
	}

	return s.helperImageInfo.String()
}

func (s *executor) makePodProxyServices(
	ctx context.Context,
	ownerReferences []metav1.OwnerReference,
) ([]api.Service, error) {
	s.BuildLogger.Debugln("Creating pod proxy services")

	ch := make(chan serviceCreateResponse)
	var wg sync.WaitGroup
	wg.Add(len(s.ProxyPool))

	for serviceName, serviceProxy := range s.ProxyPool {
		serviceName = dns.MakeRFC1123Compatible(serviceName)
		servicePorts := make([]api.ServicePort, len(serviceProxy.Settings.Ports))
		for i, port := range serviceProxy.Settings.Ports {
			// When there is more than one port Kubernetes requires a port name
			portName := fmt.Sprintf("%s-%d", serviceName, port.Number)
			servicePorts[i] = api.ServicePort{
				Port:       int32(port.Number),
				TargetPort: intstr.FromInt32(int32(port.Number)),
				Name:       portName,
			}
		}

		serviceConfig := s.prepareServiceConfig(serviceName, servicePorts, ownerReferences)
		go s.createKubernetesService(ctx, &serviceConfig, serviceProxy.Settings, ch, &wg)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var proxyServices []api.Service
	for res := range ch {
		if res.err != nil {
			err := fmt.Errorf("error creating the proxy service %q: %w", res.service.Name, res.err)
			s.BuildLogger.Errorln(err)

			return []api.Service{}, err
		}

		proxyServices = append(proxyServices, *res.service)
	}

	return proxyServices, nil
}

func (s *executor) prepareServiceConfig(
	name string,
	ports []api.ServicePort,
	ownerReferences []metav1.OwnerReference,
) api.Service {
	return api.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            generateNameForK8sResources(name),
			Namespace:       s.configurationOverwrites.namespace,
			OwnerReferences: ownerReferences,
		},
		Spec: api.ServiceSpec{
			Ports:    ports,
			Selector: map[string]string{"job." + runnerLabelNamespace + "/pod": sanitizeLabel(s.Build.ProjectUniqueName())},
			Type:     api.ServiceTypeClusterIP,
		},
	}
}

func (s *executor) createKubernetesService(
	ctx context.Context,
	service *api.Service,
	proxySettings *proxy.Settings,
	ch chan<- serviceCreateResponse,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	var err error
	service, err = retry.WithValueFn(s, func() (*api.Service, error) {
		return s.requestServiceCreation(ctx, service, s.pod.Namespace)
	}).Run()

	if err == nil {
		// Updating the internal service name reference and activating the proxy
		proxySettings.ServiceName = service.Name
	}

	ch <- serviceCreateResponse{service: service, err: err}
}

func (s *executor) requestServiceCreation(
	ctx context.Context,
	service *api.Service,
	namespace string,
) (*api.Service, error) {
	// kubeAPI: services, create
	srv, err := s.kubeClient.CoreV1().
		Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if isConflict(err) {
		s.BuildLogger.Debugln(
			fmt.Sprintf(
				"Conflict while trying to create the service  %s ... Retrieving the existing resource",
				service.Name,
			),
		)

		// kubeAPI: services, get
		srv, err = s.kubeClient.CoreV1().
			Services(namespace).Get(ctx, service.Name, metav1.GetOptions{})
	}

	return srv, err
}

func (s *executor) watchPodStatus(ctx context.Context, extendedStatusFunc podStatusChecker) <-chan error {
	// Buffer of 1 in case the context is cancelled while the timer tick case is being executed
	// and the consumer is no longer reading from the channel while we try to write to it
	ch := make(chan error, 1)

	go func() {
		defer close(ch)

		t := time.NewTicker(time.Duration(s.Config.Kubernetes.GetPollInterval()) * time.Second)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				err := s.checkPodStatus(ctx, extendedStatusFunc)
				if err != nil {
					ch <- err
					return
				}
			}
		}
	}()

	return ch
}

// Interface to check if a job pod is unhealthy
type podStatusChecker interface {
	// Checks if a job pod is unhealthy
	check(context.Context, *api.Pod) error
}

// Checks if a pod is unhealthy based on the statuses of its containers
type podContainerStatusChecker struct {
	// Filter to determine which containers to check
	shouldCheckContainerFilter func(api.ContainerStatus) bool
}

func (c *podContainerStatusChecker) check(ctx context.Context, pod *api.Pod) error {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if c.shouldCheckContainerFilter != nil && !c.shouldCheckContainerFilter(containerStatus) {
			continue
		}
		if containerStatus.State.Terminated != nil &&
			containerStatus.State.Terminated.ExitCode >= 0 {
			return &podContainerError{
				containerName: containerStatus.Name,
				exitCode:      int(containerStatus.State.Terminated.ExitCode),
				reason:        containerStatus.State.Terminated.Reason,
			}
		}
	}
	return nil
}

func isNotServiceContainerName(name string) bool {
	return name == buildContainerName || name == helperContainerName
}

func (s *executor) checkPodStatus(ctx context.Context, extendedStatusCheck podStatusChecker) error {
	pod, err := retry.WithValueFn(s, func() (*api.Pod, error) {
		// kubeAPI: pods, get
		return s.kubeClient.CoreV1().
			Pods(s.pod.Namespace).Get(ctx, s.pod.Name, metav1.GetOptions{})
	}).Run()
	if IsKubernetesPodNotFoundError(err) {
		return err
	}

	if err != nil {
		// General request failure
		s.BuildLogger.Warningln("Getting job pod status", err)
		return nil
	}

	if pod.Status.Phase != api.PodRunning {
		return &podPhaseError{
			name:  s.pod.Name,
			phase: pod.Status.Phase,
		}
	}

	return extendedStatusCheck.check(ctx, pod)
}

func (s *executor) runInContainer(
	ctx context.Context,
	stage common.BuildStage,
	name string,
	command []string,
) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		attach := AttachOptions{
			PodName:       s.pod.Name,
			Namespace:     s.pod.Namespace,
			ContainerName: name,
			Command:       command,

			Config:     s.kubeConfig,
			KubeClient: s.kubeClient,
			Executor:   &DefaultRemoteExecutor{},

			Context: ctx,
		}

		kubeRequest := retry.WithFn(s, func() error {
			err := attach.Run()
			s.BuildLogger.Debugln(fmt.Sprintf("Trying to execute stage %v, got error %v", stage, err))
			return s.checkScriptExecution(stage, err)
		})

		if err := kubeRequest.Run(); err != nil {
			errCh <- err
		}

		exitStatus := <-s.remoteProcessTerminated
		s.BuildLogger.Debugln("Remote process exited with the status:", exitStatus)

		// CommandExitCode is guaranteed to be non nil when sent over the remoteProcessTerminated channel
		if *exitStatus.CommandExitCode == 0 {
			errCh <- nil
			return
		}

		errCh <- &commandTerminatedError{exitCode: *exitStatus.CommandExitCode}
	}()

	return errCh
}

func (s *executor) runInContainerWithExec(
	ctx context.Context,
	name string,
	command []string,
	script string,
	stdout, stderr io.WriteCloser,
) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		exec := ExecOptions{
			PodName:       s.pod.Name,
			Namespace:     s.pod.Namespace,
			ContainerName: name,
			Command:       command,
			In:            strings.NewReader(script),
			Out:           stdout,
			Err:           stderr,
			Stdin:         true,
			Config:        s.kubeConfig,
			KubeClient:    s.kubeClient,
			Executor:      &DefaultRemoteExecutor{},

			Context: ctx,
		}

		errCh <- retry.WithFn(s, exec.Run).Run()
	}()

	return errCh
}

func (s *executor) checkScriptExecution(stage common.BuildStage, err error) error {
	// Retrying attach command is a bit different from regular Kubernetes requests.
	// Since the attach commands are executed by openning an HTTP stream to the Kubernetes server
	// and piping the command into that stream and then expecting a response there's no good place to check
	// whether the whole command execution was successful.
	// If we check whether the Stdin stream was read - the connection might have broken up after during transit of that
	// meaning that the command was never executed.
	// It could have also been broken during the reading of the response stream - meaning that it was executed, but we can't know that.
	// The only solution is to check for certain whether the process is already running.
	// For attach that is easy since the process is completely running in the background, and we receive the status of it through
	// the log file and the log processor moves things forward.

	// Non-network errors don't concern this function
	if slices.ContainsFunc(retryNetworkErrorsGroup, func(v string) bool {
		return err != nil && strings.Contains(err.Error(), v)
	}) {
		return err
	}

	s.remoteStageStatusMutex.Lock()
	defer s.remoteStageStatusMutex.Unlock()
	s.BuildLogger.Debugln(fmt.Sprintf("Checking remote stage status after trying attach with err %v. Remote stage status: %v", err, s.remoteStageStatus))

	// If the remote stage is the one we are trying to retry it means that it was already executed.
	s.BuildLogger.Debugln(fmt.Sprintf("Remote stage: %v, trying to execute stage %v", s.remoteStageStatus.BuildStage(), stage))
	if s.remoteStageStatus.BuildStage() == stage {
		return nil
	}

	// If the remote stage is not the same, then we can retry
	return err
}

func (s *executor) prepareOverwrites(variables spec.Variables) error {
	values, err := createOverwrites(s.Config.Kubernetes, variables, s.BuildLogger)
	if err != nil {
		return err
	}

	s.configurationOverwrites = values
	return nil
}

func (s *executor) prepareServiceOverwrites(services map[string]*spec.Image) error {
	for name, service := range services {
		if err := s.configurationOverwrites.evaluateExplicitServiceResourceOverwrite(
			s.Config.Kubernetes,
			name,
			service.Variables,
			s.BuildLogger,
		); err != nil {
			return err
		}
	}

	return nil
}

func (s *executor) prepareOptions(build *common.Build) {
	index := 0
	usedAliases := make(map[string]struct{})
	s.options = &kubernetesOptions{
		Image:    build.Image,
		Services: make(map[string]*spec.Image),
	}

	for _, svc := range s.Config.Kubernetes.GetExpandedServices(s.Build.GetAllVariables()) {
		if svc.Name == "" {
			continue
		}

		serviceName, service := "", svc.ToImageDefinition()
		index, serviceName = s.getServiceDefinition(&service, usedAliases, index)
		s.options.Services[serviceName] = &service
	}

	for _, service := range build.Services {
		if service.Name == "" {
			continue
		}

		serviceName := ""
		index, serviceName = s.getServiceDefinition(&service, usedAliases, index)
		s.options.Services[serviceName] = &service
	}
}

func (s *executor) getServiceDefinition(
	service *spec.Image,
	usedAliases map[string]struct{},
	serviceIndex int,
) (int, string) {
	name := getServiceName(service, usedAliases)
	if name == "" {
		name = fmt.Sprintf("%s%d", serviceContainerPrefix, serviceIndex)
		serviceIndex++
	}

	return serviceIndex, name
}

func getServiceName(svc *spec.Image, usedAliases map[string]struct{}) string {
	for _, alias := range svc.Aliases() {
		if _, ok := usedAliases[alias]; ok {
			continue
		}
		if len(validation.IsDNS1123Label(alias)) != 0 {
			usedAliases[alias] = struct{}{}
			continue
		}

		usedAliases[alias] = struct{}{}
		return alias
	}

	return ""
}

func (s *executor) prepareLifecycleHooks() *api.Lifecycle {
	lifecycleCfg := s.Config.Kubernetes.GetContainerLifecycle()

	if lifecycleCfg.PostStart == nil && lifecycleCfg.PreStop == nil {
		return nil
	}

	lifecycle := &api.Lifecycle{}

	if lifecycleCfg.PostStart != nil {
		lifecycle.PostStart = lifecycleCfg.PostStart.ToKubernetesLifecycleHandler()
	}
	if lifecycleCfg.PreStop != nil {
		lifecycle.PreStop = lifecycleCfg.PreStop.ToKubernetesLifecycleHandler()
	}

	return lifecycle
}

func (s *executor) getServiceVariables(serviceDefinition spec.Image) spec.Variables {
	variables := s.Build.GetAllVariables().PublicOrInternal()
	variables = append(variables, serviceDefinition.Variables...)

	return variables.Expand()
}

// checkDefaults Defines the configuration for the Pod on Kubernetes
func (s *executor) checkDefaults() error {
	if s.options.Image.Name == "" {
		k8sConfigImageName := s.ExpandValue(s.Config.Kubernetes.Image)
		if k8sConfigImageName == "" {
			return fmt.Errorf("no image specified and no default set in config")
		}

		s.BuildLogger.
			WithFields(logrus.Fields{
				"executor": "kubernetes",
				"image":    k8sConfigImageName,
			}).
			Infoln("Using default image")

		s.options.Image.Name = k8sConfigImageName
	}

	if s.Config.Kubernetes.NamespacePerJob {
		s.configurationOverwrites.namespace = fmt.Sprintf("ci-job-%d", s.Build.ID)
	}

	if s.configurationOverwrites.namespace == "" {
		s.BuildLogger.Warningln(
			fmt.Sprintf("Namespace is empty, therefore assuming '%s'.", DefaultResourceIdentifier),
		)
		s.configurationOverwrites.namespace = DefaultResourceIdentifier
	}

	s.BuildLogger.Println("Using Kubernetes namespace:", s.configurationOverwrites.namespace)

	return nil
}

// captureServiceContainersLogs initiates capturing logs for the services containers to a desired additional sink. The
// sink can be any io.Writer. Currently the sink is the jobs main trace, which is wrapped in an inlineServiceLogWriter
// instance to add additional context to logs. In the future this could be separate file.
func (s *executor) captureServiceContainersLogs(ctx context.Context, containers []api.Container) {
	if !s.Build.IsCIDebugServiceEnabled() {
		return
	}

	for _, name := range s.options.getSortedServiceNames() {
		service := s.options.Services[name]
		for _, container := range containers {
			if service.Name != container.Image || isNotServiceContainerName(container.Name) {
				continue
			}

			logger := s.BuildLogger.Stream(buildlogger.StreamStartingServiceLevel, buildlogger.Stdout)
			defer logger.Close()

			aliases := append([]string{strings.Split(container.Image, ":")[0]}, service.Aliases()...)
			sink := service_helpers.NewInlineServiceLogWriter(strings.Join(aliases, "-"), logger)
			if err := s.captureContainerLogs(ctx, container.Name, sink); err != nil {
				s.BuildLogger.Warningln(err.Error())
			}
			logger.Close()
		}
	}
}

// captureContainerLogs tails (i.e. reads) logs emitted to stdout or stderr from
// processes in the specified kubernetes managed container, and redirects them
// to the specified sink, which can be any io.Writer (e.g. this process's
// stdout, a file, a log aggregator). The logs are streamed as they are emitted,
// rather than batched and written when we disconnect from the container (or it
// is stopped). The specified sink is closed when the source is completely
// drained.
func (s *executor) captureContainerLogs(ctx context.Context, containerName string, sink io.WriteCloser) error {
	podLogOpts := api.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		Timestamps: true,
	}

	podLogs, err := retry.WithValueFn(s, func() (io.ReadCloser, error) {
		err := waitForRunningContainer(ctx, s.kubeClient, s.Config.Kubernetes.GetPollTimeout(), s.pod.Namespace, s.pod.Name, containerName)
		if err != nil {
			return nil, err
		}

		// kubeAPI: pods/log, get, list, FF_KUBERNETES_HONOR_ENTRYPOINT=true,FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY=false
		return s.kubeClient.CoreV1().Pods(s.pod.Namespace).GetLogs(s.pod.Name, &podLogOpts).Stream(ctx)
	}).Run()
	if err != nil {
		return fmt.Errorf("failed to open log stream for container %s: %w", containerName, err)
	}

	s.BuildLogger.Debugln("streaming logs for container " + containerName)
	go func() {
		defer podLogs.Close()
		defer sink.Close()

		if _, err = io.Copy(sink, podLogs); err != nil {
			if err != io.EOF && !errors.Is(err, context.Canceled) {
				s.BuildLogger.Warningln(fmt.Sprintf(
					"error streaming logs for container %s: %s",
					containerName,
					err.Error(),
				))
			}
		}
		s.BuildLogger.Debugln("stopped streaming logs for container " + containerName)
	}()
	return nil
}

func generateNameForK8sResources(pattern string) string {
	suffix := make([]rune, k8sResourcesNameSuffixLength)
	for i := range suffix {
		suffix[i] = chars[rand.Intn(len(chars))]
	}

	if len(pattern) > (k8sResourcesNameMaxLength - k8sResourcesNameSuffixLength - 1) {
		pattern = pattern[:k8sResourcesNameMaxLength-k8sResourcesNameSuffixLength-1]
	}

	return fmt.Sprintf("%s-%s", pattern, string(suffix))
}

// When calling the k8s API request, it can happen that despite the failure of the request,
// the resource was actually created. When it comes to POST method, the following retries will get
// a 409 status code (conflits because of the name that must be unique)
// When such status code is received, we stop the retries
func isConflict(err error) bool {
	var statusError *kubeerrors.StatusError
	return errors.As(err, &statusError) &&
		statusError.ErrStatus.Code == http.StatusConflict &&
		strings.Contains(statusError.ErrStatus.Message, errorAlreadyExistsMessage)
}

func IsKubernetesPodNotFoundError(err error) bool {
	var statusErr *kubeerrors.StatusError
	return errors.As(err, &statusErr) &&
		statusErr.ErrStatus.Code == http.StatusNotFound &&
		statusErr.ErrStatus.Details != nil &&
		statusErr.ErrStatus.Details.Kind == "pods"
}

func IsKubernetesPodFailedError(err error) bool {
	var podPhaseErr *podPhaseError
	return errors.As(err, &podPhaseErr) &&
		podPhaseErr.phase == api.PodFailed
}

func IsKubernetesPodContainerError(err error) bool {
	var podServiceError *podContainerError
	return errors.As(err, &podServiceError)
}

// Use 'gitlab-runner check-health' to wait until any/all configured services are healthy.
func (s *executor) waitForServices(ctx context.Context) error {
	portArgs := ""
	for _, name := range s.options.getSortedServiceNames() {
		service := s.options.Services[name]
		port := service.Variables.Get("HEALTHCHECK_TCP_PORT")
		if port == "" {
			continue
		}
		portArgs += fmt.Sprintf("--port '%s' ", port)
	}
	if portArgs == "" {
		return nil
	}
	command := "gitlab-runner-helper health-check " + portArgs

	var err error
	if s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		err = s.setupPodLegacy(ctx)
	} else {
		err = s.ensurePodsConfigured(ctx)
	}
	if err != nil {
		return err
	}

	podStatusCh := s.watchPodStatus(ctx, &podContainerStatusChecker{})

	stdout, stderr := s.getExecutorIoWriters()
	defer stdout.Close()
	defer stderr.Close()

	select {
	case err := <-s.runInContainerWithExec(ctx, helperContainerName, s.BuildShell.DockerCommand, command, stdout, stderr):
		s.BuildLogger.Debugln(fmt.Sprintf("Container helper exited with error: %v", err))
		var exitError exec.CodeExitError
		if err != nil && errors.As(err, &exitError) {
			return &common.BuildError{Inner: err, ExitCode: exitError.ExitStatus()}
		}
	case err := <-podStatusCh:
		s.BuildLogger.Println("Health check aborted due to error: ", err.Error())
		return err

	case <-ctx.Done():
		return fmt.Errorf("health check aborted")
	}

	return nil
}

func (s *executor) getExecutorIoWriters() (io.WriteCloser, io.WriteCloser) {
	return s.BuildLogger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stdout),
		s.BuildLogger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stderr)
}

func newExecutor() *executor {
	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
			Config: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{},
				},
			},
		},
		remoteProcessTerminated: make(chan shells.StageCommandStatus),
		newKubeClient: func(config *restclient.Config) (kubernetes.Interface, error) {
			return kubernetes.NewForConfig(config)
		},
		getKubeConfig:        getKubeClientConfig,
		windowsKernelVersion: os_helpers.LocalKernelVersion,
	}

	type resourceCheckResult struct {
		allowed bool
		reason  string
	}
	e.newPodWatcher = func(c podWatcherConfig) podWatcher {
		gvr := metav1.GroupVersionResource{Version: "v1", Resource: "pods"}
		docLink := "https://docs.gitlab.com/runner/executors/kubernetes/#informers"

		for _, verb := range []string{"list", "watch"} {
			res, err := retry.WithValueFn(c.retryProvider, func() (resourceCheckResult, error) {
				allowed, reason, err := c.featureChecker.IsResourceVerbAllowed(c.ctx, gvr, c.namespace, verb)
				return resourceCheckResult{allowed, reason}, err
			}).Run()
			if res.allowed && err == nil {
				continue
			}
			reason := res.reason
			if err != nil {
				reason = err.Error()
			}
			c.logger.Warningln(fmt.Sprintf("won't use informers: %q, see: %s", reason, docLink))
			return watchers.NoopPodWatcher{}
		}

		return watchers.NewPodWatcher(c.ctx, c.logger, c.kubeClient, c.namespace, c.labels, c.maxSyncDuration)
	}

	e.newLogProcessor = func() logProcessor {
		return newKubernetesLogProcessor(
			e.kubeClient,
			e.kubeConfig,
			&backoff.Backoff{Min: time.Second, Max: 30 * time.Second},
			e.Build.Log(),
			kubernetesLogProcessorPodConfig{
				namespace:          e.pod.Namespace,
				pod:                e.pod.Name,
				container:          helperContainerName,
				logPath:            e.logFile(),
				waitLogFileTimeout: waitLogFileTimeout,
			},
		)
	}

	return e
}

func featuresFn(features *common.FeaturesInfo) {
	features.Artifacts = true
	features.Cache = true
	features.FallbackCacheKeys = true
	features.Image = true
	features.ImageExecutorOpts = true
	features.Proxy = true
	features.ServiceMultipleAliases = true
	features.ServiceVariables = true
	features.Services = true
	features.Session = true
	features.Terminal = true
	features.Variables = true
}

func init() {
	common.RegisterExecutorProvider(common.ExecutorKubernetes, executorProvider{
		DefaultExecutorProvider: executors.DefaultExecutorProvider{
			Creator: func() common.Executor {
				return newExecutor()
			},
			FeaturesUpdater:  featuresFn,
			DefaultShellName: executorOptions.Shell.Shell,
		},
	})
}

package common

import (
	"bufio"
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"time"

	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
	log "github.com/Sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers/ssh"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers/timeperiod"
)

type DockerPullPolicy string

const (
	PullPolicyAlways       = "always"
	PullPolicyNever        = "never"
	PullPolicyIfNotPresent = "if-not-present"

	defaultHelperImage = "gitlab/gitlab-runner-helper"
)

// Get returns one of the predefined values or returns an error if the value can't match the predefined
func (p DockerPullPolicy) Get() (DockerPullPolicy, error) {
	// Default policy is always
	if p == "" {
		return PullPolicyAlways, nil
	}

	// Verify pull policy
	if p != PullPolicyNever &&
		p != PullPolicyIfNotPresent &&
		p != PullPolicyAlways {
		return "", fmt.Errorf("unsupported docker-pull-policy: %v", p)
	}
	return p, nil
}

type DockerConfig struct {
	docker_helpers.DockerCredentials
	Hostname               string           `toml:"hostname,omitempty" json:"hostname" long:"hostname" env:"DOCKER_HOSTNAME" description:"Custom container hostname"`
	Image                  string           `toml:"image" json:"image" long:"image" env:"DOCKER_IMAGE" description:"Docker image to be used"`
	CPUSetCPUs             string           `toml:"cpuset_cpus,omitempty" json:"cpuset_cpus" long:"cpuset-cpus" env:"DOCKER_CPUSET_CPUS" description:"String value containing the cgroups CpusetCpus to use"`
	DNS                    []string         `toml:"dns,omitempty" json:"dns" long:"dns" env:"DOCKER_DNS" description:"A list of DNS servers for the container to use"`
	DNSSearch              []string         `toml:"dns_search,omitempty" json:"dns_search" long:"dns-search" env:"DOCKER_DNS_SEARCH" description:"A list of DNS search domains"`
	Privileged             bool             `toml:"privileged,omitzero" json:"privileged" long:"privileged" env:"DOCKER_PRIVILEGED" description:"Give extended privileges to container"`
	CapAdd                 []string         `toml:"cap_add" json:"cap_add" long:"cap-add" env:"DOCKER_CAP_ADD" description:"Add Linux capabilities"`
	CapDrop                []string         `toml:"cap_drop" json:"cap_drop" long:"cap-drop" env:"DOCKER_CAP_DROP" description:"Drop Linux capabilities"`
	SecurityOpt            []string         `toml:"security_opt" json:"security_opt" long:"security-opt" env:"DOCKER_SECURITY_OPT" description:"Security Options"`
	Devices                []string         `toml:"devices" json:"devices" long:"devices" env:"DOCKER_DEVICES" description:"Add a host device to the container"`
	DisableCache           bool             `toml:"disable_cache,omitzero" json:"disable_cache" long:"disable-cache" env:"DOCKER_DISABLE_CACHE" description:"Disable all container caching"`
	Volumes                []string         `toml:"volumes,omitempty" json:"volumes" long:"volumes" env:"DOCKER_VOLUMES" description:"Bind mount a volumes"`
	VolumeDriver           string           `toml:"volume_driver,omitempty" json:"volume_driver" long:"volume-driver" env:"DOCKER_VOLUME_DRIVER" description:"Volume driver to be used"`
	CacheDir               string           `toml:"cache_dir,omitempty" json:"cache_dir" long:"cache-dir" env:"DOCKER_CACHE_DIR" description:"Directory where to store caches"`
	ExtraHosts             []string         `toml:"extra_hosts,omitempty" json:"extra_hosts" long:"extra-hosts" env:"DOCKER_EXTRA_HOSTS" description:"Add a custom host-to-IP mapping"`
	VolumesFrom            []string         `toml:"volumes_from,omitempty" json:"volumes_from" long:"volumes-from" env:"DOCKER_VOLUMES_FROM" description:"A list of volumes to inherit from another container"`
	NetworkMode            string           `toml:"network_mode,omitempty" json:"network_mode" long:"network-mode" env:"DOCKER_NETWORK_MODE" description:"Add container to a custom network"`
	Links                  []string         `toml:"links,omitempty" json:"links" long:"links" env:"DOCKER_LINKS" description:"Add link to another container"`
	Services               []string         `toml:"services,omitempty" json:"services" long:"services" env:"DOCKER_SERVICES" description:"Add service that is started with container"`
	WaitForServicesTimeout int              `toml:"wait_for_services_timeout,omitzero" json:"wait_for_services_timeout" long:"wait-for-services-timeout" env:"DOCKER_WAIT_FOR_SERVICES_TIMEOUT" description:"How long to wait for service startup"`
	AllowedImages          []string         `toml:"allowed_images,omitempty" json:"allowed_images" long:"allowed-images" env:"DOCKER_ALLOWED_IMAGES" description:"Whitelist allowed images"`
	AllowedServices        []string         `toml:"allowed_services,omitempty" json:"allowed_services" long:"allowed-services" env:"DOCKER_ALLOWED_SERVICES" description:"Whitelist allowed services"`
	PullPolicy             DockerPullPolicy `toml:"pull_policy,omitempty" json:"pull_policy" long:"pull-policy" env:"DOCKER_PULL_POLICY" description:"Image pull policy: never, if-not-present, always"`
}

type DockerMachine struct {
	IdleCount      int      `long:"idle-nodes" env:"MACHINE_IDLE_COUNT" description:"Maximum idle machines"`
	IdleTime       int      `toml:"IdleTime,omitzero" long:"idle-time" env:"MACHINE_IDLE_TIME" description:"Minimum time after node can be destroyed"`
	MaxBuilds      int      `toml:"MaxBuilds,omitzero" long:"max-builds" env:"MACHINE_MAX_BUILDS" description:"Maximum number of builds processed by machine"`
	MachineDriver  string   `long:"machine-driver" env:"MACHINE_DRIVER" description:"The driver to use when creating machine"`
	MachineName    string   `long:"machine-name" env:"MACHINE_NAME" description:"The template for machine name (needs to include %s)"`
	MachineOptions []string `long:"machine-options" env:"MACHINE_OPTIONS" description:"Additional machine creation options"`

	OffPeakPeriods   []string `long:"off-peak-periods" env:"MACHINE_OFF_PEAK_PERIODS" description:"Time periods when the scheduler is in the OffPeak mode"`
	OffPeakTimezone  string   `long:"off-peak-timezone" env:"MACHINE_OFF_PEAK_TIMEZONE" description:"Timezone for the OffPeak periods (defaults to local)"`
	OffPeakIdleCount int      `long:"off-peak-idle-count" env:"MACHINE_OFF_PEAK_IDLE_COUNT" description:"Maximum idle machines when the scheduler is in the OffPeak mode"`
	OffPeakIdleTime  int      `long:"off-peak-idle-time" env:"MACHINE_OFF_PEAK_IDLE_TIME" description:"Minimum time after machine can be destroyed when the scheduler is in the OffPeak mode"`

	offPeakTimePeriods *timeperiod.TimePeriod
}

type ParallelsConfig struct {
	BaseName         string `toml:"base_name" json:"base_name" long:"base-name" env:"PARALLELS_BASE_NAME" description:"VM name to be used"`
	TemplateName     string `toml:"template_name,omitempty" json:"template_name" long:"template-name" env:"PARALLELS_TEMPLATE_NAME" description:"VM template to be created"`
	DisableSnapshots bool   `toml:"disable_snapshots,omitzero" json:"disable_snapshots" long:"disable-snapshots" env:"PARALLELS_DISABLE_SNAPSHOTS" description:"Disable snapshoting to speedup VM creation"`
}

type VirtualBoxConfig struct {
	BaseName         string `toml:"base_name" json:"base_name" long:"base-name" env:"VIRTUALBOX_BASE_NAME" description:"VM name to be used"`
	BaseSnapshot     string `toml:"base_snapshot,omitempty" json:"base_snapshot" long:"base-snapshot" env:"VIRTUALBOX_BASE_SNAPSHOT" description:"Name or UUID of a specific VM snapshot to clone"`
	DisableSnapshots bool   `toml:"disable_snapshots,omitzero" json:"disable_snapshots" long:"disable-snapshots" env:"VIRTUALBOX_DISABLE_SNAPSHOTS" description:"Disable snapshoting to speedup VM creation"`
}

type KubernetesPullPolicy string

// Get returns one of the predefined values in kubernetes notation or returns an error if the value can't match the predefined
func (p KubernetesPullPolicy) Get() (KubernetesPullPolicy, error) {
	switch {
	case p == "":
		return "", nil
	case p == PullPolicyAlways:
		return "Always", nil
	case p == PullPolicyNever:
		return "Never", nil
	case p == PullPolicyIfNotPresent:
		return "IfNotPresent", nil
	}
	return "", fmt.Errorf("unsupported kubernetes-pull-policy: %v", p)
}

type KubernetesConfig struct {
	Host                          string               `toml:"host" json:"host" long:"host" env:"KUBERNETES_HOST" description:"Optional Kubernetes master host URL (auto-discovery attempted if not specified)"`
	CertFile                      string               `toml:"cert_file,omitempty" json:"cert_file" long:"cert-file" env:"KUBERNETES_CERT_FILE" description:"Optional Kubernetes master auth certificate"`
	KeyFile                       string               `toml:"key_file,omitempty" json:"key_file" long:"key-file" env:"KUBERNETES_KEY_FILE" description:"Optional Kubernetes master auth private key"`
	CAFile                        string               `toml:"ca_file,omitempty" json:"ca_file" long:"ca-file" env:"KUBERNETES_CA_FILE" description:"Optional Kubernetes master auth ca certificate"`
	Image                         string               `toml:"image" json:"image" long:"image" env:"KUBERNETES_IMAGE" description:"Default docker image to use for builds when none is specified"`
	Namespace                     string               `toml:"namespace" json:"namespace" long:"namespace" env:"KUBERNETES_NAMESPACE" description:"Namespace to run Kubernetes jobs in"`
	NamespaceOverwriteAllowed     string               `toml:"namespace_overwrite_allowed" json:"namespace_overwrite_allowed" long:"namespace_overwrite_allowed" env:"KUBERNETES_NAMESPACE_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_NAMESPACE_OVERWRITE' value"`
	Privileged                    bool                 `toml:"privileged,omitzero" json:"privileged" long:"privileged" env:"KUBERNETES_PRIVILEGED" description:"Run all containers with the privileged flag enabled"`
	CPUs                          string               `toml:"cpus,omitempty" json:"cpus" long:"cpus" env:"KUBERNETES_CPUS" description:"(deprecated) The CPU allocation given to build containers"`
	Memory                        string               `toml:"memory,omitempty" json:"memory" long:"memory" env:"KUBERNETES_MEMORY" description:"(deprecated) The amount of memory allocated to build containers"`
	ServiceCPUs                   string               `toml:"service_cpus,omitempty" json:"service_cpus" long:"service-cpus" env:"KUBERNETES_SERVICE_CPUS" description:"(deprecated) The CPU allocation given to build service containers"`
	ServiceMemory                 string               `toml:"service_memory,omitempty" json:"service_memory" long:"service-memory" env:"KUBERNETES_SERVICE_MEMORY" description:"(deprecated) The amount of memory allocated to build service containers"`
	HelperCPUs                    string               `toml:"helper_cpus,omitempty" json:"helper_cpus" long:"helper-cpus" env:"KUBERNETES_HELPER_CPUS" description:"(deprecated) The CPU allocation given to build helper containers"`
	HelperMemory                  string               `toml:"helper_memory,omitempty" json:"helper_memory" long:"helper-memory" env:"KUBERNETES_HELPER_MEMORY" description:"(deprecated) The amount of memory allocated to build helper containers"`
	CPULimit                      string               `toml:"cpu_limit,omitempty" json:"cpu_limit" long:"cpu-limit" env:"KUBERNETES_CPU_LIMIT" description:"The CPU allocation given to build containers"`
	MemoryLimit                   string               `toml:"memory_limit,omitempty" json:"memory_limit" long:"memory-limit" env:"KUBERNETES_MEMORY_LIMIT" description:"The amount of memory allocated to build containers"`
	ServiceCPULimit               string               `toml:"service_cpu_limit,omitempty" json:"service_cpu_limit" long:"service-cpu-limit" env:"KUBERNETES_SERVICE_CPU_LIMIT" description:"The CPU allocation given to build service containers"`
	ServiceMemoryLimit            string               `toml:"service_memory_limit,omitempty" json:"service_memory_limit" long:"service-memory-limit" env:"KUBERNETES_SERVICE_MEMORY_LIMIT" description:"The amount of memory allocated to build service containers"`
	HelperCPULimit                string               `toml:"helper_cpu_limit,omitempty" json:"helper_cpu_limit" long:"helper-cpu-limit" env:"KUBERNETES_HELPER_CPU_LIMIT" description:"The CPU allocation given to build helper containers"`
	HelperMemoryLimit             string               `toml:"helper_memory_limit,omitempty" json:"helper_memory_limit" long:"helper-memory-limit" env:"KUBERNETES_HELPER_MEMORY_LIMIT" description:"The amount of memory allocated to build helper containers"`
	CPURequest                    string               `toml:"cpu_request,omitempty" json:"cpu_request" long:"cpu-request" env:"KUBERNETES_CPU_REQUEST" description:"The CPU allocation requested for build containers"`
	MemoryRequest                 string               `toml:"memory_request,omitempty" json:"memory_request" long:"memory-request" env:"KUBERNETES_MEMORY_REQUEST" description:"The amount of memory requested from build containers"`
	ServiceCPURequest             string               `toml:"service_cpu_request,omitempty" json:"service_cpu_request" long:"service-cpu-request" env:"KUBERNETES_SERVICE_CPU_REQUEST" description:"The CPU allocation requested for build service containers"`
	ServiceMemoryRequest          string               `toml:"service_memory_request,omitempty" json:"service_memory_request" long:"service-memory-request" env:"KUBERNETES_SERVICE_MEMORY_REQUEST" description:"The amount of memory requested for build service containers"`
	HelperCPURequest              string               `toml:"helper_cpu_request,omitempty" json:"helper_cpu_request" long:"helper-cpu-request" env:"KUBERNETES_HELPER_CPU_REQUEST" description:"The CPU allocation requested for build helper containers"`
	HelperMemoryRequest           string               `toml:"helper_memory_request,omitempty" json:"helper_memory_request" long:"helper-memory-request" env:"KUBERNETES_HELPER_MEMORY_REQUEST" description:"The amount of memory requested for build helper containers"`
	PullPolicy                    KubernetesPullPolicy `toml:"pull_policy,omitempty" json:"pull_policy" long:"pull-policy" env:"KUBERNETES_PULL_POLICY" description:"Policy for if/when to pull a container image (never, if-not-present, always). The cluster default will be used if not set"`
	NodeSelector                  map[string]string    `toml:"node_selector,omitempty" json:"node_selector" long:"node-selector" description:"A toml table/json object of key=value. Value is expected to be a string. When set this will create pods on k8s nodes that match all the key=value pairs."`
	ImagePullSecrets              []string             `toml:"image_pull_secrets,omitempty" json:"image_pull_secrets" long:"image-pull-secrets" env:"KUBERNETES_IMAGE_PULL_SECRETS" description:"A list of image pull secrets that are used for pulling docker image"`
	HelperImage                   string               `toml:"helper_image,omitempty" json:"helper_image" long:"helper-image" env:"KUBERNETES_HELPER_IMAGE" description:"[ADVANCED] Override the default helper image used to clone repos and upload artifacts"`
	TerminationGracePeriodSeconds int64                `toml:"terminationGracePeriodSeconds,omitzero" json:"terminationGracePeriodSeconds" long:"terminationGracePeriodSeconds" env:"KUBERNETES_TERMINATIONGRACEPERIODSECONDS" description:"Duration after the processes running in the pod are sent a termination signal and the time when the processes are forcibly halted with a kill signal."`
	PollInterval                  int                  `toml:"poll_interval,omitzero" json:"poll_interval" long:"poll-interval" env:"KUBERNETES_POLL_INTERVAL" description:"How frequently, in seconds, the runner will poll the Kubernetes pod it has just created to check its status"`
	PollTimeout                   int                  `toml:"poll_timeout,omitzero" json:"poll_timeout" long:"poll-timeout" env:"KUBERNETES_POLL_TIMEOUT" description:"The total amount of time, in seconds, that needs to pass before the runner will timeout attempting to connect to the pod it has just created (useful for queueing more builds that the cluster can handle at a time)"`
}

type RunnerCredentials struct {
	URL       string `toml:"url" json:"url" short:"u" long:"url" env:"CI_SERVER_URL" required:"true" description:"Runner URL"`
	Token     string `toml:"token" json:"token" short:"t" long:"token" env:"CI_SERVER_TOKEN" required:"true" description:"Runner token"`
	TLSCAFile string `toml:"tls-ca-file,omitempty" json:"tls-ca-file" long:"tls-ca-file" env:"CI_SERVER_TLS_CA_FILE" description:"File containing the certificates to verify the peer when using HTTPS"`
}

type CacheConfig struct {
	Type           string `toml:"Type,omitempty" long:"type" env:"CACHE_TYPE" description:"Select caching method: s3, to use S3 buckets"`
	ServerAddress  string `toml:"ServerAddress,omitempty" long:"s3-server-address" env:"S3_SERVER_ADDRESS" description:"A host:port to the used S3-compatible server"`
	AccessKey      string `toml:"AccessKey,omitempty" long:"s3-access-key" env:"S3_ACCESS_KEY" description:"S3 Access Key"`
	SecretKey      string `toml:"SecretKey,omitempty" long:"s3-secret-key" env:"S3_SECRET_KEY" description:"S3 Secret Key"`
	BucketName     string `toml:"BucketName,omitempty" long:"s3-bucket-name" env:"S3_BUCKET_NAME" description:"Name of the bucket where cache will be stored"`
	BucketLocation string `toml:"BucketLocation,omitempty" long:"s3-bucket-location" env:"S3_BUCKET_LOCATION" description:"Name of S3 region"`
	Insecure       bool   `toml:"Insecure,omitempty" long:"s3-insecure" env:"S3_CACHE_INSECURE" description:"Use insecure mode (without https)"`
	Path           string `toml:"Path,omitempty" long:"s3-cache-path" env:"S3_CACHE_PATH" description:"Name of the path to prepend to the cache URL"`
	Shared         bool   `toml:"Shared,omitempty" long:"cache-shared" env:"CACHE_SHARED" description:"Enable cache sharing between runners."`
}

type RunnerSettings struct {
	Executor  string `toml:"executor" json:"executor" long:"executor" env:"RUNNER_EXECUTOR" required:"true" description:"Select executor, eg. shell, docker, etc."`
	BuildsDir string `toml:"builds_dir,omitempty" json:"builds_dir" long:"builds-dir" env:"RUNNER_BUILDS_DIR" description:"Directory where builds are stored"`
	CacheDir  string `toml:"cache_dir,omitempty" json:"cache_dir" long:"cache-dir" env:"RUNNER_CACHE_DIR" description:"Directory where build cache is stored"`

	Environment     []string `toml:"environment,omitempty" json:"environment" long:"env" env:"RUNNER_ENV" description:"Custom environment variables injected to build environment"`
	PreCloneScript  string   `toml:"pre_clone_script,omitempty" json:"pre_clone_script" long:"pre-clone-script" env:"RUNNER_PRE_CLONE_SCRIPT" description:"Runner-specific command script executed before code is pulled"`
	PreBuildScript  string   `toml:"pre_build_script,omitempty" json:"pre_build_script" long:"pre-build-script" env:"RUNNER_PRE_BUILD_SCRIPT" description:"Runner-specific command script executed after code is pulled, just before build executes"`
	PostBuildScript string   `toml:"post_build_script,omitempty" json:"post_build_script" long:"post-build-script" env:"RUNNER_POST_BUILD_SCRIPT" description:"Runner-specific command script executed after code is pulled and just after build executes"`

	Shell string `toml:"shell,omitempty" json:"shell" long:"shell" env:"RUNNER_SHELL" description:"Select bash, cmd or powershell"`

	SSH        *ssh.Config       `toml:"ssh,omitempty" json:"ssh" group:"ssh executor" namespace:"ssh"`
	Docker     *DockerConfig     `toml:"docker,omitempty" json:"docker" group:"docker executor" namespace:"docker"`
	Parallels  *ParallelsConfig  `toml:"parallels,omitempty" json:"parallels" group:"parallels executor" namespace:"parallels"`
	VirtualBox *VirtualBoxConfig `toml:"virtualbox,omitempty" json:"virtualbox" group:"virtualbox executor" namespace:"virtualbox"`
	Cache      *CacheConfig      `toml:"cache,omitempty" json:"cache" group:"cache configuration" namespace:"cache"`
	Machine    *DockerMachine    `toml:"machine,omitempty" json:"machine" group:"docker machine provider" namespace:"machine"`
	Kubernetes *KubernetesConfig `toml:"kubernetes,omitempty" json:"kubernetes" group:"kubernetes executor" namespace:"kubernetes"`
}

type RunnerConfig struct {
	Name        string `toml:"name" json:"name" short:"name" long:"description" env:"RUNNER_NAME" description:"Runner name"`
	Limit       int    `toml:"limit,omitzero" json:"limit" long:"limit" env:"RUNNER_LIMIT" description:"Maximum number of builds processed by this runner"`
	OutputLimit int    `toml:"output_limit,omitzero" long:"output-limit" env:"RUNNER_OUTPUT_LIMIT" description:"Maximum build trace size in kilobytes"`

	RunnerCredentials
	RunnerSettings
}

type Config struct {
	MetricsServerAddress string          `toml:"metrics_server,omitempty" json:"metrics_server"`
	Concurrent           int             `toml:"concurrent" json:"concurrent"`
	CheckInterval        int             `toml:"check_interval" json:"check_interval" description:"Define active checking interval of jobs"`
	User                 string          `toml:"user,omitempty" json:"user"`
	Runners              []*RunnerConfig `toml:"runners" json:"runners"`
	SentryDSN            *string         `toml:"sentry_dsn"`
	ModTime              time.Time       `toml:"-"`
	Loaded               bool            `toml:"-"`
}

func (c *KubernetesConfig) GetHelperImage() string {
	if len(c.HelperImage) > 0 {
		return c.HelperImage
	}

	rev := REVISION
	if rev == "HEAD" {
		rev = "latest"
	}

	return fmt.Sprintf("%s:x86_64-%s", defaultHelperImage, rev)
}

func (c *KubernetesConfig) GetPollAttempts() int {
	if c.PollTimeout <= 0 {
		c.PollTimeout = KubernetesPollTimeout
	}

	return c.PollTimeout / c.GetPollInterval()
}

func (c *KubernetesConfig) GetPollInterval() int {
	if c.PollInterval <= 0 {
		c.PollInterval = KubernetesPollInterval
	}

	return c.PollInterval
}

func (c *DockerMachine) GetIdleCount() int {
	if c.isOffPeak() {
		return c.OffPeakIdleCount
	}

	return c.IdleCount
}

func (c *DockerMachine) GetIdleTime() int {
	if c.isOffPeak() {
		return c.OffPeakIdleTime
	}

	return c.IdleTime
}

func (c *DockerMachine) isOffPeak() bool {
	if c.offPeakTimePeriods == nil {
		c.CompileOffPeakPeriods()
	}

	return c.offPeakTimePeriods != nil && c.offPeakTimePeriods.InPeriod()
}

func (c *DockerMachine) CompileOffPeakPeriods() (err error) {
	c.offPeakTimePeriods, err = timeperiod.TimePeriods(c.OffPeakPeriods, c.OffPeakTimezone)
	if err != nil {
		err = errors.New(fmt.Sprint("Invalid OffPeakPeriods value: ", err))
	}

	return
}

func (c *RunnerCredentials) ShortDescription() string {
	return helpers.ShortenToken(c.Token)
}

func (c *RunnerCredentials) UniqueID() string {
	return c.URL + c.Token
}

func (c *RunnerCredentials) Log() *log.Entry {
	if c.ShortDescription() != "" {
		return log.WithField("runner", c.ShortDescription())
	}
	return log.WithFields(log.Fields{})
}

func (c *RunnerConfig) String() string {
	return fmt.Sprintf("%v url=%v token=%v executor=%v", c.Name, c.URL, c.Token, c.Executor)
}

func (c *RunnerConfig) GetVariables() BuildVariables {
	var variables BuildVariables

	for _, environment := range c.Environment {
		if variable, err := ParseVariable(environment); err == nil {
			variable.Internal = true
			variables = append(variables, variable)
		}
	}

	return variables
}

func NewConfig() *Config {
	return &Config{
		Concurrent: 1,
	}
}

func (c *Config) StatConfig(configFile string) error {
	_, err := os.Stat(configFile)
	if err != nil {
		return err
	}
	return nil
}

func (c *Config) LoadConfig(configFile string) error {
	info, err := os.Stat(configFile)

	// permission denied is soft error
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	if _, err = toml.DecodeFile(configFile, c); err != nil {
		return err
	}

	for _, runner := range c.Runners {
		if runner.Machine == nil {
			continue
		}

		err := runner.Machine.CompileOffPeakPeriods()
		if err != nil {
			return err
		}
	}

	c.ModTime = info.ModTime()
	c.Loaded = true
	return nil
}

func (c *Config) SaveConfig(configFile string) error {
	var newConfig bytes.Buffer
	newBuffer := bufio.NewWriter(&newConfig)

	if err := toml.NewEncoder(newBuffer).Encode(c); err != nil {
		log.Fatalf("Error encoding TOML: %s", err)
		return err
	}

	if err := newBuffer.Flush(); err != nil {
		return err
	}

	// create directory to store configuration
	os.MkdirAll(filepath.Dir(configFile), 0700)

	// write config file
	if err := ioutil.WriteFile(configFile, newConfig.Bytes(), 0600); err != nil {
		return err
	}

	c.Loaded = true
	return nil
}

func (c *Config) GetCheckInterval() time.Duration {
	if c.CheckInterval > 0 {
		return time.Duration(c.CheckInterval) * time.Second
	}
	return CheckInterval
}

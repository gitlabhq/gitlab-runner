package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/BurntSushi/toml"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/timeperiod"
	"gitlab.com/gitlab-org/gitlab-runner/referees"
)

type DockerPullPolicy string
type DockerSysCtls map[string]string

type KubernetesHookHandlerType string

const (
	PullPolicyAlways       = "always"
	PullPolicyNever        = "never"
	PullPolicyIfNotPresent = "if-not-present"

	DNSPolicyNone                    KubernetesDNSPolicy = "none"
	DNSPolicyDefault                 KubernetesDNSPolicy = "default"
	DNSPolicyClusterFirst            KubernetesDNSPolicy = "cluster-first"
	DNSPolicyClusterFirstWithHostNet KubernetesDNSPolicy = "cluster-first-with-host-net"

	GenerateArtifactsMetadataVariable = "RUNNER_GENERATE_ARTIFACTS_METADATA"
)

// InvalidTimePeriodsError represents that the time period specified is not valid.
type InvalidTimePeriodsError struct {
	periods []string
	cause   error
}

func NewInvalidTimePeriodsError(periods []string, cause error) *InvalidTimePeriodsError {
	return &InvalidTimePeriodsError{periods: periods, cause: cause}
}

func (e *InvalidTimePeriodsError) Error() string {
	return fmt.Sprintf("invalid time periods %v, caused by: %v", e.periods, e.cause)
}

func (e *InvalidTimePeriodsError) Is(err error) bool {
	_, ok := err.(*InvalidTimePeriodsError)

	return ok
}

func (e *InvalidTimePeriodsError) Unwrap() error {
	return e.cause
}

// GetPullPolicies returns a validated list of pull policies, falling back to a predefined value if empty,
// or returns an error if the list is not valid
func (c DockerConfig) GetPullPolicies() ([]DockerPullPolicy, error) {
	// Default policy is always
	if len(c.PullPolicy) == 0 {
		return []DockerPullPolicy{PullPolicyAlways}, nil
	}

	// Verify pull policies
	policies := make([]DockerPullPolicy, len(c.PullPolicy))
	for idx, p := range c.PullPolicy {
		switch p {
		case PullPolicyAlways, PullPolicyIfNotPresent, PullPolicyNever:
			policies[idx] = DockerPullPolicy(p)
		default:
			return []DockerPullPolicy{}, fmt.Errorf("unsupported pull_policy config: %q", p)
		}
	}

	return policies, nil
}

// GetAllowedPullPolicies returns a validated list of allowed pull policies,
// falling back to a predefined value if empty, or returns an error if the list is not valid
func (c DockerConfig) GetAllowedPullPolicies() ([]DockerPullPolicy, error) {
	if len(c.AllowedPullPolicies) == 0 {
		return c.GetPullPolicies()
	}

	// Verify allowed pull policies
	policies := make([]DockerPullPolicy, len(c.AllowedPullPolicies))
	for idx, p := range c.AllowedPullPolicies {
		switch p {
		case PullPolicyAlways, PullPolicyIfNotPresent, PullPolicyNever:
			policies[idx] = p
		default:
			return []DockerPullPolicy{}, fmt.Errorf("unsupported allowed_pull_policies config: %q", p)
		}
	}

	return policies, nil
}

func (c KubernetesConfig) GetAllowedPullPolicies() ([]api.PullPolicy, error) {
	if len(c.AllowedPullPolicies) == 0 {
		return c.GetPullPolicies()
	}

	// Verify allowed pull policies
	pullPolicies, err := c.ConvertFromDockerPullPolicy(c.AllowedPullPolicies)
	if err != nil {
		return nil, fmt.Errorf("allowed_pull_policies config: %w", err)
	}

	return pullPolicies, nil
}

// StringOrArray implements UnmarshalTOML to unmarshal either a string or array of strings.
type StringOrArray []string

func (p *StringOrArray) UnmarshalTOML(data interface{}) error {
	switch v := data.(type) {
	case string:
		*p = StringOrArray{v}
	case []interface{}:
		for _, vv := range v {
			switch item := vv.(type) {
			case string:
				*p = append(*p, item)
			default:
				return fmt.Errorf(
					"cannot load value of type %s into a StringOrArray",
					reflect.TypeOf(item).String(),
				)
			}
		}
	default:
		return fmt.Errorf("cannot load value of type %s into a StringOrArray", reflect.TypeOf(v).String())
	}

	return nil
}

//nolint:lll
type DockerConfig struct {
	docker.Credentials
	Hostname                   string             `toml:"hostname,omitempty" json:"hostname" long:"hostname" env:"DOCKER_HOSTNAME" description:"Custom container hostname"`
	Image                      string             `toml:"image" json:"image" long:"image" env:"DOCKER_IMAGE" description:"Docker image to be used"`
	Runtime                    string             `toml:"runtime,omitempty" json:"runtime" long:"runtime" env:"DOCKER_RUNTIME" description:"Docker runtime to be used"`
	Memory                     string             `toml:"memory,omitempty" json:"memory" long:"memory" env:"DOCKER_MEMORY" description:"Memory limit (format: <number>[<unit>]). Unit can be one of b, k, m, or g. Minimum is 4M."`
	MemorySwap                 string             `toml:"memory_swap,omitempty" json:"memory_swap" long:"memory-swap" env:"DOCKER_MEMORY_SWAP" description:"Total memory limit (memory + swap, format: <number>[<unit>]). Unit can be one of b, k, m, or g."`
	MemoryReservation          string             `toml:"memory_reservation,omitempty" json:"memory_reservation" long:"memory-reservation" env:"DOCKER_MEMORY_RESERVATION" description:"Memory soft limit (format: <number>[<unit>]). Unit can be one of b, k, m, or g."`
	CPUSetCPUs                 string             `toml:"cpuset_cpus,omitempty" json:"cpuset_cpus" long:"cpuset-cpus" env:"DOCKER_CPUSET_CPUS" description:"String value containing the cgroups CpusetCpus to use"`
	CPUS                       string             `toml:"cpus,omitempty" json:"cpus" long:"cpus" env:"DOCKER_CPUS" description:"Number of CPUs"`
	CPUShares                  int64              `toml:"cpu_shares,omitzero" json:"cpu_shares" long:"cpu-shares" env:"DOCKER_CPU_SHARES" description:"Number of CPU shares"`
	DNS                        []string           `toml:"dns,omitempty" json:"dns,omitempty" long:"dns" env:"DOCKER_DNS" description:"A list of DNS servers for the container to use"`
	DNSSearch                  []string           `toml:"dns_search,omitempty" json:"dns_search,omitempty" long:"dns-search" env:"DOCKER_DNS_SEARCH" description:"A list of DNS search domains"`
	Privileged                 bool               `toml:"privileged,omitzero" json:"privileged" long:"privileged" env:"DOCKER_PRIVILEGED" description:"Give extended privileges to container"`
	ServicesPrivileged         *bool              `toml:"services_privileged,omitempty" json:"services_privileged" long:"services_privileged" env:"DOCKER_SERVICES_PRIVILEGED" description:"When set this will give or remove extended privileges to container services"`
	DisableEntrypointOverwrite bool               `toml:"disable_entrypoint_overwrite,omitzero" json:"disable_entrypoint_overwrite" long:"disable-entrypoint-overwrite" env:"DOCKER_DISABLE_ENTRYPOINT_OVERWRITE" description:"Disable the possibility for a container to overwrite the default image entrypoint"`
	User                       string             `toml:"user,omitempty" json:"user" long:"user" env:"DOCKER_USER" description:"Run all commands in the container as the specified user."`
	UsernsMode                 string             `toml:"userns_mode,omitempty" json:"userns_mode" long:"userns" env:"DOCKER_USERNS_MODE" description:"User namespace to use"`
	CapAdd                     []string           `toml:"cap_add" json:"cap_add,omitempty" long:"cap-add" env:"DOCKER_CAP_ADD" description:"Add Linux capabilities"`
	CapDrop                    []string           `toml:"cap_drop" json:"cap_drop,omitempty" long:"cap-drop" env:"DOCKER_CAP_DROP" description:"Drop Linux capabilities"`
	OomKillDisable             bool               `toml:"oom_kill_disable,omitzero" json:"oom_kill_disable" long:"oom-kill-disable" env:"DOCKER_OOM_KILL_DISABLE" description:"Do not kill processes in a container if an out-of-memory (OOM) error occurs"`
	OomScoreAdjust             int                `toml:"oom_score_adjust,omitzero" json:"oom_score_adjust" long:"oom-score-adjust" env:"DOCKER_OOM_SCORE_ADJUST" description:"Adjust OOM score"`
	SecurityOpt                []string           `toml:"security_opt" json:"security_opt,omitempty" long:"security-opt" env:"DOCKER_SECURITY_OPT" description:"Security Options"`
	ServicesSecurityOpt        []string           `toml:"services_security_opt" json:"services_security_opt,omitempty" long:"services-security-opt" env:"DOCKER_SERVICES_SECURITY_OPT" description:"Security Options for container services"`
	Devices                    []string           `toml:"devices" json:"devices" long:"devices,omitempty" env:"DOCKER_DEVICES" description:"Add a host device to the container"`
	DeviceCgroupRules          []string           `toml:"device_cgroup_rules,omitempty" json:"device_cgroup_rules" long:"device-cgroup-rules" env:"DOCKER_DEVICE_CGROUP_RULES" description:"Add a device cgroup rule to the container"`
	Gpus                       string             `toml:"gpus,omitempty" json:"gpus" long:"gpus" env:"DOCKER_GPUS" description:"Request GPUs to be used by Docker"`
	DisableCache               bool               `toml:"disable_cache,omitzero" json:"disable_cache" long:"disable-cache" env:"DOCKER_DISABLE_CACHE" description:"Disable all container caching"`
	Volumes                    []string           `toml:"volumes,omitempty" json:"volumes,omitempty" long:"volumes" env:"DOCKER_VOLUMES" description:"Bind-mount a volume and create it if it doesn't exist prior to mounting. Can be specified multiple times once per mountpoint, e.g. --docker-volumes 'test0:/test0' --docker-volumes 'test1:/test1'"`
	VolumeDriver               string             `toml:"volume_driver,omitempty" json:"volume_driver" long:"volume-driver" env:"DOCKER_VOLUME_DRIVER" description:"Volume driver to be used"`
	VolumeDriverOps            map[string]string  `toml:"volume_driver_ops,omitempty" json:"volume_driver_ops,omitempty" long:"volume-driver-ops" env:"DOCKER_VOLUME_DRIVER_OPS" description:"A toml table/json object with the format key=values. Volume driver ops to be specified"`
	CacheDir                   string             `toml:"cache_dir,omitempty" json:"cache_dir" long:"cache-dir" env:"DOCKER_CACHE_DIR" description:"Directory where to store caches"`
	ExtraHosts                 []string           `toml:"extra_hosts,omitempty" json:"extra_hosts,omitempty" long:"extra-hosts" env:"DOCKER_EXTRA_HOSTS" description:"Add a custom host-to-IP mapping"`
	VolumesFrom                []string           `toml:"volumes_from,omitempty" json:"volumes_from,omitempty" long:"volumes-from" env:"DOCKER_VOLUMES_FROM" description:"A list of volumes to inherit from another container"`
	NetworkMode                string             `toml:"network_mode,omitempty" json:"network_mode" long:"network-mode" env:"DOCKER_NETWORK_MODE" description:"Add container to a custom network"`
	IpcMode                    string             `toml:"ipcmode,omitempty" json:"ipcmode" long:"ipcmode" env:"DOCKER_IPC_MODE" description:"Select IPC mode for container"`
	MacAddress                 string             `toml:"mac_address,omitempty" json:"mac_address" long:"mac-address" env:"DOCKER_MAC_ADDRESS" description:"Container MAC address (e.g., 92:d0:c6:0a:29:33)"`
	Links                      []string           `toml:"links,omitempty" json:"links,omitempty" long:"links" env:"DOCKER_LINKS" description:"Add link to another container"`
	Services                   []Service          `toml:"services,omitempty" json:"services,omitempty" description:"Add service that is started with container"`
	WaitForServicesTimeout     int                `toml:"wait_for_services_timeout,omitzero" json:"wait_for_services_timeout" long:"wait-for-services-timeout" env:"DOCKER_WAIT_FOR_SERVICES_TIMEOUT" description:"How long to wait for service startup"`
	AllowedImages              []string           `toml:"allowed_images,omitempty" json:"allowed_images,omitempty" long:"allowed-images" env:"DOCKER_ALLOWED_IMAGES" description:"Image allowlist"`
	AllowedPullPolicies        []DockerPullPolicy `toml:"allowed_pull_policies,omitempty" json:"allowed_pull_policies,omitempty" long:"allowed-pull-policies" env:"DOCKER_ALLOWED_PULL_POLICIES" description:"Pull policy allowlist"`
	AllowedServices            []string           `toml:"allowed_services,omitempty" json:"allowed_services,omitempty" long:"allowed-services" env:"DOCKER_ALLOWED_SERVICES" description:"Service allowlist"`
	PullPolicy                 StringOrArray      `toml:"pull_policy,omitempty" json:"pull_policy" long:"pull-policy" env:"DOCKER_PULL_POLICY" description:"Image pull policy: never, if-not-present, always"`
	Isolation                  string             `toml:"isolation,omitempty" json:"isolation" long:"isolation" env:"DOCKER_ISOLATION" description:"Container isolation technology. Windows only"`
	ShmSize                    int64              `toml:"shm_size,omitempty" json:"shm_size" long:"shm-size" env:"DOCKER_SHM_SIZE" description:"Shared memory size for docker images (in bytes)"`
	Tmpfs                      map[string]string  `toml:"tmpfs,omitempty" json:"tmpfs" long:"tmpfs" env:"DOCKER_TMPFS" description:"A toml table/json object with the format key=values. When set this will mount the specified path in the key as a tmpfs volume in the main container, using the options specified as key. For the supported options, see the documentation for the unix 'mount' command"`
	ServicesTmpfs              map[string]string  `toml:"services_tmpfs,omitempty" json:"services_tmpfs" long:"services-tmpfs" env:"DOCKER_SERVICES_TMPFS" description:"A toml table/json object with the format key=values. When set this will mount the specified path in the key as a tmpfs volume in all the service containers, using the options specified as key. For the supported options, see the documentation for the unix 'mount' command"`
	SysCtls                    DockerSysCtls      `toml:"sysctls,omitempty" json:"sysctls" long:"sysctls" env:"DOCKER_SYSCTLS" description:"Sysctl options, a toml table/json object of key=value. Value is expected to be a string."`
	HelperImage                string             `toml:"helper_image,omitempty" json:"helper_image" long:"helper-image" env:"DOCKER_HELPER_IMAGE" description:"[ADVANCED] Override the default helper image used to clone repos and upload artifacts"`
	HelperImageFlavor          string             `toml:"helper_image_flavor,omitempty" json:"helper_image_flavor" long:"helper-image-flavor" env:"DOCKER_HELPER_IMAGE_FLAVOR" description:"Set helper image flavor (alpine, ubuntu), defaults to alpine"`
	ContainerLabels            map[string]string  `toml:"container_labels,omitempty" json:"container_labels" long:"container-labels" description:"A toml table/json object of key-value. Value is expected to be a string. When set, this will create containers with the given container labels. Environment variables will be substituted for values here."`
	EnableIPv6                 bool               `toml:"enable_ipv6,omitempty" json:"enable_ipv6" long:"enable-ipv6" description:"Enable IPv6 for automatically created networks. This is only takes affect when the feature flag FF_NETWORK_PER_BUILD is enabled."`
}

//nolint:lll
type InstanceConfig struct {
	AllowedImages []string `toml:"allowed_images,omitempty" description:"When VM Isolation is enabled, allowed images controls which images a job is allowed to specify"`
}

type AutoscalerConfig struct {
	CapacityPerInstance int                      `toml:"capacity_per_instance,omitempty"`
	MaxUseCount         int                      `toml:"max_use_count,omitempty"`
	MaxInstances        int                      `toml:"max_instances,omitempty"`
	Plugin              string                   `toml:"plugin,omitempty"`
	PluginConfig        AutoscalerSettingsMap    `toml:"plugin_config,omitempty"`
	ConnectorConfig     ConnectorConfig          `toml:"connector_config,omitempty"`
	Policy              []AutoscalerPolicyConfig `toml:"policy,omitempty" json:",omitempty"`

	VMIsolation VMIsolation `toml:"vm_isolation,omitempty"`

	InstanceOperationTimeBuckets []float64 `toml:"instance_operation_time_buckets,omitempty" json:",omitempty"`
}

type VMIsolation struct {
	Enabled         bool                  `toml:"enabled,omitempty"`
	NestingHost     string                `toml:"nesting_host,omitempty"`
	NestingConfig   AutoscalerSettingsMap `toml:"nesting_config,omitempty" json:",omitempty"`
	Image           string                `toml:"image,omitempty"`
	ConnectorConfig ConnectorConfig       `toml:"connector_config,omitempty"`
}

type ConnectorConfig struct {
	OS                   string        `toml:"os,omitempty"`
	Arch                 string        `toml:"arch,omitempty"`
	Protocol             string        `toml:"protocol,omitempty"`
	Username             string        `toml:"username"`
	Password             string        `toml:"password"`
	KeyPathname          string        `toml:"key_path"`
	UseStaticCredentials bool          `toml:"use_static_credentials"`
	Keepalive            time.Duration `toml:"keepalive"`
	Timeout              time.Duration `toml:"timeout"`
	UseExternalAddr      bool          `toml:"use_external_addr"`
}

type AutoscalerSettingsMap map[string]interface{}

func (settings AutoscalerSettingsMap) JSON() ([]byte, error) {
	return json.Marshal(settings)
}

type AutoscalerPolicyConfig struct {
	Periods          []string      `toml:"periods,omitempty" json:",omitempty"`
	Timezone         string        `toml:"timezone,omitempty"`
	IdleCount        int           `toml:"idle_count,omitempty"`
	IdleTime         time.Duration `toml:"idle_time,omitempty" json:",omitempty" jsonschema:"minimum=1000000000"`
	ScaleFactor      float64       `toml:"scale_factor,omitempty"`
	ScaleFactorLimit int           `toml:"scale_factor_limit,omitempty"`
}

//nolint:lll
type DockerMachine struct {
	MaxGrowthRate int `toml:"MaxGrowthRate,omitzero" long:"max-growth-rate" env:"MACHINE_MAX_GROWTH_RATE" description:"Maximum machines being provisioned concurrently, set to 0 for unlimited"`

	IdleCount       int      `long:"idle-nodes" env:"MACHINE_IDLE_COUNT" description:"Maximum idle machines"`
	IdleScaleFactor float64  `long:"idle-scale-factor" env:"MACHINE_IDLE_SCALE_FACTOR" description:"(Experimental) Defines what factor of in-use machines should be used as current idle value, but never more then defined IdleCount. 0.0 means use IdleCount as a static number (defaults to 0.0). Must be defined as float number."`
	IdleCountMin    int      `long:"idle-count-min" env:"MACHINE_IDLE_COUNT_MIN" description:"Minimal number of idle machines when IdleScaleFactor is in use. Defaults to 1."`
	IdleTime        int      `toml:"IdleTime,omitzero" long:"idle-time" env:"MACHINE_IDLE_TIME" description:"Minimum time after node can be destroyed"`
	MaxBuilds       int      `toml:"MaxBuilds,omitzero" long:"max-builds" env:"MACHINE_MAX_BUILDS" description:"Maximum number of builds processed by machine"`
	MachineDriver   string   `long:"machine-driver" env:"MACHINE_DRIVER" description:"The driver to use when creating machine"`
	MachineName     string   `long:"machine-name" env:"MACHINE_NAME" description:"The template for machine name (needs to include %s)"`
	MachineOptions  []string `long:"machine-options" json:",omitempty" env:"MACHINE_OPTIONS" description:"Additional machine creation options"`

	OffPeakPeriods   []string `toml:"OffPeakPeriods,omitempty" json:",omitempty" description:"Time periods when the scheduler is in the OffPeak mode. DEPRECATED"`              // DEPRECATED
	OffPeakTimezone  string   `toml:"OffPeakTimezone,omitempty" description:"Timezone for the OffPeak periods (defaults to Local). DEPRECATED"`                                 // DEPRECATED
	OffPeakIdleCount int      `toml:"OffPeakIdleCount,omitzero" description:"Maximum idle machines when the scheduler is in the OffPeak mode. DEPRECATED"`                      // DEPRECATED
	OffPeakIdleTime  int      `toml:"OffPeakIdleTime,omitzero" description:"Minimum time after machine can be destroyed when the scheduler is in the OffPeak mode. DEPRECATED"` // DEPRECATED

	AutoscalingConfigs []*DockerMachineAutoscaling `toml:"autoscaling" description:"Ordered list of configurations for autoscaling periods (last match wins)"`
}

//nolint:lll
type DockerMachineAutoscaling struct {
	Periods         []string `long:"periods" json:",omitempty" description:"List of crontab expressions for this autoscaling configuration"`
	Timezone        string   `long:"timezone" description:"Timezone for the periods (defaults to Local)"`
	IdleCount       int      `long:"idle-count" description:"Maximum idle machines when this configuration is active"`
	IdleScaleFactor float64  `long:"idle-scale-factor" description:"(Experimental) Defines what factor of in-use machines should be used as current idle value, but never more then defined IdleCount. 0.0 means use IdleCount as a static number (defaults to 0.0). Must be defined as float number."`
	IdleCountMin    int      `long:"idle-count-min" description:"Minimal number of idle machines when IdleScaleFactor is in use. Defaults to 1."`
	IdleTime        int      `long:"idle-time" description:"Minimum time after which and idle machine can be destroyed when this configuration is active"`
	compiledPeriods *timeperiod.TimePeriod
}

//nolint:lll
type ParallelsConfig struct {
	BaseName         string   `toml:"base_name" json:"base_name" long:"base-name" env:"PARALLELS_BASE_NAME" description:"VM name to be used"`
	TemplateName     string   `toml:"template_name,omitempty" json:"template_name" long:"template-name" env:"PARALLELS_TEMPLATE_NAME" description:"VM template to be created"`
	DisableSnapshots bool     `toml:"disable_snapshots,omitzero" json:"disable_snapshots" long:"disable-snapshots" env:"PARALLELS_DISABLE_SNAPSHOTS" description:"Disable snapshoting to speedup VM creation"`
	TimeServer       string   `toml:"time_server,omitempty" json:"time_server" long:"time-server" env:"PARALLELS_TIME_SERVER" description:"Timeserver to sync the guests time from. Defaults to time.apple.com"`
	AllowedImages    []string `toml:"allowed_images,omitempty" json:"allowed_images,omitempty" long:"allowed-images" env:"PARALLELS_ALLOWED_IMAGES" description:"Image (base_name) allowlist"`
}

//nolint:lll
type VirtualBoxConfig struct {
	BaseName         string   `toml:"base_name" json:"base_name" long:"base-name" env:"VIRTUALBOX_BASE_NAME" description:"VM name to be used"`
	BaseSnapshot     string   `toml:"base_snapshot,omitempty" json:"base_snapshot" long:"base-snapshot" env:"VIRTUALBOX_BASE_SNAPSHOT" description:"Name or UUID of a specific VM snapshot to clone"`
	BaseFolder       string   `toml:"base_folder" json:"base_folder" long:"base-folder" env:"VIRTUALBOX_BASE_FOLDER" description:"Folder in which to save the new VM. If empty, uses VirtualBox default"`
	DisableSnapshots bool     `toml:"disable_snapshots,omitzero" json:"disable_snapshots" long:"disable-snapshots" env:"VIRTUALBOX_DISABLE_SNAPSHOTS" description:"Disable snapshoting to speedup VM creation"`
	AllowedImages    []string `toml:"allowed_images,omitempty" json:"allowed_images,omitempty" long:"allowed-images" env:"VIRTUALBOX_ALLOWED_IMAGES" description:"Image allowlist"`
	StartType        string   `toml:"start_type" json:"start_type" long:"start-type" env:"VIRTUALBOX_START_TYPE" description:"Graphical front-end type"`
}

//nolint:lll
type CustomConfig struct {
	ConfigExec        string   `toml:"config_exec,omitempty" json:"config_exec" long:"config-exec" env:"CUSTOM_CONFIG_EXEC" description:"Executable that allows to inject configuration values to the executor"`
	ConfigArgs        []string `toml:"config_args,omitempty" json:"config_args,omitempty" long:"config-args" description:"Arguments for the config executable"`
	ConfigExecTimeout *int     `toml:"config_exec_timeout,omitempty" json:"config_exec_timeout" long:"config-exec-timeout" env:"CUSTOM_CONFIG_EXEC_TIMEOUT" description:"Timeout for the config executable (in seconds)"`

	PrepareExec        string   `toml:"prepare_exec,omitempty" json:"prepare_exec" long:"prepare-exec" env:"CUSTOM_PREPARE_EXEC" description:"Executable that prepares executor"`
	PrepareArgs        []string `toml:"prepare_args,omitempty" json:"prepare_args,omitempty" long:"prepare-args" description:"Arguments for the prepare executable"`
	PrepareExecTimeout *int     `toml:"prepare_exec_timeout,omitempty" json:"prepare_exec_timeout,omitempty" long:"prepare-exec-timeout" env:"CUSTOM_PREPARE_EXEC_TIMEOUT" description:"Timeout for the prepare executable (in seconds)"`

	RunExec string   `toml:"run_exec" json:"run_exec" long:"run-exec" env:"CUSTOM_RUN_EXEC" description:"Executable that runs the job script in executor"`
	RunArgs []string `toml:"run_args,omitempty" json:"run_args" long:"run-args" description:"Arguments for the run executable"`

	CleanupExec        string   `toml:"cleanup_exec,omitempty" json:"cleanup_exec" long:"cleanup-exec" env:"CUSTOM_CLEANUP_EXEC" description:"Executable that cleanups after executor run"`
	CleanupArgs        []string `toml:"cleanup_args,omitempty" json:"cleanup_args,omitempty" long:"cleanup-args" description:"Arguments for the cleanup executable"`
	CleanupExecTimeout *int     `toml:"cleanup_exec_timeout,omitempty" json:"cleanup_exec_timeout,omitempty" long:"cleanup-exec-timeout" env:"CUSTOM_CLEANUP_EXEC_TIMEOUT" description:"Timeout for the cleanup executable (in seconds)"`

	GracefulKillTimeout *int `toml:"graceful_kill_timeout,omitempty" json:"graceful_kill_timeout,omitempty" long:"graceful-kill-timeout" env:"CUSTOM_GRACEFUL_KILL_TIMEOUT" description:"Graceful timeout for scripts execution after SIGTERM is sent to the process (in seconds). This limits the time given for scripts to perform the cleanup before exiting"`
	ForceKillTimeout    *int `toml:"force_kill_timeout,omitempty" json:"force_kill_timeout,omitempty" long:"force-kill-timeout" env:"CUSTOM_FORCE_KILL_TIMEOUT" description:"Force timeout for scripts execution (in seconds). Counted from the force kill call; if process will be not terminated, Runner will abandon process termination and log an error"`
}

// GetPullPolicies returns a validated list of pull policies, falling back to a predefined value if empty,
// or returns an error if the list is not valid
func (c KubernetesConfig) GetPullPolicies() ([]api.PullPolicy, error) {
	// Default to cluster pull policy
	if len(c.PullPolicy) == 0 {
		return []api.PullPolicy{""}, nil
	}

	// Verify pull policies
	policies := make([]DockerPullPolicy, len(c.PullPolicy))
	for idx, policy := range c.PullPolicy {
		policies[idx] = DockerPullPolicy(policy)
	}

	pullPolicies, err := c.ConvertFromDockerPullPolicy(policies)
	if err != nil {
		return nil, fmt.Errorf("pull_policy config: %w", err)
	}

	return pullPolicies, nil
}

// ConvertFromDockerPullPolicy converts an array of DockerPullPolicy to an api.PullPolicy array
// or returns an error if the list contains invalid pull policies.
func (c KubernetesConfig) ConvertFromDockerPullPolicy(dockerPullPolicies []DockerPullPolicy) ([]api.PullPolicy, error) {
	policies := make([]api.PullPolicy, len(dockerPullPolicies))

	for idx, policy := range dockerPullPolicies {
		switch policy {
		case "":
			policies[idx] = ""
		case PullPolicyAlways:
			policies[idx] = api.PullAlways
		case PullPolicyNever:
			policies[idx] = api.PullNever
		case PullPolicyIfNotPresent:
			policies[idx] = api.PullIfNotPresent
		default:
			return []api.PullPolicy{""}, fmt.Errorf("unsupported pull policy: %q", policy)
		}
	}

	return policies, nil
}

type KubernetesDNSPolicy string

// Get returns one of the predefined values in kubernetes notation or an error if the value is not matched.
// If the DNSPolicy is a blank string, returns the k8s default ("ClusterFirst")
func (p KubernetesDNSPolicy) Get() (api.DNSPolicy, error) {
	const defaultPolicy = api.DNSClusterFirst

	switch p {
	case "":
		logrus.Debugf("DNSPolicy string is blank, using %q as default", defaultPolicy)
		return defaultPolicy, nil
	case DNSPolicyNone:
		return api.DNSNone, nil
	case DNSPolicyDefault:
		return api.DNSDefault, nil
	case DNSPolicyClusterFirst:
		return api.DNSClusterFirst, nil
	case DNSPolicyClusterFirstWithHostNet:
		return api.DNSClusterFirstWithHostNet, nil
	}

	return "", fmt.Errorf("unsupported kubernetes-dns-policy: %q", p)
}

//nolint:lll
type KubernetesConfig struct {
	Host                                              string                             `toml:"host" json:"host" long:"host" env:"KUBERNETES_HOST" description:"Optional Kubernetes master host URL (auto-discovery attempted if not specified)"`
	CertFile                                          string                             `toml:"cert_file,omitempty" json:"cert_file" long:"cert-file" env:"KUBERNETES_CERT_FILE" description:"Optional Kubernetes master auth certificate"`
	KeyFile                                           string                             `toml:"key_file,omitempty" json:"key_file" long:"key-file" env:"KUBERNETES_KEY_FILE" description:"Optional Kubernetes master auth private key"`
	CAFile                                            string                             `toml:"ca_file,omitempty" json:"ca_file" long:"ca-file" env:"KUBERNETES_CA_FILE" description:"Optional Kubernetes master auth ca certificate"`
	BearerTokenOverwriteAllowed                       bool                               `toml:"bearer_token_overwrite_allowed" json:"bearer_token_overwrite_allowed" long:"bearer_token_overwrite_allowed" env:"KUBERNETES_BEARER_TOKEN_OVERWRITE_ALLOWED" description:"Bool to authorize builds to specify their own bearer token for creation."`
	BearerToken                                       string                             `toml:"bearer_token,omitempty" json:"bearer_token" long:"bearer_token" env:"KUBERNETES_BEARER_TOKEN" description:"Optional Kubernetes service account token used to start build pods."`
	Image                                             string                             `toml:"image" json:"image" long:"image" env:"KUBERNETES_IMAGE" description:"Default docker image to use for builds when none is specified"`
	Namespace                                         string                             `toml:"namespace" json:"namespace" long:"namespace" env:"KUBERNETES_NAMESPACE" description:"Namespace to run Kubernetes jobs in"`
	NamespaceOverwriteAllowed                         string                             `toml:"namespace_overwrite_allowed" json:"namespace_overwrite_allowed" long:"namespace_overwrite_allowed" env:"KUBERNETES_NAMESPACE_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_NAMESPACE_OVERWRITE' value"`
	Privileged                                        *bool                              `toml:"privileged,omitzero" json:"privileged,omitempty" long:"privileged" env:"KUBERNETES_PRIVILEGED" description:"Run all containers with the privileged flag enabled"`
	RuntimeClassName                                  *string                            `toml:"runtime_class_name,omitempty" json:"runtime_class_name,omitempty" long:"runtime-class-name" env:"KUBERNETES_RUNTIME_CLASS_NAME" description:"A Runtime Class to use for all created pods, errors if the feature is unsupported by the cluster"`
	AllowPrivilegeEscalation                          *bool                              `toml:"allow_privilege_escalation,omitzero" json:"allow_privilege_escalation,omitempty" long:"allow-privilege-escalation" env:"KUBERNETES_ALLOW_PRIVILEGE_ESCALATION" description:"Run all containers with the security context allowPrivilegeEscalation flag enabled. When empty, it does not define the allowPrivilegeEscalation flag in the container SecurityContext and allows Kubernetes to use the default privilege escalation behavior."`
	CPULimit                                          string                             `toml:"cpu_limit,omitempty" json:"cpu_limit" long:"cpu-limit" env:"KUBERNETES_CPU_LIMIT" description:"The CPU allocation given to build containers"`
	CPULimitOverwriteMaxAllowed                       string                             `toml:"cpu_limit_overwrite_max_allowed,omitempty" json:"cpu_limit_overwrite_max_allowed" long:"cpu-limit-overwrite-max-allowed" env:"KUBERNETES_CPU_LIMIT_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the cpu limit can be set to. Used with the KUBERNETES_CPU_LIMIT variable in the build."`
	CPURequest                                        string                             `toml:"cpu_request,omitempty" json:"cpu_request" long:"cpu-request" env:"KUBERNETES_CPU_REQUEST" description:"The CPU allocation requested for build containers"`
	CPURequestOverwriteMaxAllowed                     string                             `toml:"cpu_request_overwrite_max_allowed,omitempty" json:"cpu_request_overwrite_max_allowed" long:"cpu-request-overwrite-max-allowed" env:"KUBERNETES_CPU_REQUEST_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the cpu request can be set to. Used with the KUBERNETES_CPU_REQUEST variable in the build."`
	MemoryLimit                                       string                             `toml:"memory_limit,omitempty" json:"memory_limit" long:"memory-limit" env:"KUBERNETES_MEMORY_LIMIT" description:"The amount of memory allocated to build containers"`
	MemoryLimitOverwriteMaxAllowed                    string                             `toml:"memory_limit_overwrite_max_allowed,omitempty" json:"memory_limit_overwrite_max_allowed" long:"memory-limit-overwrite-max-allowed" env:"KUBERNETES_MEMORY_LIMIT_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the memory limit can be set to. Used with the KUBERNETES_MEMORY_LIMIT variable in the build."`
	MemoryRequest                                     string                             `toml:"memory_request,omitempty" json:"memory_request" long:"memory-request" env:"KUBERNETES_MEMORY_REQUEST" description:"The amount of memory requested from build containers"`
	MemoryRequestOverwriteMaxAllowed                  string                             `toml:"memory_request_overwrite_max_allowed,omitempty" json:"memory_request_overwrite_max_allowed" long:"memory-request-overwrite-max-allowed" env:"KUBERNETES_MEMORY_REQUEST_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the memory request can be set to. Used with the KUBERNETES_MEMORY_REQUEST variable in the build."`
	EphemeralStorageLimit                             string                             `toml:"ephemeral_storage_limit,omitempty" json:"ephemeral_storage_limit" long:"ephemeral-storage-limit" env:"KUBERNETES_EPHEMERAL_STORAGE_LIMIT" description:"The amount of ephemeral storage allocated to build containers"`
	EphemeralStorageLimitOverwriteMaxAllowed          string                             `toml:"ephemeral_storage_limit_overwrite_max_allowed,omitempty" json:"ephemeral_storage_limit_overwrite_max_allowed" long:"ephemeral-storage-limit-overwrite-max-allowed" env:"KUBERNETES_EPHEMERAL_STORAGE_LIMIT_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the ephemeral limit can be set to. Used with the KUBERNETES_EPHEMERAL_STORAGE_LIMIT variable in the build."`
	EphemeralStorageRequest                           string                             `toml:"ephemeral_storage_request,omitempty" json:"ephemeral_storage_request" long:"ephemeral-storage-request" env:"KUBERNETES_EPHEMERAL_STORAGE_REQUEST" description:"The amount of ephemeral storage requested from build containers"`
	EphemeralStorageRequestOverwriteMaxAllowed        string                             `toml:"ephemeral_storage_request_overwrite_max_allowed,omitempty" json:"ephemeral_storage_request_overwrite_max_allowed" long:"ephemeral-storage-request-overwrite-max-allowed" env:"KUBERNETES_EPHEMERAL_STORAGE_REQUEST_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the ephemeral storage request can be set to. Used with the KUBERNETES_EPHEMERAL_STORAGE_REQUEST variable in the build."`
	ServiceCPULimit                                   string                             `toml:"service_cpu_limit,omitempty" json:"service_cpu_limit" long:"service-cpu-limit" env:"KUBERNETES_SERVICE_CPU_LIMIT" description:"The CPU allocation given to build service containers"`
	ServiceCPULimitOverwriteMaxAllowed                string                             `toml:"service_cpu_limit_overwrite_max_allowed,omitempty" json:"service_cpu_limit_overwrite_max_allowed" long:"service-cpu-limit-overwrite-max-allowed" env:"KUBERNETES_SERVICE_CPU_LIMIT_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the service cpu limit can be set to. Used with the KUBERNETES_SERVICE_CPU_LIMIT variable in the build."`
	ServiceCPURequest                                 string                             `toml:"service_cpu_request,omitempty" json:"service_cpu_request" long:"service-cpu-request" env:"KUBERNETES_SERVICE_CPU_REQUEST" description:"The CPU allocation requested for build service containers"`
	ServiceCPURequestOverwriteMaxAllowed              string                             `toml:"service_cpu_request_overwrite_max_allowed,omitempty" json:"service_cpu_request_overwrite_max_allowed" long:"service-cpu-request-overwrite-max-allowed" env:"KUBERNETES_SERVICE_CPU_REQUEST_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the service cpu request can be set to. Used with the KUBERNETES_SERVICE_CPU_REQUEST variable in the build."`
	ServiceMemoryLimit                                string                             `toml:"service_memory_limit,omitempty" json:"service_memory_limit" long:"service-memory-limit" env:"KUBERNETES_SERVICE_MEMORY_LIMIT" description:"The amount of memory allocated to build service containers"`
	ServiceMemoryLimitOverwriteMaxAllowed             string                             `toml:"service_memory_limit_overwrite_max_allowed,omitempty" json:"service_memory_limit_overwrite_max_allowed" long:"service-memory-limit-overwrite-max-allowed" env:"KUBERNETES_SERVICE_MEMORY_LIMIT_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the service memory limit can be set to. Used with the KUBERNETES_SERVICE_MEMORY_LIMIT variable in the build."`
	ServiceMemoryRequest                              string                             `toml:"service_memory_request,omitempty" json:"service_memory_request" long:"service-memory-request" env:"KUBERNETES_SERVICE_MEMORY_REQUEST" description:"The amount of memory requested for build service containers"`
	ServiceMemoryRequestOverwriteMaxAllowed           string                             `toml:"service_memory_request_overwrite_max_allowed,omitempty" json:"service_memory_request_overwrite_max_allowed" long:"service-memory-request-overwrite-max-allowed" env:"KUBERNETES_SERVICE_MEMORY_REQUEST_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the service memory request can be set to. Used with the KUBERNETES_SERVICE_MEMORY_REQUEST variable in the build."`
	ServiceEphemeralStorageLimit                      string                             `toml:"service_ephemeral_storage_limit,omitempty" json:"service_ephemeral_storage_limit" long:"service-ephemeral_storage-limit" env:"KUBERNETES_SERVICE_EPHEMERAL_STORAGE_LIMIT" description:"The amount of ephemeral storage allocated to build service containers"`
	ServiceEphemeralStorageLimitOverwriteMaxAllowed   string                             `toml:"service_ephemeral_storage_limit_overwrite_max_allowed,omitempty" json:"service_ephemeral_storage_limit_overwrite_max_allowed" long:"service-ephemeral_storage-limit-overwrite-max-allowed" env:"KUBERNETES_SERVICE_EPHEMERAL_STORAGE_LIMIT_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the service ephemeral storage limit can be set to. Used with the KUBERNETES_SERVICE_EPHEMERAL_STORAGE_LIMIT variable in the build."`
	ServiceEphemeralStorageRequest                    string                             `toml:"service_ephemeral_storage_request,omitempty" json:"service_ephemeral_storage_request" long:"service-ephemeral_storage-request" env:"KUBERNETES_SERVICE_EPHEMERAL_STORAGE_REQUEST" description:"The amount of ephemeral storage requested for build service containers"`
	ServiceEphemeralStorageRequestOverwriteMaxAllowed string                             `toml:"service_ephemeral_storage_request_overwrite_max_allowed,omitempty" json:"service_ephemeral_storage_request_overwrite_max_allowed" long:"service-ephemeral_storage-request-overwrite-max-allowed" env:"KUBERNETES_SERVICE_EPHEMERAL_STORAGE_REQUEST_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the service ephemeral storage request can be set to. Used with the KUBERNETES_SERVICE_EPHEMERAL_STORAGE_REQUEST variable in the build."`
	HelperCPULimit                                    string                             `toml:"helper_cpu_limit,omitempty" json:"helper_cpu_limit" long:"helper-cpu-limit" env:"KUBERNETES_HELPER_CPU_LIMIT" description:"The CPU allocation given to build helper containers"`
	HelperCPULimitOverwriteMaxAllowed                 string                             `toml:"helper_cpu_limit_overwrite_max_allowed,omitempty" json:"helper_cpu_limit_overwrite_max_allowed" long:"helper-cpu-limit-overwrite-max-allowed" env:"KUBERNETES_HELPER_CPU_LIMIT_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the helper cpu limit can be set to. Used with the KUBERNETES_HELPER_CPU_LIMIT variable in the build."`
	HelperCPURequest                                  string                             `toml:"helper_cpu_request,omitempty" json:"helper_cpu_request" long:"helper-cpu-request" env:"KUBERNETES_HELPER_CPU_REQUEST" description:"The CPU allocation requested for build helper containers"`
	HelperCPURequestOverwriteMaxAllowed               string                             `toml:"helper_cpu_request_overwrite_max_allowed,omitempty" json:"helper_cpu_request_overwrite_max_allowed" long:"helper-cpu-request-overwrite-max-allowed" env:"KUBERNETES_HELPER_CPU_REQUEST_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the helper cpu request can be set to. Used with the KUBERNETES_HELPER_CPU_REQUEST variable in the build."`
	HelperMemoryLimit                                 string                             `toml:"helper_memory_limit,omitempty" json:"helper_memory_limit" long:"helper-memory-limit" env:"KUBERNETES_HELPER_MEMORY_LIMIT" description:"The amount of memory allocated to build helper containers"`
	HelperMemoryLimitOverwriteMaxAllowed              string                             `toml:"helper_memory_limit_overwrite_max_allowed,omitempty" json:"helper_memory_limit_overwrite_max_allowed" long:"helper-memory-limit-overwrite-max-allowed" env:"KUBERNETES_HELPER_MEMORY_LIMIT_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the helper memory limit can be set to. Used with the KUBERNETES_HELPER_MEMORY_LIMIT variable in the build."`
	HelperMemoryRequest                               string                             `toml:"helper_memory_request,omitempty" json:"helper_memory_request" long:"helper-memory-request" env:"KUBERNETES_HELPER_MEMORY_REQUEST" description:"The amount of memory requested for build helper containers"`
	HelperMemoryRequestOverwriteMaxAllowed            string                             `toml:"helper_memory_request_overwrite_max_allowed,omitempty" json:"helper_memory_request_overwrite_max_allowed" long:"helper-memory-request-overwrite-max-allowed" env:"KUBERNETES_HELPER_MEMORY_REQUEST_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the helper memory request can be set to. Used with the KUBERNETES_HELPER_MEMORY_REQUEST variable in the build."`
	HelperEphemeralStorageLimit                       string                             `toml:"helper_ephemeral_storage_limit,omitempty" json:"helper_ephemeral_storage_limit" long:"helper-ephemeral_storage-limit" env:"KUBERNETES_HELPER_EPHEMERAL_STORAGE_LIMIT" description:"The amount of ephemeral storage allocated to build helper containers"`
	HelperEphemeralStorageLimitOverwriteMaxAllowed    string                             `toml:"helper_ephemeral_storage_limit_overwrite_max_allowed,omitempty" json:"helper_ephemeral_storage_limit_overwrite_max_allowed" long:"helper-ephemeral_storage-limit-overwrite-max-allowed" env:"KUBERNETES_HELPER_EPHEMERAL_STORAGE_LIMIT_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the helper ephemeral storage limit can be set to. Used with the KUBERNETES_HELPER_EPHEMERAL_STORAGE_LIMIT variable in the build."`
	HelperEphemeralStorageRequest                     string                             `toml:"helper_ephemeral_storage_request,omitempty" json:"helper_ephemeral_storage_request" long:"helper-ephemeral_storage-request" env:"KUBERNETES_HELPER_EPHEMERAL_STORAGE_REQUEST" description:"The amount of ephemeral storage requested for build helper containers"`
	HelperEphemeralStorageRequestOverwriteMaxAllowed  string                             `toml:"helper_ephemeral_storage_request_overwrite_max_allowed,omitempty" json:"helper_ephemeral_storage_request_overwrite_max_allowed" long:"helper-ephemeral_storage-request-overwrite-max-allowed" env:"KUBERNETES_HELPER_EPHEMERAL_STORAGE_REQUEST_OVERWRITE_MAX_ALLOWED" description:"If set, the max amount the helper ephemeral storage request can be set to. Used with the KUBERNETES_HELPER_EPHEMERAL_STORAGE_REQUEST variable in the build."`
	AllowedImages                                     []string                           `toml:"allowed_images,omitempty" json:"allowed_images,omitempty" long:"allowed-images" env:"KUBERNETES_ALLOWED_IMAGES" description:"Image allowlist"`
	AllowedPullPolicies                               []DockerPullPolicy                 `toml:"allowed_pull_policies,omitempty" json:"allowed_pull_policies,omitempty" long:"allowed-pull-policies" env:"KUBERNETES_ALLOWED_PULL_POLICIES" description:"Pull policy allowlist"`
	AllowedServices                                   []string                           `toml:"allowed_services,omitempty" json:"allowed_services,omitempty" long:"allowed-services" env:"KUBERNETES_ALLOWED_SERVICES" description:"Service allowlist"`
	PullPolicy                                        StringOrArray                      `toml:"pull_policy,omitempty" json:"pull_policy" long:"pull-policy" env:"KUBERNETES_PULL_POLICY" description:"Policy for if/when to pull a container image (never, if-not-present, always). The cluster default will be used if not set"`
	NodeSelector                                      map[string]string                  `toml:"node_selector,omitempty" json:"node_selector,omitempty" long:"node-selector" env:"KUBERNETES_NODE_SELECTOR" description:"A toml table/json object of key:value. Value is expected to be a string. When set this will create pods on k8s nodes that match all the key:value pairs. Only one selector is supported through environment variable configuration."`
	NodeSelectorOverwriteAllowed                      string                             `toml:"node_selector_overwrite_allowed" json:"node_selector_overwrite_allowed" long:"node_selector_overwrite_allowed" env:"KUBERNETES_NODE_SELECTOR_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_NODE_SELECTOR_*' values"`
	NodeTolerations                                   map[string]string                  `toml:"node_tolerations,omitempty" json:"node_tolerations" long:"node-tolerations" env:"KUBERNETES_NODE_TOLERATIONS" description:"A toml table/json object of key=value:effect. Value and effect are expected to be strings. When set, pods will tolerate the given taints. Only one toleration is supported through environment variable configuration."`
	Affinity                                          KubernetesAffinity                 `toml:"affinity,omitempty" json:"affinity" long:"affinity" description:"Kubernetes Affinity setting that is used to select the node that spawns a pod"`
	ImagePullSecrets                                  []string                           `toml:"image_pull_secrets,omitempty" json:"image_pull_secrets,omitempty" long:"image-pull-secrets" env:"KUBERNETES_IMAGE_PULL_SECRETS" description:"A list of image pull secrets that are used for pulling docker image"`
	HelperImage                                       string                             `toml:"helper_image,omitempty" json:"helper_image" long:"helper-image" env:"KUBERNETES_HELPER_IMAGE" description:"[ADVANCED] Override the default helper image used to clone repos and upload artifacts"`
	HelperImageFlavor                                 string                             `toml:"helper_image_flavor,omitempty" json:"helper_image_flavor" long:"helper-image-flavor" env:"KUBERNETES_HELPER_IMAGE_FLAVOR" description:"Set helper image flavor (alpine, ubuntu), defaults to alpine"`
	TerminationGracePeriodSeconds                     *int64                             `toml:"terminationGracePeriodSeconds,omitzero" json:"terminationGracePeriodSeconds,omitempty" long:"terminationGracePeriodSeconds" env:"KUBERNETES_TERMINATIONGRACEPERIODSECONDS" description:"Duration after the processes running in the pod are sent a termination signal and the time when the processes are forcibly halted with a kill signal.DEPRECATED: use KUBERNETES_POD_TERMINATION_GRACE_PERIOD_SECONDS and KUBERNETES_CLEANUP_GRACE_PERIOD_SECONDS instead."`
	PodTerminationGracePeriodSeconds                  *int64                             `toml:"pod_termination_grace_period_seconds,omitzero" json:"pod_termination_grace_period_seconds,omitempty" long:"pod_termination_grace_period_seconds" env:"KUBERNETES_POD_TERMINATION_GRACE_PERIOD_SECONDS" description:"Pod-level setting which determines the duration in seconds which the pod has to terminate gracefully. After this, the processes are forcibly halted with a kill signal. Ignored if KUBERNETES_TERMINATIONGRACEPERIODSECONDS is specified."`
	CleanupGracePeriodSeconds                         *int64                             `toml:"cleanup_grace_period_seconds,omitzero" json:"cleanup_grace_period_seconds,omitempty" long:"cleanup_grace_period_seconds" env:"KUBERNETES_CLEANUP_GRACE_PERIOD_SECONDS" description:"When cleaning up a pod on completion of a job, the duration in seconds which the pod has to terminate gracefully. After this, the processes are forcibly halted with a kill signal. Ignored if KUBERNETES_TERMINATIONGRACEPERIODSECONDS is specified."`
	PollInterval                                      int                                `toml:"poll_interval,omitzero" json:"poll_interval" long:"poll-interval" env:"KUBERNETES_POLL_INTERVAL" description:"How frequently, in seconds, the runner will poll the Kubernetes pod it has just created to check its status"`
	PollTimeout                                       int                                `toml:"poll_timeout,omitzero" json:"poll_timeout" long:"poll-timeout" env:"KUBERNETES_POLL_TIMEOUT" description:"The total amount of time, in seconds, that needs to pass before the runner will timeout attempting to connect to the pod it has just created (useful for queueing more builds that the cluster can handle at a time)"`
	ResourceAvailabilityCheckMaxAttempts              int                                `toml:"resource_availability_check_max_attempts,omitzero" json:"resource_availability_check_max_attempts" long:"resource-availability-check-max-attempts" env:"KUBERNETES_RESOURCE_AVAILABILITY_CHECK_MAX_ATTEMPTS" default:"5" description:"The maximum number of attempts to check if a resource (service account and/or pull secret) set is available before giving up. There is 5 seconds interval between each attempt"`
	PodLabels                                         map[string]string                  `toml:"pod_labels,omitempty" json:"pod_labels,omitempty" long:"pod-labels" description:"A toml table/json object of key-value. Value is expected to be a string. When set, this will create pods with the given pod labels. Environment variables will be substituted for values here."`
	PodLabelsOverwriteAllowed                         string                             `toml:"pod_labels_overwrite_allowed" json:"pod_labels_overwrite_allowed" long:"pod_labels_overwrite_allowed" env:"KUBERNETES_POD_LABELS_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_POD_LABELS_*' values"`
	SchedulerName                                     string                             `toml:"scheduler_name,omitempty" json:"scheduler_name" long:"scheduler-name" env:"KUBERNETES_SCHEDULER_NAME" description:"Pods will be scheduled using this scheduler, if it exists"`
	ServiceAccount                                    string                             `toml:"service_account,omitempty" json:"service_account" long:"service-account" env:"KUBERNETES_SERVICE_ACCOUNT" description:"Executor pods will use this Service Account to talk to kubernetes API"`
	ServiceAccountOverwriteAllowed                    string                             `toml:"service_account_overwrite_allowed" json:"service_account_overwrite_allowed" long:"service_account_overwrite_allowed" env:"KUBERNETES_SERVICE_ACCOUNT_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_SERVICE_ACCOUNT' value"`
	PodAnnotations                                    map[string]string                  `toml:"pod_annotations,omitempty" json:"pod_annotations,omitempty" long:"pod-annotations" description:"A toml table/json object of key-value. Value is expected to be a string. When set, this will create pods with the given annotations. Can be overwritten in build with KUBERNETES_POD_ANNOTATION_* variables"`
	PodAnnotationsOverwriteAllowed                    string                             `toml:"pod_annotations_overwrite_allowed" json:"pod_annotations_overwrite_allowed" long:"pod_annotations_overwrite_allowed" env:"KUBERNETES_POD_ANNOTATIONS_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_POD_ANNOTATIONS_*' values"`
	PodSecurityContext                                KubernetesPodSecurityContext       `toml:"pod_security_context,omitempty" namespace:"pod-security-context" description:"A security context attached to each build pod"`
	InitPermissionsContainerSecurityContext           KubernetesContainerSecurityContext `toml:"init_permissions_container_security_context,omitempty" namespace:"init_permissions_container_security_context" description:"A security context attached to the init-permissions container inside the build pod"`
	BuildContainerSecurityContext                     KubernetesContainerSecurityContext `toml:"build_container_security_context,omitempty" namespace:"build_container_security_context" description:"A security context attached to the build container inside the build pod"`
	HelperContainerSecurityContext                    KubernetesContainerSecurityContext `toml:"helper_container_security_context,omitempty" namespace:"helper_container_security_context" description:"A security context attached to the helper container inside the build pod"`
	ServiceContainerSecurityContext                   KubernetesContainerSecurityContext `toml:"service_container_security_context,omitempty" namespace:"service_container_security_context" description:"A security context attached to the service containers inside the build pod"`
	Volumes                                           KubernetesVolumes                  `toml:"volumes"`
	HostAliases                                       []KubernetesHostAliases            `toml:"host_aliases,omitempty" json:"host_aliases" long:"host_aliases" description:"Add a custom host-to-IP mapping"`
	Services                                          []Service                          `toml:"services,omitempty" json:"services,omitempty" description:"Add service that is started with container"`
	CapAdd                                            []string                           `toml:"cap_add" json:"cap_add,omitempty" long:"cap-add" env:"KUBERNETES_CAP_ADD" description:"Add Linux capabilities"`
	CapDrop                                           []string                           `toml:"cap_drop" json:"cap_drop,omitempty" long:"cap-drop" env:"KUBERNETES_CAP_DROP" description:"Drop Linux capabilities"`
	DNSPolicy                                         KubernetesDNSPolicy                `toml:"dns_policy,omitempty" json:"dns_policy" long:"dns-policy" env:"KUBERNETES_DNS_POLICY" description:"How Kubernetes should try to resolve DNS from the created pods. If unset, Kubernetes will use the default 'ClusterFirst'. Valid values are: none, default, cluster-first, cluster-first-with-host-net"`
	DNSConfig                                         KubernetesDNSConfig                `toml:"dns_config" json:"dns_config" description:"Pod DNS config"`
	ContainerLifecycle                                KubernetesContainerLifecyle        `toml:"container_lifecycle,omitempty" json:"container_lifecycle,omitempty" description:"Actions that the management system should take in response to container lifecycle events"`
	PriorityClassName                                 string                             `toml:"priority_class_name,omitempty" json:"priority_class_name" long:"priority_class_name" env:"KUBERNETES_PRIORITY_CLASS_NAME" description:"If set, the Kubernetes Priority Class to be set to the Pods"`
}

//nolint:lll
type KubernetesDNSConfig struct {
	Nameservers []string                    `toml:"nameservers" json:",omitempty" description:"A list of IP addresses that will be used as DNS servers for the Pod."`
	Options     []KubernetesDNSConfigOption `toml:"options" json:",omitempty" description:"An optional list of objects where each object may have a name property (required) and a value property (optional)."`
	Searches    []string                    `toml:"searches" json:",omitempty" description:"A list of DNS search domains for hostname lookup in the Pod."`
}

type KubernetesDNSConfigOption struct {
	Name  string  `toml:"name"`
	Value *string `toml:"value,omitempty"`
}

//nolint:lll
type KubernetesVolumes struct {
	HostPaths  []KubernetesHostPath  `toml:"host_path" json:",omitempty" description:"The host paths which will be mounted"`
	PVCs       []KubernetesPVC       `toml:"pvc" json:",omitempty" description:"The persistent volume claims that will be mounted"`
	ConfigMaps []KubernetesConfigMap `toml:"config_map" json:",omitempty" description:"The config maps which will be mounted as volumes"`
	Secrets    []KubernetesSecret    `toml:"secret" json:",omitempty" description:"The secret maps which will be mounted"`
	EmptyDirs  []KubernetesEmptyDir  `toml:"empty_dir" json:",omitempty" description:"The empty dirs which will be mounted"`
	CSIs       []KubernetesCSI       `toml:"csi" json:",omitempty" description:"The CSI volumes which will be mounted"`
}

//nolint:lll
type KubernetesConfigMap struct {
	Name      string            `toml:"name" json:"name" description:"The name of the volume and ConfigMap to use"`
	MountPath string            `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath   string            `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	ReadOnly  bool              `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	Items     map[string]string `toml:"items,omitempty" json:",omitempty" description:"Key-to-path mapping for keys from the config map that should be used."`
}

//nolint:lll
type KubernetesHostPath struct {
	Name      string `toml:"name" json:"name" description:"The name of the volume"`
	MountPath string `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath   string `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	ReadOnly  bool   `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	HostPath  string `toml:"host_path,omitempty" description:"Path from the host that should be mounted as a volume"`
}

//nolint:lll
type KubernetesPVC struct {
	Name      string `toml:"name" json:"name" description:"The name of the volume and PVC to use"`
	MountPath string `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath   string `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	ReadOnly  bool   `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
}

//nolint:lll
type KubernetesSecret struct {
	Name      string            `toml:"name" json:"name" description:"The name of the volume and Secret to use"`
	MountPath string            `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath   string            `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	ReadOnly  bool              `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	Items     map[string]string `toml:"items,omitempty" json:",omitempty" description:"Key-to-path mapping for keys from the secret that should be used."`
}

//nolint:lll
type KubernetesEmptyDir struct {
	Name      string `toml:"name" json:"name" description:"The name of the volume and EmptyDir to use"`
	MountPath string `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath   string `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	Medium    string `toml:"medium,omitempty" description:"Set to 'Memory' to have a tmpfs"`
}

//nolint:lll
type KubernetesCSI struct {
	Name             string            `toml:"name" json:"name" description:"The name of the CSI volume and volumeMount to use"`
	MountPath        string            `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath          string            `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	Driver           string            `toml:"driver" description:"A string value that specifies the name of the volume driver to use."`
	FSType           string            `toml:"fs_type" description:"Filesystem type to mount. If not provided, the empty value is passed to the associated CSI driver which will determine the default filesystem to apply."`
	ReadOnly         bool              `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	VolumeAttributes map[string]string `toml:"volume_attributes,omitempty" json:",omitempty" description:"Key-value pair mapping for attributes of the CSI volume."`
}

//nolint:lll
type KubernetesPodSecurityContext struct {
	FSGroup            *int64  `toml:"fs_group,omitempty" json:",omitempty" long:"fs-group" env:"KUBERNETES_POD_SECURITY_CONTEXT_FS_GROUP" description:"A special supplemental group that applies to all containers in a pod"`
	RunAsGroup         *int64  `toml:"run_as_group,omitempty" json:",omitempty" long:"run-as-group" env:"KUBERNETES_POD_SECURITY_CONTEXT_RUN_AS_GROUP" description:"The GID to run the entrypoint of the container process"`
	RunAsNonRoot       *bool   `toml:"run_as_non_root,omitempty" json:",omitempty" long:"run-as-non-root" env:"KUBERNETES_POD_SECURITY_CONTEXT_RUN_AS_NON_ROOT" description:"Indicates that the container must run as a non-root user"`
	RunAsUser          *int64  `toml:"run_as_user,omitempty" json:",omitempty" long:"run-as-user" env:"KUBERNETES_POD_SECURITY_CONTEXT_RUN_AS_USER" description:"The UID to run the entrypoint of the container process"`
	SupplementalGroups []int64 `toml:"supplemental_groups,omitempty" json:",omitempty" long:"supplemental-groups" description:"A list of groups applied to the first process run in each container, in addition to the container's primary GID"`
	SELinuxType        string  `toml:"selinux_type,omitempty" long:"selinux-type" description:"The SELinux type label that applies to all containers in a pod"`
}

//nolint:lll
type KubernetesContainerCapabilities struct {
	Add  []api.Capability `toml:"add" json:",omitempty" long:"add" env:"@ADD" description:"List of capabilities to add to the build container"`
	Drop []api.Capability `toml:"drop" json:",omitempty" long:"drop" env:"@DROP" description:"List of capabilities to drop from the build container"`
}

//nolint:lll
type KubernetesContainerSecurityContext struct {
	Capabilities             *KubernetesContainerCapabilities `toml:"capabilities,omitempty" json:",omitempty" namespace:"capabilities" description:"The capabilities to add/drop when running the container"`
	Privileged               *bool                            `toml:"privileged" json:",omitempty" long:"privileged" env:"@PRIVILEGED" description:"Run container in privileged mode"`
	RunAsUser                *int64                           `toml:"run_as_user,omitempty" json:",omitempty" long:"run-as-user" env:"@RUN_AS_USER" description:"The UID to run the entrypoint of the container process" `
	RunAsGroup               *int64                           `toml:"run_as_group,omitempty" json:",omitempty" long:"run-as-group" env:"@RUN_AS_GROUP" description:"The GID to run the entrypoint of the container process" `
	RunAsNonRoot             *bool                            `toml:"run_as_non_root,omitempty" json:",omitempty" long:"run-as-non-root" env:"@RUN_AS_NON_ROOT" description:"Indicates that the container must run as a non-root user"`
	ReadOnlyRootFilesystem   *bool                            `toml:"read_only_root_filesystem" json:",omitempty" long:"read-only-root-filesystem" env:"@READ_ONLY_ROOT_FILESYSTEM" description:" Whether this container has a read-only root filesystem."`
	AllowPrivilegeEscalation *bool                            `toml:"allow_privilege_escalation" json:",omitempty" long:"allow-privilege-escalation" env:"@ALLOW_PRIVILEGE_ESCALATION" description:"AllowPrivilegeEscalation controls whether a process can gain more privileges than its parent process"`
	SELinuxType              string                           `toml:"selinux_type,omitempty" long:"selinux-type" description:"The SELinux type label that is associated with the container process"`
	ProcMount                api.ProcMountType                `toml:"proc_mount,omitempty" long:"proc-mount" env:"@PROC_MOUNT" description:"Denotes the type of proc mount to use for the container. Valid values: default | unmasked. Set to unmasked if this container will be used to build OCI images."`
}

func (c *KubernetesConfig) getCapabilities(defaultCapDrop []string) *api.Capabilities {
	enabled := make(map[string]bool)

	for _, v := range defaultCapDrop {
		enabled[v] = false
	}

	for _, v := range c.CapAdd {
		enabled[v] = true
	}

	for _, v := range c.CapDrop {
		enabled[v] = false
	}

	if len(enabled) < 1 {
		return nil
	}

	return buildCapabilities(enabled)
}

func buildCapabilities(enabled map[string]bool) *api.Capabilities {
	capabilities := new(api.Capabilities)

	for c, add := range enabled {
		if add {
			capabilities.Add = append(capabilities.Add, api.Capability(c))
			continue
		}
		capabilities.Drop = append(capabilities.Drop, api.Capability(c))
	}

	return capabilities
}

func (c *KubernetesContainerSecurityContext) getProcMount() *api.ProcMountType {
	caser := cases.Title(language.English)
	pm := api.ProcMountType(caser.String(strings.TrimSpace(string(c.ProcMount))))

	switch pm {
	case api.DefaultProcMount, api.UnmaskedProcMount:
		return &pm
	case "":
		logrus.Debugf("proc-mount not set")
		return nil
	default:
		logrus.Errorf("invalid proc-mount value: %s", c.ProcMount)
		return nil
	}
}

func (c *KubernetesConfig) GetContainerSecurityContext(
	securityContext KubernetesContainerSecurityContext,
	defaultCapDrop ...string,
) *api.SecurityContext {
	var seLinuxOptions *api.SELinuxOptions
	if securityContext.SELinuxType != "" {
		seLinuxOptions = &api.SELinuxOptions{Type: securityContext.SELinuxType}
	}

	return &api.SecurityContext{
		Capabilities: mergeCapabilitiesAddDrop(
			c.getCapabilities(defaultCapDrop),
			securityContext.getCapabilities(),
		),
		Privileged: getContainerSecurityContextEffectiveFlagValue(securityContext.Privileged, c.Privileged),
		AllowPrivilegeEscalation: getContainerSecurityContextEffectiveFlagValue(
			securityContext.AllowPrivilegeEscalation,
			c.AllowPrivilegeEscalation,
		),
		RunAsGroup:             securityContext.RunAsGroup,
		RunAsNonRoot:           securityContext.RunAsNonRoot,
		RunAsUser:              securityContext.RunAsUser,
		ReadOnlyRootFilesystem: securityContext.ReadOnlyRootFilesystem,
		ProcMount:              securityContext.getProcMount(),
		SELinuxOptions:         seLinuxOptions,
	}
}

func mergeCapabilitiesAddDrop(capabilities ...*api.Capabilities) *api.Capabilities {
	merged := &api.Capabilities{}
	for _, c := range capabilities {
		if c == nil {
			continue
		}

		if c.Add != nil {
			merged.Add = c.Add
		}

		if c.Drop != nil {
			merged.Drop = c.Drop
		}
	}

	if merged.Add == nil && merged.Drop == nil {
		return nil
	}

	return merged
}

func getContainerSecurityContextEffectiveFlagValue(containerValue, fallbackValue *bool) *bool {
	if containerValue == nil {
		return fallbackValue
	}

	return containerValue
}

func (c *KubernetesContainerSecurityContext) getCapabilities() *api.Capabilities {
	capabilities := c.Capabilities
	if capabilities == nil {
		return nil
	}

	return &api.Capabilities{
		Add:  capabilities.Add,
		Drop: capabilities.Drop,
	}
}

//nolint:lll
type KubernetesAffinity struct {
	NodeAffinity    *KubernetesNodeAffinity    `toml:"node_affinity,omitempty" json:"node_affinity,omitempty" long:"node-affinity" description:"Node affinity is conceptually similar to nodeSelector -- it allows you to constrain which nodes your pod is eligible to be scheduled on, based on labels on the node."`
	PodAffinity     *KubernetesPodAffinity     `toml:"pod_affinity,omitempty" json:"pod_affinity,omitempty" description:"Pod affinity allows to constrain which nodes your pod is eligible to be scheduled on based on the labels on other pods."`
	PodAntiAffinity *KubernetesPodAntiAffinity `toml:"pod_anti_affinity,omitempty" json:"pod_anti_affinity,omitempty" description:"Pod anti-affinity allows to constrain which nodes your pod is eligible to be scheduled on based on the labels on other pods."`
}

//nolint:lll
type KubernetesNodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector             `toml:"required_during_scheduling_ignored_during_execution,omitempty" json:"required_during_scheduling_ignored_during_execution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `toml:"preferred_during_scheduling_ignored_during_execution,omitempty" json:"preferred_during_scheduling_ignored_during_execution"`
}

//nolint:lll
type KubernetesPodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `toml:"required_during_scheduling_ignored_during_execution,omitempty" json:"required_during_scheduling_ignored_during_execution"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `toml:"preferred_during_scheduling_ignored_during_execution,omitempty" json:"preferred_during_scheduling_ignored_during_execution"`
}

//nolint:lll
type KubernetesPodAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `toml:"required_during_scheduling_ignored_during_execution,omitempty" json:"required_during_scheduling_ignored_during_execution"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `toml:"preferred_during_scheduling_ignored_during_execution,omitempty" json:"preferred_during_scheduling_ignored_during_execution"`
}

//nolint:lll
type KubernetesHostAliases struct {
	IP        string   `toml:"ip" json:"ip" long:"ip" description:"The IP address you want to attach hosts to"`
	Hostnames []string `toml:"hostnames" json:"hostnames" long:"hostnames" description:"A list of hostnames that will be attached to the IP"`
}

// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#lifecycle-v1-core
//
//nolint:lll
type KubernetesContainerLifecyle struct {
	PostStart *KubernetesLifecycleHandler `toml:"post_start,omitempty" json:"post_start,omitempty" description:"PostStart is called immediately after a container is created. If the handler fails, the container is terminated and restarted according to its restart policy. Other management of the container blocks until the hook completes"`
	PreStop   *KubernetesLifecycleHandler `toml:"pre_stop,omitempty" json:"pre_stop,omitempty" description:"PreStop is called immediately before a container is terminated due to an API request or management event such as liveness/startup probe failure, preemption, resource contention, etc. The handler is not called if the container crashes or exits. The reason for termination is passed to the handler. The Pod's termination grace period countdown begins before the PreStop hooked is executed. Regardless of the outcome of the handler, the container will eventually terminate within the Pod's termination grace period. Other management of the container blocks until the hook completes or until the termination grace period is reached"`
}

//nolint:lll
type KubernetesLifecycleHandler struct {
	Exec      *KubernetesLifecycleExecAction `toml:"exec"  json:"exec,omitempty" description:"Exec specifies the action to take"`
	HTTPGet   *KubernetesLifecycleHTTPGet    `toml:"http_get"  json:"http_get,omitempty" description:"HTTPGet specifies the http request to perform."`
	TCPSocket *KubernetesLifecycleTCPSocket  `toml:"tcp_socket"  json:"tcp_socket,omitempty" description:"TCPSocket specifies an action involving a TCP port"`
}

//nolint:lll
type KubernetesLifecycleExecAction struct {
	Command []string `toml:"command" json:"command" description:"Command is the command line to execute inside the container, the working directory for the command  is root ('/') in the container's filesystem. The command is simply exec'd, it is not run inside a shell, so traditional shell instructions ('|', etc) won't work. To use a shell, you need to explicitly call out to that shell. Exit status of 0 is treated as live/healthy and non-zero is unhealthy"`
}

//nolint:lll
type KubernetesLifecycleHTTPGet struct {
	Host        string                             `toml:"host" json:"host" description:"Host name to connect to, defaults to the pod IP. You probably want to set \"Host\" in httpHeaders instead"`
	HTTPHeaders []KubernetesLifecycleHTTPGetHeader `toml:"http_headers" json:"http_headers" description:"Custom headers to set in the request. HTTP allows repeated headers"`
	Path        string                             `toml:"path" json:"path" description:"Path to access on the HTTP server"`
	Port        int                                `toml:"port" json:"port" description:"Number of the port to access on the container. Number must be in the range 1 to 65535"`
	Scheme      string                             `toml:"scheme" json:"scheme" description:"Scheme to use for connecting to the host. Defaults to HTTP"`
}

type KubernetesLifecycleHTTPGetHeader struct {
	Name  string `toml:"name" json:"name" description:"The header field name"`
	Value string `toml:"value" json:"value" description:"The header field value"`
}

//nolint:lll
type KubernetesLifecycleTCPSocket struct {
	Host string `toml:"host" json:"host" description:"Host name to connect to, defaults to the pod IP. You probably want to set \"Host\" in httpHeaders instead"`
	Port int    `toml:"port" json:"port" description:"Number of the port to access on the container. Number must be in the range 1 to 65535"`
}

// ToKubernetesLifecycleHandler converts our lifecycle structs to the ones from the Kubernetes API.
// We can't use them directly since they don't suppor toml.
func (h *KubernetesLifecycleHandler) ToKubernetesLifecycleHandler() *api.LifecycleHandler {
	kubeHandler := &api.LifecycleHandler{}

	if h.Exec != nil {
		kubeHandler.Exec = &api.ExecAction{
			Command: h.Exec.Command,
		}
	}
	if h.HTTPGet != nil {
		httpHeaders := []api.HTTPHeader{}

		for _, e := range h.HTTPGet.HTTPHeaders {
			httpHeaders = append(httpHeaders, api.HTTPHeader{
				Name:  e.Name,
				Value: e.Value,
			})
		}

		kubeHandler.HTTPGet = &api.HTTPGetAction{
			Host:        h.HTTPGet.Host,
			Port:        intstr.FromInt(h.HTTPGet.Port),
			Path:        h.HTTPGet.Path,
			Scheme:      api.URIScheme(h.HTTPGet.Scheme),
			HTTPHeaders: httpHeaders,
		}
	}
	if h.TCPSocket != nil {
		kubeHandler.TCPSocket = &api.TCPSocketAction{
			Host: h.TCPSocket.Host,
			Port: intstr.FromInt(h.TCPSocket.Port),
		}
	}

	return kubeHandler
}

type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `toml:"node_selector_terms" json:"node_selector_terms"`
}

type PreferredSchedulingTerm struct {
	Weight     int32            `toml:"weight" json:"weight"`
	Preference NodeSelectorTerm `toml:"preference" json:"preference"`
}

type WeightedPodAffinityTerm struct {
	Weight          int32           `toml:"weight" json:"weight"`
	PodAffinityTerm PodAffinityTerm `toml:"pod_affinity_term" json:"pod_affinity_term"`
}

type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `toml:"match_expressions,omitempty" json:"match_expressions"`
	MatchFields      []NodeSelectorRequirement `toml:"match_fields,omitempty" json:"match_fields"`
}

//nolint:lll
type NodeSelectorRequirement struct {
	Key      string   `toml:"key,omitempty" json:"key"`
	Operator string   `toml:"operator,omitempty" json:"operator"`
	Values   []string `toml:"values,omitempty" json:"values"`
}

type PodAffinityTerm struct {
	LabelSelector     *LabelSelector `toml:"label_selector,omitempty" json:"label_selector,omitempty"`
	Namespaces        []string       `toml:"namespaces,omitempty" json:"namespaces"`
	TopologyKey       string         `toml:"topology_key,omitempty" json:"topology_key"`
	NamespaceSelector *LabelSelector `toml:"namespace_selector,omitempty" json:"namespace_selector,omitempty"`
}

type LabelSelector struct {
	MatchLabels      map[string]string         `toml:"match_labels,omitempty" json:"match_labels"`
	MatchExpressions []NodeSelectorRequirement `toml:"match_expressions,omitempty" json:"match_expressions"`
}

//nolint:lll
type Service struct {
	Name        string   `toml:"name" long:"name" description:"The image path for the service"`
	Alias       string   `toml:"alias,omitempty" long:"alias" description:"Space or comma-separated aliases of the service."`
	Command     []string `toml:"command" json:",omitempty" long:"command" description:"Command or script that should be used as the container’s command. Syntax is similar to https://docs.docker.com/engine/reference/builder/#cmd"`
	Entrypoint  []string `toml:"entrypoint" json:",omitempty" long:"entrypoint" description:"Command or script that should be executed as the container’s entrypoint. syntax is similar to https://docs.docker.com/engine/reference/builder/#entrypoint"`
	Environment []string `toml:"environment,omitempty" json:"environment,omitempty" long:"env" description:"Custom environment variables injected to service environment"`
}

func (s *Service) Aliases() []string { return strings.Fields(strings.ReplaceAll(s.Alias, ",", " ")) }

func (s *Service) ToImageDefinition() Image {
	image := Image{
		Name:       s.Name,
		Alias:      s.Alias,
		Command:    s.Command,
		Entrypoint: s.Entrypoint,
	}

	for _, environment := range s.Environment {
		if variable, err := ParseVariable(environment); err == nil {
			variable.Internal = true
			image.Variables = append(image.Variables, variable)
		}
	}

	return image
}

//nolint:lll
type RunnerCredentials struct {
	URL             string    `toml:"url" json:"url" short:"u" long:"url" env:"CI_SERVER_URL" required:"true" description:"GitLab instance URL" jsonschema:"minLength=1"`
	ID              int64     `toml:"id" json:"id" description:"Runner ID"`
	Token           string    `toml:"token" json:"token" short:"t" long:"token" env:"CI_SERVER_TOKEN" required:"true" description:"Runner token" jsonschema:"minLength=1"`
	TokenObtainedAt time.Time `toml:"token_obtained_at" json:"token_obtained_at" description:"When the runner token was obtained"`
	TokenExpiresAt  time.Time `toml:"token_expires_at" json:"token_expires_at" description:"Runner token expiration time"`
	TLSCAFile       string    `toml:"tls-ca-file,omitempty" json:"tls-ca-file" long:"tls-ca-file" env:"CI_SERVER_TLS_CA_FILE" description:"File containing the certificates to verify the peer when using HTTPS"`
	TLSCertFile     string    `toml:"tls-cert-file,omitempty" json:"tls-cert-file" long:"tls-cert-file" env:"CI_SERVER_TLS_CERT_FILE" description:"File containing certificate for TLS client auth when using HTTPS"`
	TLSKeyFile      string    `toml:"tls-key-file,omitempty" json:"tls-key-file" long:"tls-key-file" env:"CI_SERVER_TLS_KEY_FILE" description:"File containing private key for TLS client auth when using HTTPS"`
}

//nolint:lll
type CacheGCSCredentials struct {
	AccessID   string `toml:"AccessID,omitempty" long:"access-id" env:"CACHE_GCS_ACCESS_ID" description:"ID of GCP Service Account used to access the storage"`
	PrivateKey string `toml:"PrivateKey,omitempty" long:"private-key" env:"CACHE_GCS_PRIVATE_KEY" description:"Private key used to sign GCS requests"`
}

//nolint:lll
type CacheGCSConfig struct {
	CacheGCSCredentials
	CredentialsFile string `toml:"CredentialsFile,omitempty" long:"credentials-file" env:"GOOGLE_APPLICATION_CREDENTIALS" description:"File with GCP credentials, containing AccessID and PrivateKey"`
	BucketName      string `toml:"BucketName,omitempty" long:"bucket-name" env:"CACHE_GCS_BUCKET_NAME" description:"Name of the bucket where cache will be stored"`
}

//nolint:lll
type CacheS3Config struct {
	ServerAddress             string     `toml:"ServerAddress,omitempty" long:"server-address" env:"CACHE_S3_SERVER_ADDRESS" description:"A host:port to the used S3-compatible server"`
	AccessKey                 string     `toml:"AccessKey,omitempty" long:"access-key" env:"CACHE_S3_ACCESS_KEY" description:"S3 Access Key"`
	SecretKey                 string     `toml:"SecretKey,omitempty" long:"secret-key" env:"CACHE_S3_SECRET_KEY" description:"S3 Secret Key"`
	BucketName                string     `toml:"BucketName,omitempty" long:"bucket-name" env:"CACHE_S3_BUCKET_NAME" description:"Name of the bucket where cache will be stored"`
	BucketLocation            string     `toml:"BucketLocation,omitempty" long:"bucket-location" env:"CACHE_S3_BUCKET_LOCATION" description:"Name of S3 region"`
	Insecure                  bool       `toml:"Insecure,omitempty" long:"insecure" env:"CACHE_S3_INSECURE" description:"Use insecure mode (without https)"`
	AuthenticationType        S3AuthType `toml:"AuthenticationType,omitempty" long:"authentication_type" env:"CACHE_S3_AUTHENTICATION_TYPE" description:"IAM or credentials"`
	ServerSideEncryption      string     `toml:"ServerSideEncryption,omitempty" long:"server-side-encryption" env:"CACHE_S3_SERVER_SIDE_ENCRYPTION" description:"Server side encryption type (S3, or KMS)"`
	ServerSideEncryptionKeyID string     `toml:"ServerSideEncryptionKeyID,omitempty" long:"server-side-encryption-key-id" env:"CACHE_S3_SERVER_SIDE_ENCRYPTION_KEY_ID" description:"Server side encryption key ID (alias or Key ID)"`
}

//nolint:lll
type CacheAzureCredentials struct {
	AccountName string `toml:"AccountName,omitempty" long:"account-name" env:"CACHE_AZURE_ACCOUNT_NAME" description:"Account name for Azure Blob Storage"`
	AccountKey  string `toml:"AccountKey,omitempty" long:"account-key" env:"CACHE_AZURE_ACCOUNT_KEY" description:"Access key for Azure Blob Storage"`
}

//nolint:lll
type CacheAzureConfig struct {
	CacheAzureCredentials
	ContainerName string `toml:"ContainerName,omitempty" long:"container-name" env:"CACHE_AZURE_CONTAINER_NAME" description:"Name of the Azure container where cache will be stored"`
	StorageDomain string `toml:"StorageDomain,omitempty" long:"storage-domain" env:"CACHE_AZURE_STORAGE_DOMAIN" description:"Domain name of the Azure storage (e.g. blob.core.windows.net)"`
}

//nolint:lll
type CacheConfig struct {
	Type                   string `toml:"Type,omitempty" long:"type" env:"CACHE_TYPE" description:"Select caching method"`
	Path                   string `toml:"Path,omitempty" long:"path" env:"CACHE_PATH" description:"Name of the path to prepend to the cache URL"`
	Shared                 bool   `toml:"Shared,omitempty" long:"shared" env:"CACHE_SHARED" description:"Enable cache sharing between runners."`
	MaxUploadedArchiveSize int64  `toml:"MaxUploadedArchiveSize,omitempty" long:"max_uploaded_archive_size" env:"CACHE_MAXIMUM_UPLOADED_ARCHIVE_SIZE" description:"Limit the size of the cache archive being uploaded to cloud storage, in bytes."`

	S3    *CacheS3Config    `toml:"s3,omitempty" json:"s3,omitempty" namespace:"s3"`
	GCS   *CacheGCSConfig   `toml:"gcs,omitempty" json:"gcs,omitempty" namespace:"gcs"`
	Azure *CacheAzureConfig `toml:"azure,omitempty" json:"azure,omitempty" namespace:"azure"`
}

//nolint:lll
type RunnerSettings struct {
	Executor  string `toml:"executor" json:"executor" long:"executor" env:"RUNNER_EXECUTOR" required:"true" description:"Select executor, eg. shell, docker, etc."`
	BuildsDir string `toml:"builds_dir,omitempty" json:"builds_dir" long:"builds-dir" env:"RUNNER_BUILDS_DIR" description:"Directory where builds are stored"`
	CacheDir  string `toml:"cache_dir,omitempty" json:"cache_dir" long:"cache-dir" env:"RUNNER_CACHE_DIR" description:"Directory where build cache is stored"`
	CloneURL  string `toml:"clone_url,omitempty" json:"clone_url" long:"clone-url" env:"CLONE_URL" description:"Overwrite the default URL used to clone or fetch the git ref"`

	Environment []string `toml:"environment,omitempty" json:"environment,omitempty" long:"env" env:"RUNNER_ENV" description:"Custom environment variables injected to build environment"`

	// DEPRECATED
	// TODO: Remove in 16.0. For more details read https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29405
	PreCloneScript string `toml:"pre_clone_script,omitempty" json:"pre_clone_script" long:"pre-clone-script" env:"RUNNER_PRE_CLONE_SCRIPT" description:"[DEPRECATED] Use pre_get_sources_script instead"`
	// DEPRECATED
	// TODO: Remove in 16.0. For more details read https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29405
	PostCloneScript string `toml:"post_clone_script,omitempty" json:"post_clone_script" long:"post-clone-script" env:"RUNNER_POST_CLONE_SCRIPT" description:"[DEPRECATED] Use post_get_sources_script instead"`

	PreGetSourcesScript  string `toml:"pre_get_sources_script,omitempty" json:"pre_get_sources_script" long:"pre-get-sources-script" env:"RUNNER_PRE_GET_SOURCES_SCRIPT" description:"Runner-specific commands to be executed on the runner before updating the Git repository an updating submodules."`
	PostGetSourcesScript string `toml:"post_get_sources_script,omitempty" json:"post_get_sources_script" long:"post-get-sources-script" env:"RUNNER_POST_GET_SOURCES_SCRIPT" description:"Runner-specific commands to be executed on the runner after updating the Git repository and updating submodules."`

	PreBuildScript  string `toml:"pre_build_script,omitempty" json:"pre_build_script" long:"pre-build-script" env:"RUNNER_PRE_BUILD_SCRIPT" description:"Runner-specific command script executed just before build executes"`
	PostBuildScript string `toml:"post_build_script,omitempty" json:"post_build_script" long:"post-build-script" env:"RUNNER_POST_BUILD_SCRIPT" description:"Runner-specific command script executed just after build executes"`

	DebugTraceDisabled bool `toml:"debug_trace_disabled,omitempty" json:"debug_trace_disabled" long:"debug-trace-disabled" env:"RUNNER_DEBUG_TRACE_DISABLED" description:"When set to true Runner will disable the possibility of using the CI_DEBUG_TRACE feature"`

	Shell          string           `toml:"shell,omitempty" json:"shell" long:"shell" env:"RUNNER_SHELL" description:"Select bash, sh, cmd, pwsh or powershell" jsonschema:"enum=bash,enum=sh,enum=cmd,enum=pwsh,enum=powershell,enum="`
	CustomBuildDir *CustomBuildDir  `toml:"custom_build_dir,omitempty" json:"custom_build_dir,omitempty" group:"custom build dir configuration" namespace:"custom_build_dir"`
	Referees       *referees.Config `toml:"referees,omitempty" json:"referees,omitempty" group:"referees configuration" namespace:"referees"`
	Cache          *CacheConfig     `toml:"cache,omitempty" json:"cache,omitempty" group:"cache configuration" namespace:"cache"`

	// GracefulKillTimeout and ForceKillTimeout aren't exposed to the users yet
	// because not every executor supports it. We also have to keep in mind that
	// the CustomConfig has its configuration fields for termination so when
	// every executor supports graceful termination we should expose this single
	// configuration for all executors.
	GracefulKillTimeout *int `toml:"-" json:",omitempty"`
	ForceKillTimeout    *int `toml:"-" json:",omitempty"`

	FeatureFlags map[string]bool `toml:"feature_flags" json:"feature_flags,omitempty" long:"feature-flags" env:"FEATURE_FLAGS" description:"Enable/Disable feature flags https://docs.gitlab.com/runner/configuration/feature-flags.html"`

	Instance   *InstanceConfig   `toml:"instance,omitempty" json:"instance,omitempty"`
	SSH        *ssh.Config       `toml:"ssh,omitempty" json:"ssh,omitempty" group:"ssh executor" namespace:"ssh"`
	Docker     *DockerConfig     `toml:"docker,omitempty" json:"docker,omitempty" group:"docker executor" namespace:"docker"`
	Parallels  *ParallelsConfig  `toml:"parallels,omitempty" json:"parallels,omitempty" group:"parallels executor" namespace:"parallels"`
	VirtualBox *VirtualBoxConfig `toml:"virtualbox,omitempty" json:"virtualbox,omitempty" group:"virtualbox executor" namespace:"virtualbox"`
	Machine    *DockerMachine    `toml:"machine,omitempty" json:"machine,omitempty" group:"docker machine provider" namespace:"machine"`
	Kubernetes *KubernetesConfig `toml:"kubernetes,omitempty" json:"kubernetes,omitempty" group:"kubernetes executor" namespace:"kubernetes"`
	Custom     *CustomConfig     `toml:"custom,omitempty" json:"custom,omitempty" group:"custom executor" namespace:"custom"`

	Autoscaler *AutoscalerConfig `toml:"autoscaler,omitempty" json:",omitempty"`
}

//nolint:lll
type RunnerConfig struct {
	Name               string `toml:"name" json:"name" short:"name" long:"description" env:"RUNNER_NAME" description:"Runner name" jsonschema:"minLength=1"`
	Limit              int    `toml:"limit,omitzero" json:"limit" long:"limit" env:"RUNNER_LIMIT" description:"Maximum number of builds processed by this runner"`
	OutputLimit        int    `toml:"output_limit,omitzero" long:"output-limit" env:"RUNNER_OUTPUT_LIMIT" description:"Maximum build trace size in kilobytes"`
	RequestConcurrency int    `toml:"request_concurrency,omitzero" long:"request-concurrency" env:"RUNNER_REQUEST_CONCURRENCY" description:"Maximum concurrency for job requests" jsonschema:"min=1"`

	UnhealthyRequestsLimit int            `toml:"unhealthy_requests_limit,omitzero" long:"unhealthy-requests-limit" env:"RUNNER_UNHEALTHY_REQUESTS_LIMIT" description:"The number of 'unhealthy' responses to new job requests after which a runner worker will be disabled"`
	UnhealthyInterval      *time.Duration `toml:"unhealthy_interval,omitzero" json:",omitempty" long:"unhealthy-interval" ENV:"RUNNER_UNHEALTHY_INTERVAL" description:"Duration for which a runner worker is disabled after exceeding the unhealthy requests limit. Supports syntax like '3600s', '1h30min' etc"`

	SystemIDState *SystemIDState `toml:"-" json:",omitempty"`

	RunnerCredentials
	RunnerSettings
}

//nolint:lll
type SessionServer struct {
	ListenAddress    string `toml:"listen_address,omitempty" json:"listen_address" description:"Address that the runner will communicate directly with"`
	AdvertiseAddress string `toml:"advertise_address,omitempty" json:"advertise_address" description:"Address the runner will expose to the world to connect to the session server"`
	SessionTimeout   int    `toml:"session_timeout,omitempty" json:"session_timeout" description:"How long a terminal session can be active after a build completes, in seconds"`
}

//nolint:lll
type Config struct {
	ListenAddress string        `toml:"listen_address,omitempty" json:"listen_address"`
	SessionServer SessionServer `toml:"session_server,omitempty" json:"session_server"`

	Concurrent    int             `toml:"concurrent" json:"concurrent"`
	CheckInterval int             `toml:"check_interval" json:"check_interval" description:"Define active checking interval of jobs"`
	LogLevel      *string         `toml:"log_level" json:"log_level,omitempty" description:"Define log level (one of: panic, fatal, error, warning, info, debug)"`
	LogFormat     *string         `toml:"log_format" json:"log_format,omitempty" description:"Define log format (one of: runner, text, json)"`
	User          string          `toml:"user,omitempty" json:"user"`
	Runners       []*RunnerConfig `toml:"runners" json:"runners"`
	SentryDSN     *string         `toml:"sentry_dsn" json:",omitempty"`
	ModTime       time.Time       `toml:"-"`
	Loaded        bool            `toml:"-"`

	ShutdownTimeout int `toml:"shutdown_timeout,omitempty" json:"shutdown_timeout" description:"Number of seconds until the forceful shutdown operation times out and exits the process"`

	configSaver ConfigSaver
}

//go:generate mockery --name=ConfigSaver --inpackage
type ConfigSaver interface {
	Save(filePath string, data []byte) error
}

type defaultConfigSaver struct{}

func (s *defaultConfigSaver) Save(filePath string, data []byte) error {
	// create directory to store configuration
	err := os.MkdirAll(filepath.Dir(filePath), 0700)
	if err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// write config file
	err = os.WriteFile(filePath, data, 0o600)
	if err != nil {
		return fmt.Errorf("saving the file: %w", err)
	}

	return nil
}

//nolint:lll
type CustomBuildDir struct {
	Enabled bool `toml:"enabled,omitempty" json:"enabled" long:"enabled" env:"CUSTOM_BUILD_DIR_ENABLED" description:"Enable job specific build directories"`
}

type S3AuthType string

const (
	S3AuthTypeAccessKey S3AuthType = "access-key"
	S3AuthTypeIAM       S3AuthType = "iam"
)

func (c *CacheS3Config) AuthType() S3AuthType {
	authType := S3AuthType(strings.ToLower(string(c.AuthenticationType)))

	switch authType {
	case S3AuthTypeAccessKey, S3AuthTypeIAM:
		return authType
	}

	if authType != "" {
		return ""
	}

	if c.ServerAddress == "" || c.AccessKey == "" || c.SecretKey == "" {
		return S3AuthTypeIAM
	}

	return S3AuthTypeAccessKey
}

func (c *CacheConfig) GetPath() string {
	return c.Path
}

func (c *CacheConfig) GetShared() bool {
	return c.Shared
}

func (r *RunnerSettings) GetGracefulKillTimeout() time.Duration {
	return getDuration(r.GracefulKillTimeout, process.GracefulTimeout)
}

func (r *RunnerSettings) GetForceKillTimeout() time.Duration {
	return getDuration(r.ForceKillTimeout, process.KillTimeout)
}

// IsFeatureFlagOn check if the specified feature flag is on. If the feature
// flag is not configured it will return the default value.
func (r *RunnerSettings) IsFeatureFlagOn(name string) bool {
	if r.IsFeatureFlagDefined(name) {
		return r.FeatureFlags[name]
	}

	for _, ff := range featureflags.GetAll() {
		if ff.Name == name {
			return ff.DefaultValue
		}
	}

	return false
}

// IsFeatureFlagDefined checks if the feature flag is defined in the runner
// configuration.
func (r *RunnerSettings) IsFeatureFlagDefined(name string) bool {
	_, ok := r.FeatureFlags[name]

	return ok
}

func (r *RunnerSettings) GetPreGetSourcesScript() string {
	if r.PreGetSourcesScript != "" {
		return r.PreGetSourcesScript
	}

	return r.PreCloneScript
}

func (r *RunnerSettings) GetPostGetSourcesScript() string {
	if r.PostGetSourcesScript != "" {
		return r.PostGetSourcesScript
	}

	return r.PostCloneScript
}

func getDuration(source *int, defaultValue time.Duration) time.Duration {
	if source == nil {
		return defaultValue
	}

	timeout := *source
	if timeout <= 0 {
		return defaultValue
	}

	return time.Duration(timeout) * time.Second
}

func (c *SessionServer) GetSessionTimeout() time.Duration {
	if c.SessionTimeout > 0 {
		return time.Duration(c.SessionTimeout) * time.Second
	}

	return DefaultSessionTimeout
}

func (c *DockerConfig) GetNanoCPUs() (int64, error) {
	if c.CPUS == "" {
		return 0, nil
	}

	cpu, ok := new(big.Rat).SetString(c.CPUS)
	if !ok {
		return 0, fmt.Errorf("failed to parse %v as a rational number", c.CPUS)
	}

	nano, _ := cpu.Mul(cpu, big.NewRat(1e9, 1)).Float64()

	return int64(nano), nil
}

func (c *DockerConfig) getMemoryBytes(size string, fieldName string) int64 {
	if size == "" {
		return 0
	}

	bytes, err := units.RAMInBytes(size)
	if err != nil {
		logrus.Fatalf("Error parsing docker %s: %s", fieldName, err)
	}

	return bytes
}

func (c *DockerConfig) GetMemory() int64 {
	return c.getMemoryBytes(c.Memory, "memory")
}

func (c *DockerConfig) GetMemorySwap() int64 {
	return c.getMemoryBytes(c.MemorySwap, "memory_swap")
}

func (c *DockerConfig) GetMemoryReservation() int64 {
	return c.getMemoryBytes(c.MemoryReservation, "memory_reservation")
}

func (c *DockerConfig) GetOomKillDisable() *bool {
	return &c.OomKillDisable
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

func (c *KubernetesConfig) GetResourceAvailabilityCheckMaxAttempts() int {
	if c.ResourceAvailabilityCheckMaxAttempts < 0 {
		c.ResourceAvailabilityCheckMaxAttempts = KubernetesResourceAvailabilityCheckMaxAttempts
	}

	return c.ResourceAvailabilityCheckMaxAttempts
}

func (c *KubernetesConfig) GetNodeTolerations() []api.Toleration {
	var tolerations []api.Toleration

	for toleration, effect := range c.NodeTolerations {
		newToleration := api.Toleration{
			Effect: api.TaintEffect(effect),
		}

		if strings.Contains(toleration, "=") {
			parts := strings.Split(toleration, "=")
			newToleration.Key = parts[0]
			if len(parts) > 1 {
				newToleration.Value = parts[1]
			}
			newToleration.Operator = api.TolerationOpEqual
		} else {
			newToleration.Key = toleration
			newToleration.Operator = api.TolerationOpExists
		}

		tolerations = append(tolerations, newToleration)
	}

	return tolerations
}

func (c *KubernetesConfig) GetPodSecurityContext() *api.PodSecurityContext {
	podSecurityContext := c.PodSecurityContext

	if podSecurityContext.FSGroup == nil &&
		podSecurityContext.RunAsGroup == nil &&
		podSecurityContext.RunAsNonRoot == nil &&
		podSecurityContext.RunAsUser == nil &&
		len(podSecurityContext.SupplementalGroups) == 0 &&
		podSecurityContext.SELinuxType == "" {
		return nil
	}

	var seLinuxOptions *api.SELinuxOptions
	if podSecurityContext.SELinuxType != "" {
		seLinuxOptions = &api.SELinuxOptions{Type: podSecurityContext.SELinuxType}
	}

	return &api.PodSecurityContext{
		FSGroup:            podSecurityContext.FSGroup,
		RunAsGroup:         podSecurityContext.RunAsGroup,
		RunAsNonRoot:       podSecurityContext.RunAsNonRoot,
		RunAsUser:          podSecurityContext.RunAsUser,
		SupplementalGroups: podSecurityContext.SupplementalGroups,
		SELinuxOptions:     seLinuxOptions,
	}
}

// GetCleanupGracePeriodSeconds returns the effective value of CleanupGracePeriodSeconds
// depending on TerminationGracePeriodSeconds.
// Support for TerminationGracePeriodSeconds will be removed with
// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28165.
func (c *KubernetesConfig) GetCleanupGracePeriodSeconds() *int64 {
	if c.TerminationGracePeriodSeconds != nil {
		return c.TerminationGracePeriodSeconds
	}

	return c.CleanupGracePeriodSeconds
}

// GetPodTerminationGracePeriodSeconds returns the effective value of PodTerminationGracePeriodSeconds
// depending on TerminationGracePeriodSeconds.
// Support for TerminationGracePeriodSeconds will be removed with
// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28165.
func (c *KubernetesConfig) GetPodTerminationGracePeriodSeconds() *int64 {
	if c.TerminationGracePeriodSeconds != nil {
		return c.TerminationGracePeriodSeconds
	}

	// For backwards compatibility, the default value of the Pod termination should remain zero since that means
	//nolint:lll
	// "terminate immediately" as opposed to nil, which by default is a timeout of 30 seconds as per http://gitlab.com/gitlab-org/gitlab-runner/blob/45472cdf02591942c9a95d2ce38ef5ff3a38d842/vendor/k8s.io/api/core/v1/types.go#L2988-2988.
	// For details refer to https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2130.
	// Will be removed with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28165.
	var defaultPodTerminationGracePeriod int64
	if c.PodTerminationGracePeriodSeconds == nil {
		return &defaultPodTerminationGracePeriod
	}

	return c.PodTerminationGracePeriodSeconds
}

func (c *KubernetesConfig) GetAffinity() *api.Affinity {
	var affinity api.Affinity

	if c.Affinity.NodeAffinity != nil {
		affinity.NodeAffinity = c.GetNodeAffinity()
	}

	if c.Affinity.PodAffinity != nil {
		affinity.PodAffinity = c.GetPodAffinity()
	}

	if c.Affinity.PodAntiAffinity != nil {
		affinity.PodAntiAffinity = c.GetPodAntiAffinity()
	}

	return &affinity
}

func (c *KubernetesConfig) GetDNSConfig() *api.PodDNSConfig {
	if len(c.DNSConfig.Nameservers) == 0 && len(c.DNSConfig.Searches) == 0 && len(c.DNSConfig.Options) == 0 {
		return nil
	}

	var config api.PodDNSConfig

	config.Nameservers = c.DNSConfig.Nameservers
	config.Searches = c.DNSConfig.Searches

	for _, opt := range c.DNSConfig.Options {
		config.Options = append(config.Options, api.PodDNSConfigOption{
			Name:  opt.Name,
			Value: opt.Value,
		})
	}

	return &config
}

func (c *KubernetesConfig) GetNodeAffinity() *api.NodeAffinity {
	var nodeAffinity api.NodeAffinity

	if c.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		nodeSelector := c.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.GetNodeSelector()
		nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = nodeSelector
	}

	for _, preferred := range c.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			preferred.GetPreferredSchedulingTerm(),
		)
	}
	return &nodeAffinity
}

// GetContainerLifecycle returns the container lifecycle configuration
func (c *KubernetesConfig) GetContainerLifecycle() KubernetesContainerLifecyle {
	return c.ContainerLifecycle
}

func (c *NodeSelector) GetNodeSelector() *api.NodeSelector {
	var nodeSelector api.NodeSelector
	for _, selector := range c.NodeSelectorTerms {
		nodeSelector.NodeSelectorTerms = append(nodeSelector.NodeSelectorTerms, selector.GetNodeSelectorTerm())
	}
	return &nodeSelector
}

func (c *NodeSelectorRequirement) GetNodeSelectorRequirement() api.NodeSelectorRequirement {
	return api.NodeSelectorRequirement{
		Key:      c.Key,
		Operator: api.NodeSelectorOperator(c.Operator),
		Values:   c.Values,
	}
}

func (c *LabelSelector) GetLabelSelectorMatchExpressions() []metav1.LabelSelectorRequirement {
	var labelSelectorRequirement []metav1.LabelSelectorRequirement

	for _, label := range c.MatchExpressions {
		var expression = metav1.LabelSelectorRequirement{
			Key:      label.Key,
			Operator: metav1.LabelSelectorOperator(label.Operator),
			Values:   label.Values,
		}
		labelSelectorRequirement = append(labelSelectorRequirement, expression)
	}

	return labelSelectorRequirement
}

func (c *KubernetesConfig) GetPodAffinity() *api.PodAffinity {
	var podAffinity api.PodAffinity

	for _, required := range c.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		podAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
			podAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			required.GetPodAffinityTerm(),
		)
	}

	for _, preferred := range c.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		podAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			podAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			preferred.GetWeightedPodAffinityTerm(),
		)
	}

	return &podAffinity
}

func (c *KubernetesConfig) GetPodAntiAffinity() *api.PodAntiAffinity {
	var podAntiAffinity api.PodAntiAffinity

	for _, required := range c.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
			podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			required.GetPodAffinityTerm(),
		)
	}

	for _, preferred := range c.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			preferred.GetWeightedPodAffinityTerm(),
		)
	}

	return &podAntiAffinity
}

func (c *PodAffinityTerm) GetPodAffinityTerm() api.PodAffinityTerm {
	return api.PodAffinityTerm{
		LabelSelector:     c.GetLabelSelector(),
		Namespaces:        c.Namespaces,
		TopologyKey:       c.TopologyKey,
		NamespaceSelector: c.GetNamespaceSelector(),
	}
}

func (c *WeightedPodAffinityTerm) GetWeightedPodAffinityTerm() api.WeightedPodAffinityTerm {
	return api.WeightedPodAffinityTerm{
		Weight:          c.Weight,
		PodAffinityTerm: c.PodAffinityTerm.GetPodAffinityTerm(),
	}
}

func (c *NodeSelectorTerm) GetNodeSelectorTerm() api.NodeSelectorTerm {
	var nodeSelectorTerm = api.NodeSelectorTerm{}
	for _, expression := range c.MatchExpressions {
		nodeSelectorTerm.MatchExpressions = append(
			nodeSelectorTerm.MatchExpressions,
			expression.GetNodeSelectorRequirement(),
		)
	}
	for _, fields := range c.MatchFields {
		nodeSelectorTerm.MatchFields = append(
			nodeSelectorTerm.MatchFields,
			fields.GetNodeSelectorRequirement(),
		)
	}

	return nodeSelectorTerm
}

func (c *PreferredSchedulingTerm) GetPreferredSchedulingTerm() api.PreferredSchedulingTerm {
	return api.PreferredSchedulingTerm{
		Weight:     c.Weight,
		Preference: c.Preference.GetNodeSelectorTerm(),
	}
}

func (c *PodAffinityTerm) GetLabelSelector() *metav1.LabelSelector {
	if c.LabelSelector == nil {
		return nil
	}

	return &metav1.LabelSelector{
		MatchLabels:      c.LabelSelector.MatchLabels,
		MatchExpressions: c.LabelSelector.GetLabelSelectorMatchExpressions(),
	}
}

func (c *PodAffinityTerm) GetNamespaceSelector() *metav1.LabelSelector {
	if c.NamespaceSelector == nil {
		return nil
	}

	return &metav1.LabelSelector{
		MatchLabels:      c.NamespaceSelector.MatchLabels,
		MatchExpressions: c.NamespaceSelector.GetLabelSelectorMatchExpressions(),
	}
}

func (c *KubernetesConfig) GetHostAliases() []api.HostAlias {
	var hostAliases []api.HostAlias

	for _, hostAlias := range c.HostAliases {
		hostAliases = append(
			hostAliases,
			api.HostAlias{
				IP:        hostAlias.IP,
				Hostnames: hostAlias.Hostnames,
			},
		)
	}

	return hostAliases
}

func (c *DockerMachine) GetIdleCount() int {
	autoscaling := c.getActiveAutoscalingConfig()
	if autoscaling != nil {
		return autoscaling.IdleCount
	}

	return c.IdleCount
}

func (c *DockerMachine) GetIdleCountMin() int {
	autoscaling := c.getActiveAutoscalingConfig()
	if autoscaling != nil {
		return autoscaling.IdleCountMin
	}

	return c.IdleCountMin
}

func (c *DockerMachine) GetIdleScaleFactor() float64 {
	autoscaling := c.getActiveAutoscalingConfig()
	if autoscaling != nil {
		return autoscaling.IdleScaleFactor
	}

	return c.IdleScaleFactor
}

func (c *DockerMachine) GetIdleTime() int {
	autoscaling := c.getActiveAutoscalingConfig()
	if autoscaling != nil {
		return autoscaling.IdleTime
	}

	return c.IdleTime
}

// getActiveAutoscalingConfig returns the autoscaling config matching the current time.
// It goes through the [[docker.machine.autoscaling]] entries and returns the last one to match.
// Returns nil on no matching entries.
func (c *DockerMachine) getActiveAutoscalingConfig() *DockerMachineAutoscaling {
	var activeConf *DockerMachineAutoscaling
	for _, conf := range c.AutoscalingConfigs {
		if conf.compiledPeriods.InPeriod() {
			activeConf = conf
		}
	}

	return activeConf
}

func (c *DockerMachine) CompilePeriods() error {
	var err error

	for _, a := range c.AutoscalingConfigs {
		err = a.compilePeriods()
		if err != nil {
			return err
		}
	}

	return nil
}

var periodTimer = time.Now

func (a *DockerMachineAutoscaling) compilePeriods() error {
	periods, err := timeperiod.TimePeriodsWithTimer(a.Periods, a.Timezone, periodTimer)
	if err != nil {
		return NewInvalidTimePeriodsError(a.Periods, err)
	}

	a.compiledPeriods = periods

	return nil
}

func (c *DockerMachine) logDeprecationWarning() {
	if len(c.OffPeakPeriods) != 0 {
		logrus.Warning("OffPeak docker machine configuration is deprecated and has been removed since 14.0. " +
			"Please convert the setting into a [[docker.machine.autoscaling]] configuration instead: " +
			"https://docs.gitlab.com/runner/configuration/autoscale.html#off-peak-time-mode-configuration-deprecated")
	}
}

func (c *RunnerCredentials) GetURL() string {
	return c.URL
}

func (c *RunnerCredentials) GetTLSCAFile() string {
	return c.TLSCAFile
}

func (c *RunnerCredentials) GetTLSCertFile() string {
	return c.TLSCertFile
}

func (c *RunnerCredentials) GetTLSKeyFile() string {
	return c.TLSKeyFile
}

func (c *RunnerCredentials) GetToken() string {
	return c.Token
}

func (c *RunnerCredentials) ShortDescription() string {
	return helpers.ShortenToken(c.Token)
}

func (c *RunnerCredentials) UniqueID() string {
	return c.URL + c.Token
}

func (c *RunnerCredentials) Log() *logrus.Entry {
	if c.ShortDescription() != "" {
		return logrus.WithField("runner", c.ShortDescription())
	}
	return logrus.WithFields(logrus.Fields{})
}

func (c *RunnerCredentials) SameAs(other *RunnerCredentials) bool {
	return c.URL == other.URL && c.Token == other.Token
}

func (c *RunnerConfig) String() string {
	return fmt.Sprintf("%v url=%v token=%v executor=%v", c.Name, c.URL, c.Token, c.Executor)
}

func (c *RunnerConfig) GetUnhealthyRequestsLimit() int {
	if c.UnhealthyRequestsLimit < 1 {
		return DefaultUnhealthyRequestsLimit
	}

	return c.UnhealthyRequestsLimit
}

func (c *RunnerConfig) GetUnhealthyInterval() time.Duration {
	if c.UnhealthyInterval == nil {
		return DefaultUnhealthyInterval
	}

	return *c.UnhealthyInterval
}

func (c *RunnerConfig) GetRequestConcurrency() int {
	if c.RequestConcurrency <= 0 {
		return 1
	}
	return c.RequestConcurrency
}

func (c *RunnerConfig) GetVariables() JobVariables {
	variables := JobVariables{
		{Key: "CI_RUNNER_SHORT_TOKEN", Value: c.ShortDescription(), Public: true, Internal: true, File: false},
	}

	for _, environment := range c.Environment {
		if variable, err := ParseVariable(environment); err == nil {
			variable.Internal = true
			variables = append(variables, variable)
		}
	}

	return variables
}

// rewriteGetSourcesHooks Updates values of (pre|post)_get_sources_script setting with the value of
// (pre|post)_clone_script.
//
// Update happens only when the "source" (old) setting is non empty and the "target" (new) setting
// is empty
//
// DEPRECATED
// TODO: Remove in 16.0. For more details read https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29405
func (c *RunnerConfig) rewriteGetSourcesHooks() {
	log := logrus.WithFields(logrus.Fields{
		"runner":      c.ShortDescription(),
		"runner_name": c.Name,
	})

	if c.PreCloneScript != "" {
		log.Warning("pre_clone_script is deprecated; use pre_get_sources_script instead")

		if c.PreGetSourcesScript == "" {
			c.PreGetSourcesScript = c.PreCloneScript
			log.Warning("pre_get_sources_script is not defined; copying value of pre_clone_script")
		}
	}

	if c.PostCloneScript != "" {
		log.Warning("post_clone_script is deprecated; use post_get_sources_script instead")

		if c.PostGetSourcesScript == "" {
			c.PostGetSourcesScript = c.PostCloneScript
			log.Warning("post_get_sources_script is not defined; copying value of post_clone_script")
		}
	}
}

// DeepCopy attempts to make a deep clone of the object
func (c *RunnerConfig) DeepCopy() (*RunnerConfig, error) {
	var r RunnerConfig

	bytes, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("serialization of runner config failed: %w", err)
	}

	err = json.Unmarshal(bytes, &r)
	if err != nil {
		return nil, fmt.Errorf("deserialization of runner config failed: %w", err)
	}

	r.SystemIDState = c.SystemIDState

	return &r, err
}

func NewConfigWithSaver(s ConfigSaver) *Config {
	c := NewConfig()
	c.configSaver = s

	return c
}

func NewConfig() *Config {
	return &Config{
		Concurrent: 1,
		SessionServer: SessionServer{
			SessionTimeout: int(DefaultSessionTimeout.Seconds()),
		},
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
		runner.rewriteGetSourcesHooks()

		if runner.Machine == nil {
			continue
		}

		err := runner.Machine.CompilePeriods()
		if err != nil {
			return err
		}
		runner.Machine.logDeprecationWarning()
	}

	c.ModTime = info.ModTime()
	c.Loaded = true

	return nil
}

func (c *Config) SaveConfig(configFile string) error {
	var newConfig bytes.Buffer
	newBuffer := bufio.NewWriter(&newConfig)

	if err := toml.NewEncoder(newBuffer).Encode(c); err != nil {
		logrus.Fatalf("Error encoding TOML: %s", err)
		return err
	}

	if err := newBuffer.Flush(); err != nil {
		return err
	}

	if c.configSaver == nil {
		c.configSaver = new(defaultConfigSaver)
	}

	if err := c.configSaver.Save(configFile, newConfig.Bytes()); err != nil {
		return err
	}

	c.ModTime = time.Now()
	c.Loaded = true

	return nil
}

func (c *Config) GetCheckInterval() time.Duration {
	if c.CheckInterval > 0 {
		return time.Duration(c.CheckInterval) * time.Second
	}
	return CheckInterval
}

func (c *Config) GetShutdownTimeout() time.Duration {
	if c.ShutdownTimeout > 0 {
		return time.Duration(c.ShutdownTimeout) * time.Second
	}

	return DefaultShutdownTimeout
}

// GetPullPolicySource returns the source (i.e. file) of the pull_policy
// configuration used by this runner. This is used to produce a more detailed
// error message. See https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29115
func GetPullPolicySource(imagePullPolicies []DockerPullPolicy, pullPolicies StringOrArray) PullPolicySource {
	switch {
	case len(imagePullPolicies) != 0:
		return PullPolicySourceGitLabCI
	case len(pullPolicies) != 0:
		return PullPolicySourceRunner
	default:
		return PullPolicySourceDefault
	}
}

package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"sigs.k8s.io/yaml"

	"github.com/BurntSushi/toml"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-units"
	"github.com/minio/minio-go/v7/pkg/s3utils"
	"github.com/sirupsen/logrus"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"gitlab.com/gitlab-org/gitlab-runner/common/config/runner"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/timeperiod"
	"gitlab.com/gitlab-org/gitlab-runner/referees"
)

type (
	DockerPullPolicy = spec.PullPolicy
	DockerSysCtls    map[string]string
)

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

	UnknownSystemID = "unknown"

	DefaultConnectionMaxAge = 15 * time.Minute
)

const mask = "[MASKED]"

var (
	errPatchConversion = errors.New("converting patch to json")
	errPatchAmbiguous  = errors.New("ambiguous patch: both patch path and patch provided")
	errPatchFileFail   = errors.New("loading patch file")
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

func (c DockerConfig) IsUserAllowed(user string) bool {
	// default image user is allowed.
	if user == "" {
		return true
	}

	// if neither a user nor allowed-users have been specified in the runner config, any user is allowed.
	if len(c.AllowedUsers) == 0 && c.User == "" {
		return true
	}

	// if allowed-users was not configured, it defaults to the single user configured in the runner.
	allowedUsers := c.AllowedUsers
	if len(allowedUsers) == 0 {
		allowedUsers = []string{c.User}
	}

	return slices.Contains(allowedUsers, user)
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

type allowListKind string

const (
	allowListKindUser  allowListKind = "user"
	allowListKindGroup allowListKind = "group"
)

func (c KubernetesConfig) isUserOrGroupAllowed(v string, kind allowListKind, allowedList []string) error {
	// default image user is allowed.
	if v == "" {
		return nil
	}

	// Root requires explicit permission in allowlist, even if allowlist is empty
	if v == "0" && !slices.Contains(allowedList, "0") {
		return fmt.Errorf("%s %q is not in the allowed list: %v", kind, v, allowedList)
	}

	// if no allowed-users/groups have been specified in the runner config, any non-root user is allowed.
	if len(allowedList) == 0 {
		return nil
	}

	if !slices.Contains(allowedList, v) {
		return fmt.Errorf("%s %q is not in the allowed list: %v", kind, v, allowedList)
	}

	return nil
}

func (c KubernetesConfig) IsUserAllowed(user string) error {
	return c.isUserOrGroupAllowed(user, allowListKindUser, c.AllowedUsers)
}

func (c KubernetesConfig) IsGroupAllowed(group string) error {
	return c.isUserOrGroupAllowed(group, allowListKindGroup, c.AllowedGroups)
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

type DockerConfig struct {
	docker.Credentials
	Hostname                   string              `toml:"hostname,omitempty" json:"hostname" long:"hostname" env:"DOCKER_HOSTNAME" description:"Custom container hostname"`
	Image                      string              `toml:"image" json:"image" long:"image" env:"DOCKER_IMAGE" description:"Docker image to be used"`
	Runtime                    string              `toml:"runtime,omitempty" json:"runtime" long:"runtime" env:"DOCKER_RUNTIME" description:"Docker runtime to be used"`
	Memory                     string              `toml:"memory,omitempty" json:"memory" long:"memory" env:"DOCKER_MEMORY" description:"Memory limit (format: <number>[<unit>]). Unit can be one of b, k, m, or g. Minimum is 4M."`
	MemorySwap                 string              `toml:"memory_swap,omitempty" json:"memory_swap" long:"memory-swap" env:"DOCKER_MEMORY_SWAP" description:"Total memory limit (memory + swap, format: <number>[<unit>]). Unit can be one of b, k, m, or g."`
	MemoryReservation          string              `toml:"memory_reservation,omitempty" json:"memory_reservation" long:"memory-reservation" env:"DOCKER_MEMORY_RESERVATION" description:"Memory soft limit (format: <number>[<unit>]). Unit can be one of b, k, m, or g."`
	CgroupParent               string              `toml:"cgroup_parent,omitempty" json:"cgroup_parent" long:"cgroup-parent" env:"DOCKER_CGROUP_PARENT" description:"String value containing the cgroup parent to use"`
	CPUSetCPUs                 string              `toml:"cpuset_cpus,omitempty" json:"cpuset_cpus" long:"cpuset-cpus" env:"DOCKER_CPUSET_CPUS" description:"String value containing the cgroups CpusetCpus to use"`
	CPUSetMems                 string              `toml:"cpuset_mems,omitempty" json:"cpuset_mems" long:"cpuset-mems" env:"DOCKER_CPUSET_MEMS" description:"String value containing the cgroups CpusetMems to use"`
	CPUS                       string              `toml:"cpus,omitempty" json:"cpus" long:"cpus" env:"DOCKER_CPUS" description:"Number of CPUs"`
	CPUShares                  int64               `toml:"cpu_shares,omitzero" json:"cpu_shares" long:"cpu-shares" env:"DOCKER_CPU_SHARES" description:"Number of CPU shares"`
	DNS                        []string            `toml:"dns,omitempty" json:"dns,omitempty" long:"dns" env:"DOCKER_DNS" description:"A list of DNS servers for the container to use"`
	DNSSearch                  []string            `toml:"dns_search,omitempty" json:"dns_search,omitempty" long:"dns-search" env:"DOCKER_DNS_SEARCH" description:"A list of DNS search domains"`
	Privileged                 bool                `toml:"privileged,omitzero" json:"privileged" long:"privileged" env:"DOCKER_PRIVILEGED" description:"Give extended privileges to container"`
	ServicesPrivileged         *bool               `toml:"services_privileged,omitempty" json:"services_privileged,omitempty" long:"services_privileged" env:"DOCKER_SERVICES_PRIVILEGED" description:"When set this will give or remove extended privileges to container services"`
	DisableEntrypointOverwrite bool                `toml:"disable_entrypoint_overwrite,omitzero" json:"disable_entrypoint_overwrite" long:"disable-entrypoint-overwrite" env:"DOCKER_DISABLE_ENTRYPOINT_OVERWRITE" description:"Disable the possibility for a container to overwrite the default image entrypoint"`
	User                       string              `toml:"user,omitempty" json:"user" long:"user" env:"DOCKER_USER" description:"Run all commands in the container as the specified user."`
	AllowedUsers               []string            `toml:"allowed_users,omitempty" json:"allowed_users,omitempty" long:"allowed_users" env:"DOCKER_ALLOWED_USERS" description:"List of allowed users under which to run commands in the build container."`
	GroupAdd                   []string            `toml:"group_add" json:"group_add,omitempty" long:"group-add" env:"DOCKER_GROUP_ADD" description:"Add additional groups to join"`
	UsernsMode                 string              `toml:"userns_mode,omitempty" json:"userns_mode" long:"userns" env:"DOCKER_USERNS_MODE" description:"User namespace to use"`
	CapAdd                     []string            `toml:"cap_add" json:"cap_add,omitempty" long:"cap-add" env:"DOCKER_CAP_ADD" description:"Add Linux capabilities"`
	CapDrop                    []string            `toml:"cap_drop" json:"cap_drop,omitempty" long:"cap-drop" env:"DOCKER_CAP_DROP" description:"Drop Linux capabilities"`
	OomKillDisable             bool                `toml:"oom_kill_disable,omitzero" json:"oom_kill_disable" long:"oom-kill-disable" env:"DOCKER_OOM_KILL_DISABLE" description:"Do not kill processes in a container if an out-of-memory (OOM) error occurs"`
	OomScoreAdjust             int                 `toml:"oom_score_adjust,omitzero" json:"oom_score_adjust" long:"oom-score-adjust" env:"DOCKER_OOM_SCORE_ADJUST" description:"Adjust OOM score"`
	SecurityOpt                []string            `toml:"security_opt" json:"security_opt,omitempty" long:"security-opt" env:"DOCKER_SECURITY_OPT" description:"Security Options"`
	ServicesSecurityOpt        []string            `toml:"services_security_opt" json:"services_security_opt,omitempty" long:"services-security-opt" env:"DOCKER_SERVICES_SECURITY_OPT" description:"Security Options for container services"`
	Devices                    []string            `toml:"devices" json:"devices,omitempty" long:"devices" env:"DOCKER_DEVICES" description:"Add a host device to the container"`
	DeviceCgroupRules          []string            `toml:"device_cgroup_rules,omitempty" json:"device_cgroup_rules,omitempty" long:"device-cgroup-rules" env:"DOCKER_DEVICE_CGROUP_RULES" description:"Add a device cgroup rule to the container"`
	Gpus                       string              `toml:"gpus,omitempty" json:"gpus" long:"gpus" env:"DOCKER_GPUS" description:"Request GPUs to be used by Docker"`
	ServicesDevices            map[string][]string `toml:"services_devices,omitempty" json:"services_devices,omitempty" long:"services_devices" env:"DOCKER_SERVICES_DEVICES" description:"A toml table/json object with the format key=values. Expose host devices to services based on image name."`
	DisableCache               bool                `toml:"disable_cache,omitzero" json:"disable_cache" long:"disable-cache" env:"DOCKER_DISABLE_CACHE" description:"Disable all container caching"`
	Volumes                    []string            `toml:"volumes,omitempty" json:"volumes,omitempty" long:"volumes" env:"DOCKER_VOLUMES" description:"Bind-mount a volume and create it if it doesn't exist prior to mounting. Can be specified multiple times once per mountpoint, e.g. --docker-volumes 'test0:/test0' --docker-volumes 'test1:/test1'"`
	VolumeDriver               string              `toml:"volume_driver,omitempty" json:"volume_driver" long:"volume-driver" env:"DOCKER_VOLUME_DRIVER" description:"Volume driver to be used"`
	VolumeDriverOps            map[string]string   `toml:"volume_driver_ops,omitempty" json:"volume_driver_ops,omitempty" long:"volume-driver-ops" env:"DOCKER_VOLUME_DRIVER_OPS" description:"A toml table/json object with the format key=values. Volume driver ops to be specified"`
	CacheDir                   string              `toml:"cache_dir,omitempty" json:"cache_dir" long:"cache-dir" env:"DOCKER_CACHE_DIR" description:"Directory where to store caches"`
	ExtraHosts                 []string            `toml:"extra_hosts,omitempty" json:"extra_hosts,omitempty" long:"extra-hosts" env:"DOCKER_EXTRA_HOSTS" description:"Add a custom host-to-IP mapping"`
	VolumesFrom                []string            `toml:"volumes_from,omitempty" json:"volumes_from,omitempty" long:"volumes-from" env:"DOCKER_VOLUMES_FROM" description:"A list of volumes to inherit from another container"`
	NetworkMode                string              `toml:"network_mode,omitempty" json:"network_mode" long:"network-mode" env:"DOCKER_NETWORK_MODE" description:"Add container to a custom network"`
	IpcMode                    string              `toml:"ipcmode,omitempty" json:"ipcmode" long:"ipcmode" env:"DOCKER_IPC_MODE" description:"Select IPC mode for container"`
	MacAddress                 string              `toml:"mac_address,omitempty" json:"mac_address" long:"mac-address" env:"DOCKER_MAC_ADDRESS" description:"Container MAC address (e.g., 92:d0:c6:0a:29:33)"`
	Links                      []string            `toml:"links,omitempty" json:"links,omitempty" long:"links" env:"DOCKER_LINKS" description:"Add link to another container"`
	Services                   []Service           `toml:"services,omitempty" json:"services,omitempty" description:"Add service that is started with container"`
	ServicesLimit              *int                `toml:"services_limit,omitempty" json:"services_limit,omitempty" long:"services-limit" env:"DOCKER_SERVICES_LIMIT" description:"The maximum amount of services allowed"`
	ServiceMemory              string              `toml:"service_memory,omitempty" json:"service_memory" long:"service-memory" env:"DOCKER_SERVICE_MEMORY" description:"Service memory limit (format: <number>[<unit>]). Unit can be one of b (if omitted), k, m, or g. Minimum is 4M."`
	ServiceMemorySwap          string              `toml:"service_memory_swap,omitempty" json:"service_memory_swap" long:"service-memory-swap" env:"DOCKER_SERVICE_MEMORY_SWAP" description:"Service total memory limit (memory + swap, format: <number>[<unit>]). Unit can be one of b (if omitted), k, m, or g."`
	ServiceMemoryReservation   string              `toml:"service_memory_reservation,omitempty" json:"service_memory_reservation" long:"service-memory-reservation" env:"DOCKER_SERVICE_MEMORY_RESERVATION" description:"Service memory soft limit (format: <number>[<unit>]). Unit can be one of b (if omitted), k, m, or g."`
	ServiceCgroupParent        string              `toml:"service_cgroup_parent,omitempty" json:"service_cgroup_parent" long:"service-cgroup-parent" env:"DOCKER_SERVICE_CGROUP_PARENT" description:"String value containing the cgroup parent to use for service"`
	ServiceSlotCgroupTemplate  string              `toml:"service_slot_cgroup_template,omitempty" json:"service_slot_cgroup_template" long:"service-slot-cgroup-template" env:"DOCKER_SERVICE_SLOT_CGROUP_TEMPLATE" description:"Template for service slot-derived cgroup names (use ${slot} placeholder)"`
	ServiceCPUSetCPUs          string              `toml:"service_cpuset_cpus,omitempty" json:"service_cpuset_cpus" long:"service-cpuset-cpus" env:"DOCKER_SERVICE_CPUSET_CPUS" description:"String value containing the cgroups CpusetCpus to use for service"`
	ServiceCPUS                string              `toml:"service_cpus,omitempty" json:"service_cpus" long:"service-cpus" env:"DOCKER_SERVICE_CPUS" description:"Number of CPUs for service"`
	ServiceCPUShares           int64               `toml:"service_cpu_shares,omitzero" json:"service_cpu_shares" long:"service-cpu-shares" env:"DOCKER_SERVICE_CPU_SHARES" description:"Number of CPU shares for service"`
	ServiceGpus                string              `toml:"service_gpus,omitempty" json:"service_gpus" long:"service_gpus" env:"DOCKER_SERVICE_GPUS" description:"Request GPUs to be used by Docker for services"`
	WaitForServicesTimeout     int                 `toml:"wait_for_services_timeout,omitzero" json:"wait_for_services_timeout" long:"wait-for-services-timeout" env:"DOCKER_WAIT_FOR_SERVICES_TIMEOUT" description:"How long to wait for service startup"`
	AllowedImages              []string            `toml:"allowed_images,omitempty" json:"allowed_images,omitempty" long:"allowed-images" env:"DOCKER_ALLOWED_IMAGES" description:"Image allowlist"`
	AllowedPrivilegedImages    []string            `toml:"allowed_privileged_images,omitempty" json:"allowed_privileged_images,omitempty" long:"allowed-privileged-images" env:"DOCKER_ALLOWED_PRIVILEGED_IMAGES" description:"Privileged image allowlist"`
	AllowedPrivilegedServices  []string            `toml:"allowed_privileged_services,omitempty" json:"allowed_privileged_services,omitempty" long:"allowed-privileged-services" env:"DOCKER_ALLOWED_PRIVILEGED_SERVICES" description:"Privileged Service allowlist"`
	AllowedPullPolicies        []DockerPullPolicy  `toml:"allowed_pull_policies,omitempty" json:"allowed_pull_policies,omitempty" long:"allowed-pull-policies" env:"DOCKER_ALLOWED_PULL_POLICIES" description:"Pull policy allowlist"`
	AllowedServices            []string            `toml:"allowed_services,omitempty" json:"allowed_services,omitempty" long:"allowed-services" env:"DOCKER_ALLOWED_SERVICES" description:"Service allowlist"`
	PullPolicy                 StringOrArray       `toml:"pull_policy,omitempty" json:"pull_policy,omitempty" long:"pull-policy" env:"DOCKER_PULL_POLICY" description:"Image pull policy: never, if-not-present, always"`
	Isolation                  string              `toml:"isolation,omitempty" json:"isolation" long:"isolation" env:"DOCKER_ISOLATION" description:"Container isolation technology. Windows only"`
	ShmSize                    int64               `toml:"shm_size,omitempty" json:"shm_size" long:"shm-size" env:"DOCKER_SHM_SIZE" description:"Shared memory size for docker images (in bytes)"`
	Tmpfs                      map[string]string   `toml:"tmpfs,omitempty" json:"tmpfs,omitempty" long:"tmpfs" env:"DOCKER_TMPFS" description:"A toml table/json object with the format key=values. When set this will mount the specified path in the key as a tmpfs volume in the main container, using the options specified as key. For the supported options, see the documentation for the unix 'mount' command"`
	ServicesTmpfs              map[string]string   `toml:"services_tmpfs,omitempty" json:"services_tmpfs,omitempty" long:"services-tmpfs" env:"DOCKER_SERVICES_TMPFS" description:"A toml table/json object with the format key=values. When set this will mount the specified path in the key as a tmpfs volume in all the service containers, using the options specified as key. For the supported options, see the documentation for the unix 'mount' command"`
	SysCtls                    DockerSysCtls       `toml:"sysctls,omitempty" json:"sysctls,omitempty" long:"sysctls" env:"DOCKER_SYSCTLS" description:"Sysctl options, a toml table/json object of key=value. Value is expected to be a string."`
	HelperImage                string              `toml:"helper_image,omitempty" json:"helper_image" long:"helper-image" env:"DOCKER_HELPER_IMAGE" description:"[ADVANCED] Override the default helper image used to clone repos and upload artifacts"`
	HelperImageFlavor          string              `toml:"helper_image_flavor,omitempty" json:"helper_image_flavor" long:"helper-image-flavor" env:"DOCKER_HELPER_IMAGE_FLAVOR" description:"Set helper image flavor (alpine, ubuntu), defaults to alpine"`
	ContainerLabels            map[string]string   `toml:"container_labels,omitempty" json:"container_labels,omitempty" long:"container-labels" description:"A toml table/json object of key-value. Value is expected to be a string. When set, this will create containers with the given container labels. Environment variables will be substituted for values here."`
	EnableIPv6                 bool                `toml:"enable_ipv6,omitempty" json:"enable_ipv6" long:"enable-ipv6" description:"Enable IPv6 for automatically created networks. This is only takes affect when the feature flag FF_NETWORK_PER_BUILD is enabled."`
	Ulimit                     map[string]string   `toml:"ulimit,omitempty" json:"ulimit,omitempty" long:"ulimit" env:"DOCKER_ULIMIT" description:"Ulimit options for container"`
	NetworkMTU                 int                 `toml:"network_mtu,omitempty" json:"network_mtu" long:"network-mtu" description:"MTU of the Docker network created for the job IFF the FF_NETWORK_PER_BUILD feature-flag was specified."`
	LogOptions                 map[string]string   `toml:"log_options,omitempty" json:"log_options,omitempty" long:"log-options" env:"DOCKER_LOG_OPTIONS" description:"Log driver options for json-file logging"`
}

type InstanceConfig struct {
	AllowedImages     []string `toml:"allowed_images,omitempty" json:",omitempty" description:"When VM Isolation is enabled, allowed images controls which images a job is allowed to specify"`
	UseCommonBuildDir bool     `toml:"use_common_build_dir,omitempty" json:"use_common_build_dir,omitempty" description:"When use common build dir is enabled, all jobs will use the same build directory. This can only be enabled when VM isolation is enabled or a max use count is 1."`
}

type AutoscalerConfig struct {
	CapacityPerInstance int                      `toml:"capacity_per_instance,omitempty"`
	MaxUseCount         int                      `toml:"max_use_count,omitempty"`
	MaxInstances        int                      `toml:"max_instances,omitempty"`
	Plugin              string                   `toml:"plugin,omitempty"`
	PluginConfig        AutoscalerSettingsMap    `toml:"plugin_config,omitempty"`
	ConnectorConfig     ConnectorConfig          `toml:"connector_config,omitempty"`
	Policy              []AutoscalerPolicyConfig `toml:"policy,omitempty" json:",omitempty"`

	InstanceReadyCommand        string                  `toml:"instance_ready_command,omitempty" json:",omitempty"`
	InstanceAcquireTimeout      time.Duration           `toml:"instance_acquire_timeout,omitempty" json:",omitempty"`
	UpdateInterval              time.Duration           `toml:"update_interval,omitempty" json:",omitempty"`
	UpdateIntervalWhenExpecting time.Duration           `toml:"update_interval_when_expecting,omitempty" json:",omitempty"`
	DeletionRetryInterval       time.Duration           `toml:"deletion_retry_interval,omitempty" json:",omitempty"`
	ShutdownDeletionInterval    time.Duration           `toml:"shutdown_deletion_interval,omitempty" json:",omitempty"`
	ShutdownDeletionRetries     int                     `toml:"shutdown_deletion_retries,omitempty" json:",omitempty"`
	FailureThreshold            int                     `toml:"failure_threshold,omitempty" json:",omitempty"`
	ScaleThrottle               AutoscalerScaleThrottle `toml:"scale_throttle,omitempty" json:",omitempty"`
	ReservationThrottling       *bool                   `toml:"reservation_throttling,omitempty" json:",omitempty"`

	LogInternalIP bool `toml:"log_internal_ip,omitempty" json:",omitempty"`
	LogExternalIP bool `toml:"log_external_ip,omitempty" json:",omitempty"`

	DeleteInstancesOnShutdown bool `toml:"delete_instances_on_shutdown,omitempty" json:",omitempty"`

	VMIsolation VMIsolation `toml:"vm_isolation,omitempty"`

	StateStorage AutoscalerStateStorage `toml:"state_storage,omitempty" json:",omitempty"`

	// instance_operation_time_buckets was introduced some time ago, so we can't just delete it.
	// Someone can already depend on that setting.
	// Instead, it's now used as a way to define "default" buckets for the different operation
	// types, and more specific settings can be used to adjust what's needed to be adjusted.
	InstanceOperationTimeBuckets []float64 `toml:"instance_operation_time_buckets,omitempty" json:",omitempty"`

	InstanceCreationTimeBuckets  []float64 `toml:"instance_creation_time_buckets,omitempty" json:",omitempty"`
	InstanceIsRunningTimeBuckets []float64 `toml:"instance_is_running_time_buckets,omitempty" json:",omitempty"`
	InstanceDeletionTimeBuckets  []float64 `toml:"instance_deletion_time_buckets,omitempty" json:",omitempty"`
	InstanceReadinessTimeBuckets []float64 `toml:"instance_readiness_time_buckets,omitempty" json:",omitempty"`

	InstanceLifeDurationBuckets []float64 `toml:"instance_life_duration_buckets,omitempty" json:",omitempty"`
}

type AutoscalerStateStorage struct {
	Enabled bool   `toml:"enabled,omitempty" json:",omitempty"`
	Dir     string `toml:"dir,omitempty" json:",omitempty"`

	KeepInstanceWithAcquisitions bool `toml:"keep_instance_with_acquisitions,omitempty" json:",omitempty"`
}

type AutoscalerScaleThrottle struct {
	Limit int `toml:"limit,omitempty" json:",omitempty"`
	Burst int `toml:"burst,omitempty" json:",omitempty"`
}

func (c AutoscalerConfig) GetInstanceCreationTimeBuckets() []float64 {
	if len(c.InstanceCreationTimeBuckets) > 0 {
		return c.InstanceCreationTimeBuckets
	}
	return c.InstanceOperationTimeBuckets
}

func (c AutoscalerConfig) GetInstanceIsRunningTimeBuckets() []float64 {
	if len(c.InstanceIsRunningTimeBuckets) > 0 {
		return c.InstanceIsRunningTimeBuckets
	}
	return c.InstanceOperationTimeBuckets
}

func (c AutoscalerConfig) GetInstanceDeletionTimeBuckets() []float64 {
	if len(c.InstanceDeletionTimeBuckets) > 0 {
		return c.InstanceDeletionTimeBuckets
	}
	return c.InstanceOperationTimeBuckets
}

func (c AutoscalerConfig) GetInstanceReadinessTimeBuckets() []float64 {
	if len(c.InstanceReadinessTimeBuckets) > 0 {
		return c.InstanceReadinessTimeBuckets
	}
	return c.InstanceOperationTimeBuckets
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
	ProtocolPort         int           `toml:"protocol_port,omitempty"`
	Username             string        `toml:"username,omitempty"`
	Password             string        `toml:"password,omitempty"`
	KeyPathname          string        `toml:"key_path,omitempty"`
	UseStaticCredentials bool          `toml:"use_static_credentials,omitempty"`
	Keepalive            time.Duration `toml:"keepalive,omitempty"`
	Timeout              time.Duration `toml:"timeout,omitempty"`
	UseExternalAddr      bool          `toml:"use_external_addr,omitempty"`
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
	PreemptiveMode   *bool         `toml:"preemptive_mode,omitempty"`
}

func (policy *AutoscalerPolicyConfig) PreemptiveModeEnabled() bool {
	if policy.PreemptiveMode == nil {
		return policy.IdleCount > 0
	}
	return *policy.PreemptiveMode
}

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

	MachineOptionsWithName []string `long:"machine-options-with-name" json:",omitempty" env:"MACHINE_OPTIONS_WITH_NAME" description:"Template for additional options that may reference the machine name (need to include %s)"`

	OffPeakPeriods   []string `toml:"OffPeakPeriods,omitempty" json:",omitempty" description:"Time periods when the scheduler is in the OffPeak mode. DEPRECATED"`              // DEPRECATED
	OffPeakTimezone  string   `toml:"OffPeakTimezone,omitempty" description:"Timezone for the OffPeak periods (defaults to Local). DEPRECATED"`                                 // DEPRECATED
	OffPeakIdleCount int      `toml:"OffPeakIdleCount,omitzero" description:"Maximum idle machines when the scheduler is in the OffPeak mode. DEPRECATED"`                      // DEPRECATED
	OffPeakIdleTime  int      `toml:"OffPeakIdleTime,omitzero" description:"Minimum time after machine can be destroyed when the scheduler is in the OffPeak mode. DEPRECATED"` // DEPRECATED

	AutoscalingConfigs []*DockerMachineAutoscaling `toml:"autoscaling" json:",omitempty" description:"Ordered list of configurations for autoscaling periods (last match wins)"`
}

type DockerMachineAutoscaling struct {
	Periods         []string `long:"periods" json:",omitempty" description:"List of crontab expressions for this autoscaling configuration"`
	Timezone        string   `long:"timezone" description:"Timezone for the periods (defaults to Local)"`
	IdleCount       int      `long:"idle-count" description:"Maximum idle machines when this configuration is active"`
	IdleScaleFactor float64  `long:"idle-scale-factor" description:"(Experimental) Defines what factor of in-use machines should be used as current idle value, but never more then defined IdleCount. 0.0 means use IdleCount as a static number (defaults to 0.0). Must be defined as float number."`
	IdleCountMin    int      `long:"idle-count-min" description:"Minimal number of idle machines when IdleScaleFactor is in use. Defaults to 1."`
	IdleTime        int      `long:"idle-time" description:"Minimum time after which and idle machine can be destroyed when this configuration is active"`
	compiledPeriods *timeperiod.TimePeriod
}

type ParallelsConfig struct {
	BaseName         string   `toml:"base_name" json:"base_name" long:"base-name" env:"PARALLELS_BASE_NAME" description:"VM name to be used"`
	TemplateName     string   `toml:"template_name,omitempty" json:"template_name" long:"template-name" env:"PARALLELS_TEMPLATE_NAME" description:"VM template to be created"`
	DisableSnapshots bool     `toml:"disable_snapshots,omitzero" json:"disable_snapshots" long:"disable-snapshots" env:"PARALLELS_DISABLE_SNAPSHOTS" description:"Disable snapshoting to speedup VM creation"`
	TimeServer       string   `toml:"time_server,omitempty" json:"time_server" long:"time-server" env:"PARALLELS_TIME_SERVER" description:"Timeserver to sync the guests time from. Defaults to time.apple.com"`
	AllowedImages    []string `toml:"allowed_images,omitempty" json:"allowed_images,omitempty" long:"allowed-images" env:"PARALLELS_ALLOWED_IMAGES" description:"Image (base_name) allowlist"`
}

type VirtualBoxConfig struct {
	BaseName         string   `toml:"base_name" json:"base_name" long:"base-name" env:"VIRTUALBOX_BASE_NAME" description:"VM name to be used"`
	BaseSnapshot     string   `toml:"base_snapshot,omitempty" json:"base_snapshot" long:"base-snapshot" env:"VIRTUALBOX_BASE_SNAPSHOT" description:"Name or UUID of a specific VM snapshot to clone"`
	BaseFolder       string   `toml:"base_folder" json:"base_folder" long:"base-folder" env:"VIRTUALBOX_BASE_FOLDER" description:"Folder in which to save the new VM. If empty, uses VirtualBox default"`
	DisableSnapshots bool     `toml:"disable_snapshots,omitzero" json:"disable_snapshots" long:"disable-snapshots" env:"VIRTUALBOX_DISABLE_SNAPSHOTS" description:"Disable snapshoting to speedup VM creation"`
	AllowedImages    []string `toml:"allowed_images,omitempty" json:"allowed_images,omitempty" long:"allowed-images" env:"VIRTUALBOX_ALLOWED_IMAGES" description:"Image allowlist"`
	StartType        string   `toml:"start_type" json:"start_type" long:"start-type" env:"VIRTUALBOX_START_TYPE" description:"Graphical front-end type"`
}

type CustomConfig struct {
	ConfigExec        string   `toml:"config_exec,omitempty" json:"config_exec" long:"config-exec" env:"CUSTOM_CONFIG_EXEC" description:"Executable that allows to inject configuration values to the executor"`
	ConfigArgs        []string `toml:"config_args,omitempty" json:"config_args,omitempty" long:"config-args" description:"Arguments for the config executable"`
	ConfigExecTimeout *int     `toml:"config_exec_timeout,omitempty" json:"config_exec_timeout,omitempty" long:"config-exec-timeout" env:"CUSTOM_CONFIG_EXEC_TIMEOUT" description:"Timeout for the config executable (in seconds)"`

	PrepareExec        string   `toml:"prepare_exec,omitempty" json:"prepare_exec" long:"prepare-exec" env:"CUSTOM_PREPARE_EXEC" description:"Executable that prepares executor"`
	PrepareArgs        []string `toml:"prepare_args,omitempty" json:"prepare_args,omitempty" long:"prepare-args" description:"Arguments for the prepare executable"`
	PrepareExecTimeout *int     `toml:"prepare_exec_timeout,omitempty" json:"prepare_exec_timeout,omitempty" long:"prepare-exec-timeout" env:"CUSTOM_PREPARE_EXEC_TIMEOUT" description:"Timeout for the prepare executable (in seconds)"`

	RunExec string   `toml:"run_exec" json:"run_exec" long:"run-exec" env:"CUSTOM_RUN_EXEC" description:"Executable that runs the job script in executor"`
	RunArgs []string `toml:"run_args,omitempty" json:"run_args,omitempty" long:"run-args" description:"Arguments for the run executable"`

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

func (c *DockerConfig) GetUlimits() ([]*units.Ulimit, error) {
	ulimits := make([]*units.Ulimit, 0, len(c.Ulimit))

	for tp, limits := range c.Ulimit {
		ulimit := units.Ulimit{
			Name: tp,
		}

		before, after, ok := strings.Cut(limits, ":")

		var err error
		ulimit.Soft, err = strconv.ParseInt(before, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid soft limit value: %w", err)
		}

		ulimit.Hard = ulimit.Soft
		if ok {
			ulimit.Hard, err = strconv.ParseInt(after, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid soft limit value: %w", err)
			}
		}

		ulimits = append(ulimits, &ulimit)
	}
	return ulimits, nil
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

type KubernetesHostAliasesFlag []KubernetesHostAliases

func (h *KubernetesHostAliasesFlag) UnmarshalFlag(value string) error {
	return json.Unmarshal([]byte(value), h)
}

type KubernetesConfig struct {
	Host                                              string                             `toml:"host" json:"host" long:"host" env:"KUBERNETES_HOST" description:"Optional Kubernetes master host URL (auto-discovery attempted if not specified)"`
	Context                                           string                             `toml:"context,omitempty" json:"context" long:"context" env:"KUBECTL_CONTEXT" description:"Optional Kubernetes context name to use if host is not specified (kubectl config get-contexts)."`
	CertFile                                          string                             `toml:"cert_file,omitempty" json:"cert_file" long:"cert-file" env:"KUBERNETES_CERT_FILE" description:"Optional Kubernetes master auth certificate"`
	KeyFile                                           string                             `toml:"key_file,omitempty" json:"key_file" long:"key-file" env:"KUBERNETES_KEY_FILE" description:"Optional Kubernetes master auth private key"`
	CAFile                                            string                             `toml:"ca_file,omitempty" json:"ca_file" long:"ca-file" env:"KUBERNETES_CA_FILE" description:"Optional Kubernetes master auth ca certificate"`
	BearerTokenOverwriteAllowed                       bool                               `toml:"bearer_token_overwrite_allowed" json:"bearer_token_overwrite_allowed" long:"bearer_token_overwrite_allowed" env:"KUBERNETES_BEARER_TOKEN_OVERWRITE_ALLOWED" description:"Bool to authorize builds to specify their own bearer token for creation."`
	BearerToken                                       string                             `toml:"bearer_token,omitempty" json:"bearer_token" long:"bearer_token" env:"KUBERNETES_BEARER_TOKEN" description:"Optional Kubernetes service account token used to start build pods."`
	Image                                             string                             `toml:"image" json:"image" long:"image" env:"KUBERNETES_IMAGE" description:"Default docker image to use for builds when none is specified"`
	Namespace                                         string                             `toml:"namespace" json:"namespace" long:"namespace" env:"KUBERNETES_NAMESPACE" description:"Namespace to run Kubernetes jobs in"`
	NamespaceOverwriteAllowed                         string                             `toml:"namespace_overwrite_allowed" json:"namespace_overwrite_allowed" long:"namespace_overwrite_allowed" env:"KUBERNETES_NAMESPACE_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_NAMESPACE_OVERWRITE' value"`
	NamespacePerJob                                   bool                               `toml:"namespace_per_job" json:"namespace_per_job" long:"namespace_per_job" env:"KUBERNETES_NAMESPACE_PER_JOB" description:"Use separate namespace for each job. If set, 'KUBERNETES_NAMESPACE' and 'KUBERNETES_NAMESPACE_OVERWRITE_ALLOWED' are ignored."`
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
	AllowedUsers                                      []string                           `toml:"allowed_users,omitempty" json:"allowed_users,omitempty" long:"allowed-users" env:"KUBERNETES_ALLOWED_USERS" description:"User allowlist"`
	AllowedGroups                                     []string                           `toml:"allowed_groups,omitempty" json:"allowed_groups,omitempty" long:"allowed-groups" env:"KUBERNETES_ALLOWED_GROUPS" description:"Group allowlist"`
	PullPolicy                                        StringOrArray                      `toml:"pull_policy,omitempty" json:"pull_policy,omitempty" long:"pull-policy" env:"KUBERNETES_PULL_POLICY" description:"Policy for if/when to pull a container image (never, if-not-present, always). The cluster default will be used if not set"`
	NodeSelector                                      map[string]string                  `toml:"node_selector,omitempty" json:"node_selector,omitempty" long:"node-selector" env:"KUBERNETES_NODE_SELECTOR" description:"A toml table/json object of key:value. Value is expected to be a string. When set this will create pods on k8s nodes that match all the key:value pairs. Only one selector is supported through environment variable configuration."`
	NodeSelectorOverwriteAllowed                      string                             `toml:"node_selector_overwrite_allowed" json:"node_selector_overwrite_allowed" long:"node_selector_overwrite_allowed" env:"KUBERNETES_NODE_SELECTOR_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_NODE_SELECTOR_*' values"`
	NodeTolerations                                   map[string]string                  `toml:"node_tolerations,omitempty" json:"node_tolerations,omitempty" long:"node-tolerations" env:"KUBERNETES_NODE_TOLERATIONS" description:"A toml table/json object of key=value:effect. Value and effect are expected to be strings. When set, pods will tolerate the given taints. Only one toleration is supported through environment variable configuration."`
	NodeTolerationsOverwriteAllowed                   string                             `toml:"node_tolerations_overwrite_allowed" json:"node_tolerations_overwrite_allowed" long:"node_tolerations_overwrite_allowed" env:"KUBERNETES_NODE_TOLERATIONS_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_NODE_TOLERATIONS_*' values"`
	Affinity                                          KubernetesAffinity                 `toml:"affinity,omitempty" json:"affinity" long:"affinity" description:"Kubernetes Affinity setting that is used to select the node that spawns a pod"`
	ImagePullSecrets                                  []string                           `toml:"image_pull_secrets,omitempty" json:"image_pull_secrets,omitempty" long:"image-pull-secrets" env:"KUBERNETES_IMAGE_PULL_SECRETS" description:"A list of image pull secrets that are used for pulling docker image"`
	UseServiceAccountImagePullSecrets                 bool                               `toml:"use_service_account_image_pull_secrets,omitempty" json:"use_service_account_image_pull_secrets" long:"use-service-account-image-pull-secrets" env:"KUBERNETES_USE_SERVICE_ACCOUNT_IMAGE_PULL_SECRETS" description:"Do not provide any image pull secrets to the Pod created, so the secrets from the ServiceAccount can be used"`
	HelperImage                                       string                             `toml:"helper_image,omitempty" json:"helper_image" long:"helper-image" env:"KUBERNETES_HELPER_IMAGE" description:"[ADVANCED] Override the default helper image used to clone repos and upload artifacts"`
	HelperImageFlavor                                 string                             `toml:"helper_image_flavor,omitempty" json:"helper_image_flavor" long:"helper-image-flavor" env:"KUBERNETES_HELPER_IMAGE_FLAVOR" description:"Set helper image flavor (alpine, ubuntu), defaults to alpine"`
	HelperImageAutosetArchAndOS                       bool                               `toml:"helper_image_autoset_arch_and_os,omitempty" json:"helper_image_autoset_arch_and_os" long:"helper-image-autoset-arch-and-os" env:"KUBERNETES_HELPER_IMAGE_AUTOSET_ARCH_AND_OS" description:"When set, it uses the underlying OS to set the Helper Image ARCH and OS"`
	PodTerminationGracePeriodSeconds                  *int64                             `toml:"pod_termination_grace_period_seconds,omitzero" json:"pod_termination_grace_period_seconds,omitempty" long:"pod_termination_grace_period_seconds" env:"KUBERNETES_POD_TERMINATION_GRACE_PERIOD_SECONDS" description:"Pod-level setting which determines the duration in seconds which the pod has to terminate gracefully. After this, the processes are forcibly halted with a kill signal. Ignored if KUBERNETES_TERMINATIONGRACEPERIODSECONDS is specified."`
	CleanupGracePeriodSeconds                         *int64                             `toml:"cleanup_grace_period_seconds" json:"cleanup_grace_period_seconds,omitempty" long:"cleanup_grace_period_seconds" env:"KUBERNETES_CLEANUP_GRACE_PERIOD_SECONDS" description:"When cleaning up a pod on completion of a job, the duration in seconds which the pod has to terminate gracefully. After this, the processes are forcibly halted with a kill signal. Ignored if KUBERNETES_TERMINATIONGRACEPERIODSECONDS is specified."`
	CleanupResourcesTimeout                           *time.Duration                     `toml:"cleanup_resources_timeout,omitzero" json:"cleanup_resources_timeout,omitempty" long:"cleanup_resources_timeout" env:"KUBERNETES_CLEANUP_RESOURCES_TIMEOUT" description:"The total amount of time for Kubernetes resources to be cleaned up after the job completes. Supported syntax: '1h30m', '300s', '10m'. Default is 5 minutes ('5m')."`
	PollInterval                                      int                                `toml:"poll_interval,omitzero" json:"poll_interval" long:"poll-interval" env:"KUBERNETES_POLL_INTERVAL" description:"How frequently, in seconds, the runner will poll the Kubernetes pod it has just created to check its status"`
	PollTimeout                                       int                                `toml:"poll_timeout,omitzero" json:"poll_timeout" long:"poll-timeout" env:"KUBERNETES_POLL_TIMEOUT" description:"The total amount of time, in seconds, that needs to pass before the runner will timeout attempting to connect to the pod it has just created (useful for queueing more builds that the cluster can handle at a time)"`
	ResourceAvailabilityCheckMaxAttempts              int                                `toml:"resource_availability_check_max_attempts,omitzero" json:"resource_availability_check_max_attempts" long:"resource-availability-check-max-attempts" env:"KUBERNETES_RESOURCE_AVAILABILITY_CHECK_MAX_ATTEMPTS" default:"5" description:"The maximum number of attempts to check if a resource (service account and/or pull secret) set is available before giving up. There is 5 seconds interval between each attempt"`
	RequestRetryLimit                                 RequestRetryLimit                  `toml:"retry_limit,omitzero" json:"retry_limit" long:"retry-limit" env:"KUBERNETES_REQUEST_RETRY_LIMIT" default:"5" description:"The maximum number of attempts to communicate with Kubernetes API. The retry interval between each attempt is based on a backoff algorithm starting at 500 ms"`
	RequestRetryBackoffMax                            RequestRetryBackoffMax             `toml:"retry_backoff_max,omitzero" json:"retry_backoff_max" long:"retry-backoff-max" env:"KUBERNETES_REQUEST_RETRY_BACKOFF_MAX" default:"2000" description:"The max backoff interval value in milliseconds that can be reached for retry attempts to communicate with Kubernetes API"`
	RequestRetryLimits                                RequestRetryLimits                 `toml:"retry_limits" json:"retry_limits,omitempty" long:"retry-limits" env:"KUBERNETES_RETRY_LIMITS" description:"How many times each request error is to be retried"`
	PodLabels                                         map[string]string                  `toml:"pod_labels,omitempty" json:"pod_labels,omitempty" long:"pod-labels" description:"A toml table/json object of key-value. Value is expected to be a string. When set, this will create pods with the given pod labels. Environment variables will be substituted for values here."`
	PodLabelsOverwriteAllowed                         string                             `toml:"pod_labels_overwrite_allowed" json:"pod_labels_overwrite_allowed" long:"pod_labels_overwrite_allowed" env:"KUBERNETES_POD_LABELS_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_POD_LABELS_*' values"`
	SchedulerName                                     string                             `toml:"scheduler_name,omitempty" json:"scheduler_name" long:"scheduler-name" env:"KUBERNETES_SCHEDULER_NAME" description:"Pods will be scheduled using this scheduler, if it exists"`
	ServiceAccount                                    string                             `toml:"service_account,omitempty" json:"service_account" long:"service-account" env:"KUBERNETES_SERVICE_ACCOUNT" description:"Executor pods will use this Service Account to talk to kubernetes API"`
	ServiceAccountOverwriteAllowed                    string                             `toml:"service_account_overwrite_allowed" json:"service_account_overwrite_allowed" long:"service_account_overwrite_allowed" env:"KUBERNETES_SERVICE_ACCOUNT_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_SERVICE_ACCOUNT' value"`
	AutomountServiceAccountToken                      *bool                              `toml:"automount_service_account_token,omitzero" json:"automount_service_account_token,omitempty" long:"automount-service-account-token" env:"KUBERNETES_AUTOMOUNT_SERVICE_ACCOUNT_TOKEN" description:"Boolean to control the automount of the service account token in the build pod."`
	PodAnnotations                                    map[string]string                  `toml:"pod_annotations,omitempty" json:"pod_annotations,omitempty" long:"pod-annotations" description:"A toml table/json object of key-value. Value is expected to be a string. When set, this will create pods with the given annotations. Can be overwritten in build with KUBERNETES_POD_ANNOTATION_* variables"`
	PodAnnotationsOverwriteAllowed                    string                             `toml:"pod_annotations_overwrite_allowed" json:"pod_annotations_overwrite_allowed" long:"pod_annotations_overwrite_allowed" env:"KUBERNETES_POD_ANNOTATIONS_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_POD_ANNOTATIONS_*' values"`
	PodSecurityContext                                KubernetesPodSecurityContext       `toml:"pod_security_context,omitempty" namespace:"pod-security-context" description:"A security context attached to each build pod"`
	InitPermissionsContainerSecurityContext           KubernetesContainerSecurityContext `toml:"init_permissions_container_security_context,omitempty" namespace:"init_permissions_container_security_context" description:"A security context attached to the init-permissions container inside the build pod"`
	BuildContainerSecurityContext                     KubernetesContainerSecurityContext `toml:"build_container_security_context,omitempty" namespace:"build_container_security_context" description:"A security context attached to the build container inside the build pod"`
	HelperContainerSecurityContext                    KubernetesContainerSecurityContext `toml:"helper_container_security_context,omitempty" namespace:"helper_container_security_context" description:"A security context attached to the helper container inside the build pod"`
	ServiceContainerSecurityContext                   KubernetesContainerSecurityContext `toml:"service_container_security_context,omitempty" namespace:"service_container_security_context" description:"A security context attached to the service containers inside the build pod"`
	Volumes                                           KubernetesVolumes                  `toml:"volumes"`
	HostAliases                                       KubernetesHostAliasesFlag          `toml:"host_aliases,omitempty" json:"host_aliases,omitempty" long:"host_aliases" description:"Add a custom host-to-IP mapping"`
	Services                                          []Service                          `toml:"services,omitempty" json:"services,omitempty" description:"Add service that is started with container"`
	CapAdd                                            []string                           `toml:"cap_add" json:"cap_add,omitempty" long:"cap-add" env:"KUBERNETES_CAP_ADD" description:"Add Linux capabilities"`
	CapDrop                                           []string                           `toml:"cap_drop" json:"cap_drop,omitempty" long:"cap-drop" env:"KUBERNETES_CAP_DROP" description:"Drop Linux capabilities"`
	DNSPolicy                                         KubernetesDNSPolicy                `toml:"dns_policy,omitempty" json:"dns_policy" long:"dns-policy" env:"KUBERNETES_DNS_POLICY" description:"How Kubernetes should try to resolve DNS from the created pods. If unset, Kubernetes will use the default 'ClusterFirst'. Valid values are: none, default, cluster-first, cluster-first-with-host-net"`
	DNSConfig                                         KubernetesDNSConfig                `toml:"dns_config" json:"dns_config" description:"Pod DNS config"`
	ContainerLifecycle                                KubernetesContainerLifecyle        `toml:"container_lifecycle,omitempty" json:"container_lifecycle,omitempty" description:"Actions that the management system should take in response to container lifecycle events"`
	PriorityClassName                                 string                             `toml:"priority_class_name,omitempty" json:"priority_class_name" long:"priority_class_name" env:"KUBERNETES_PRIORITY_CLASS_NAME" description:"If set, the Kubernetes Priority Class to be set to the Pods"`
	PodSpec                                           []KubernetesPodSpec                `toml:"pod_spec" json:",omitempty"`
	LogsBaseDir                                       string                             `toml:"logs_base_dir,omitempty" json:"logs_base_dir" long:"logs-base-dir" env:"KUBERNETES_LOGS_BASE_DIR" description:"Base directory for the path where build logs are stored. This directory is prepended to the final generated path. For example, <logs_base_dir>/logs-<project_id>-<job_id>."`
	ScriptsBaseDir                                    string                             `toml:"scripts_base_dir,omitempty" json:"scripts_base_dir" long:"scripts-base-dir" env:"KUBERNETES_SCRIPTS_BASE_DIR" description:"Base directory for the path where build scripts are stored. This directory is prepended to the final generated path. For example, <scripts_base_dir>/scripts-<project_id>-<job_id>."`
	PrintPodWarningEvents                             *bool                              `toml:"print_pod_warning_events,omitempty" json:"print_pod_warning_events,omitempty" long:"print-pod-warning-events" env:"KUBERNETES_PRINT_POD_WARNING_EVENTS" description:"When enabled, all warning events associated with the pod are retrieved when the job fails. Enabled by default."`
}

type RequestRetryLimit int

func (r RequestRetryLimit) Get() int {
	if r > 0 {
		return int(r)
	}

	return DefaultRequestRetryLimit
}

type RequestRetryLimits map[string]int

type RequestRetryBackoffMax int

func (r RequestRetryBackoffMax) Get() time.Duration {
	switch {
	case r <= 0:
		return DefaultRequestRetryBackoffMax
	case time.Duration(r)*time.Millisecond <= RequestRetryBackoffMin:
		return RequestRetryBackoffMin
	default:
		return time.Duration(r) * time.Millisecond
	}
}

type KubernetesPodSpec struct {
	Name      string                     `toml:"name"`
	PatchPath string                     `toml:"patch_path"`
	Patch     string                     `toml:"patch"`
	PatchType KubernetesPodSpecPatchType `toml:"patch_type"`
}

// PodSpecPatch returns the patch data (JSON encoded) and type
func (s *KubernetesPodSpec) PodSpecPatch() ([]byte, KubernetesPodSpecPatchType, error) {
	patchBytes := []byte(s.Patch)
	patchType := s.PatchType
	if patchType == "" {
		patchType = PatchTypeStrategicMergePatchType
	}

	if s.PatchPath != "" {
		if s.Patch != "" {
			return nil, "", fmt.Errorf("%w (%s)", errPatchAmbiguous, s.Name)
		}

		var err error
		patchBytes, err = os.ReadFile(s.PatchPath)
		if err != nil {
			return nil, "", fmt.Errorf("%w (%s): %w", errPatchFileFail, s.Name, err)
		}
	}

	patchBytes, err := yaml.YAMLToJSON(patchBytes)
	if err != nil {
		return nil, "", fmt.Errorf("%w (%s): %w", errPatchConversion, s.Name, err)
	}

	return patchBytes, patchType, nil
}

type KubernetesPodSpecPatchType string

const (
	PatchTypeJSONPatchType           = KubernetesPodSpecPatchType("json")
	PatchTypeMergePatchType          = KubernetesPodSpecPatchType("merge")
	PatchTypeStrategicMergePatchType = KubernetesPodSpecPatchType("strategic")
)

type KubernetesDNSConfig struct {
	Nameservers []string                    `toml:"nameservers" json:",omitempty" description:"A list of IP addresses that will be used as DNS servers for the Pod."`
	Options     []KubernetesDNSConfigOption `toml:"options" json:",omitempty" description:"An optional list of objects where each object may have a name property (required) and a value property (optional)."`
	Searches    []string                    `toml:"searches" json:",omitempty" description:"A list of DNS search domains for hostname lookup in the Pod."`
}

type KubernetesDNSConfigOption struct {
	Name  string  `toml:"name"`
	Value *string `toml:"value,omitempty"`
}

type KubernetesVolumes struct {
	HostPaths  []KubernetesHostPath  `toml:"host_path" json:",omitempty" description:"The host paths which will be mounted"`
	PVCs       []KubernetesPVC       `toml:"pvc" json:",omitempty" description:"The persistent volume claims that will be mounted"`
	ConfigMaps []KubernetesConfigMap `toml:"config_map" json:",omitempty" description:"The config maps which will be mounted as volumes"`
	Secrets    []KubernetesSecret    `toml:"secret" json:",omitempty" description:"The secret maps which will be mounted"`
	EmptyDirs  []KubernetesEmptyDir  `toml:"empty_dir" json:",omitempty" description:"The empty dirs which will be mounted"`
	CSIs       []KubernetesCSI       `toml:"csi" json:",omitempty" description:"The CSI volumes which will be mounted"`
}

type KubernetesConfigMap struct {
	Name      string            `toml:"name" json:"name" description:"The name of the volume and ConfigMap to use"`
	MountPath string            `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath   string            `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	ReadOnly  bool              `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	Items     map[string]string `toml:"items,omitempty" json:",omitempty" description:"Key-to-path mapping for keys from the config map that should be used."`
}

type KubernetesHostPath struct {
	Name             string  `toml:"name" json:"name" description:"The name of the volume"`
	MountPath        string  `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath          string  `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	ReadOnly         bool    `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	HostPath         string  `toml:"host_path,omitempty" description:"Path from the host that should be mounted as a volume"`
	MountPropagation *string `toml:"mount_propagation,omitempty" description:"Mount propagation mode for the volume"`
}

type KubernetesPVC struct {
	Name             string  `toml:"name" json:"name" description:"The name of the volume and PVC to use"`
	MountPath        string  `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath          string  `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	ReadOnly         bool    `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	MountPropagation *string `toml:"mount_propagation,omitempty" description:"Mount propagation mode for the volume"`
}

type KubernetesSecret struct {
	Name      string            `toml:"name" json:"name" description:"The name of the volume and Secret to use"`
	MountPath string            `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath   string            `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	ReadOnly  bool              `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	Items     map[string]string `toml:"items,omitempty" json:",omitempty" description:"Key-to-path mapping for keys from the secret that should be used."`
}

type KubernetesEmptyDir struct {
	Name      string `toml:"name" json:"name" description:"The name of the volume and EmptyDir to use"`
	MountPath string `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath   string `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	Medium    string `toml:"medium,omitempty" description:"Set to 'Memory' to have a tmpfs"`
	SizeLimit string `toml:"size_limit,omitempty" description:"Total amount of local storage required."`
}

type KubernetesCSI struct {
	Name             string            `toml:"name" json:"name" description:"The name of the CSI volume and volumeMount to use"`
	MountPath        string            `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	SubPath          string            `toml:"sub_path,omitempty" description:"The sub-path of the volume to mount (defaults to volume root)"`
	Driver           string            `toml:"driver" description:"A string value that specifies the name of the volume driver to use."`
	FSType           string            `toml:"fs_type" description:"Filesystem type to mount. If not provided, the empty value is passed to the associated CSI driver which will determine the default filesystem to apply."`
	ReadOnly         bool              `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	VolumeAttributes map[string]string `toml:"volume_attributes,omitempty" json:",omitempty" description:"Key-value pair mapping for attributes of the CSI volume."`
}

type KubernetesPodSecurityContext struct {
	FSGroup            *int64  `toml:"fs_group,omitempty" json:",omitempty" long:"fs-group" env:"KUBERNETES_POD_SECURITY_CONTEXT_FS_GROUP" description:"A special supplemental group that applies to all containers in a pod"`
	RunAsGroup         *int64  `toml:"run_as_group,omitempty" json:",omitempty" long:"run-as-group" env:"KUBERNETES_POD_SECURITY_CONTEXT_RUN_AS_GROUP" description:"The GID to run the entrypoint of the container process"`
	RunAsNonRoot       *bool   `toml:"run_as_non_root,omitempty" json:",omitempty" long:"run-as-non-root" env:"KUBERNETES_POD_SECURITY_CONTEXT_RUN_AS_NON_ROOT" description:"Indicates that the container must run as a non-root user"`
	RunAsUser          *int64  `toml:"run_as_user,omitempty" json:",omitempty" long:"run-as-user" env:"KUBERNETES_POD_SECURITY_CONTEXT_RUN_AS_USER" description:"The UID to run the entrypoint of the container process"`
	SupplementalGroups []int64 `toml:"supplemental_groups,omitempty" json:",omitempty" long:"supplemental-groups" description:"A list of groups applied to the first process run in each container, in addition to the container's primary GID"`
	SELinuxType        string  `toml:"selinux_type,omitempty" long:"selinux-type" description:"The SELinux type label that applies to all containers in a pod"`
}

type KubernetesContainerCapabilities struct {
	Add  []api.Capability `toml:"add" json:",omitempty" long:"add" env:"@ADD" description:"List of capabilities to add to the build container"`
	Drop []api.Capability `toml:"drop" json:",omitempty" long:"drop" env:"@DROP" description:"List of capabilities to drop from the build container"`
}

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

type KubernetesAffinity struct {
	NodeAffinity    *KubernetesNodeAffinity    `toml:"node_affinity,omitempty" json:"node_affinity,omitempty" long:"node-affinity" description:"Node affinity is conceptually similar to nodeSelector -- it allows you to constrain which nodes your pod is eligible to be scheduled on, based on labels on the node."`
	PodAffinity     *KubernetesPodAffinity     `toml:"pod_affinity,omitempty" json:"pod_affinity,omitempty" description:"Pod affinity allows to constrain which nodes your pod is eligible to be scheduled on based on the labels on other pods."`
	PodAntiAffinity *KubernetesPodAntiAffinity `toml:"pod_anti_affinity,omitempty" json:"pod_anti_affinity,omitempty" description:"Pod anti-affinity allows to constrain which nodes your pod is eligible to be scheduled on based on the labels on other pods."`
}

type KubernetesNodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector             `toml:"required_during_scheduling_ignored_during_execution,omitempty" json:"required_during_scheduling_ignored_during_execution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `toml:"preferred_during_scheduling_ignored_during_execution,omitempty" json:"preferred_during_scheduling_ignored_during_execution,omitempty"`
}

type KubernetesPodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `toml:"required_during_scheduling_ignored_during_execution,omitempty" json:"required_during_scheduling_ignored_during_execution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `toml:"preferred_during_scheduling_ignored_during_execution,omitempty" json:"preferred_during_scheduling_ignored_during_execution,omitempty"`
}

type KubernetesPodAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `toml:"required_during_scheduling_ignored_during_execution,omitempty" json:"required_during_scheduling_ignored_during_execution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `toml:"preferred_during_scheduling_ignored_during_execution,omitempty" json:"preferred_during_scheduling_ignored_during_execution,omitempty"`
}

type KubernetesHostAliases struct {
	IP        string   `toml:"ip" json:"ip" long:"ip" description:"The IP address you want to attach hosts to"`
	Hostnames []string `toml:"hostnames" json:"hostnames,omitempty" long:"hostnames" description:"A list of hostnames that will be attached to the IP"`
}

// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#lifecycle-v1-core
type KubernetesContainerLifecyle struct {
	PostStart *KubernetesLifecycleHandler `toml:"post_start,omitempty" json:"post_start,omitempty" description:"PostStart is called immediately after a container is created. If the handler fails, the container is terminated and restarted according to its restart policy. Other management of the container blocks until the hook completes"`
	PreStop   *KubernetesLifecycleHandler `toml:"pre_stop,omitempty" json:"pre_stop,omitempty" description:"PreStop is called immediately before a container is terminated due to an API request or management event such as liveness/startup probe failure, preemption, resource contention, etc. The handler is not called if the container crashes or exits. The reason for termination is passed to the handler. The Pod's termination grace period countdown begins before the PreStop hooked is executed. Regardless of the outcome of the handler, the container will eventually terminate within the Pod's termination grace period. Other management of the container blocks until the hook completes or until the termination grace period is reached"`
}

type KubernetesLifecycleHandler struct {
	Exec      *KubernetesLifecycleExecAction `toml:"exec"  json:"exec,omitempty" description:"Exec specifies the action to take"`
	HTTPGet   *KubernetesLifecycleHTTPGet    `toml:"http_get"  json:"http_get,omitempty" description:"HTTPGet specifies the http request to perform."`
	TCPSocket *KubernetesLifecycleTCPSocket  `toml:"tcp_socket"  json:"tcp_socket,omitempty" description:"TCPSocket specifies an action involving a TCP port"`
}

type KubernetesLifecycleExecAction struct {
	Command []string `toml:"command" json:"command,omitempty" description:"Command is the command line to execute inside the container, the working directory for the command  is root ('/') in the container's filesystem. The command is simply exec'd, it is not run inside a shell, so traditional shell instructions ('|', etc) won't work. To use a shell, you need to explicitly call out to that shell. Exit status of 0 is treated as live/healthy and non-zero is unhealthy"`
}

type KubernetesLifecycleHTTPGet struct {
	Host        string                             `toml:"host" json:"host" description:"Host name to connect to, defaults to the pod IP. You probably want to set \"Host\" in httpHeaders instead"`
	HTTPHeaders []KubernetesLifecycleHTTPGetHeader `toml:"http_headers" json:"http_headers,omitempty" description:"Custom headers to set in the request. HTTP allows repeated headers"`
	Path        string                             `toml:"path" json:"path" description:"Path to access on the HTTP server"`
	Port        int                                `toml:"port" json:"port" description:"Number of the port to access on the container. Number must be in the range 1 to 65535"`
	Scheme      string                             `toml:"scheme" json:"scheme" description:"Scheme to use for connecting to the host. Defaults to HTTP"`
}

type KubernetesLifecycleHTTPGetHeader struct {
	Name  string `toml:"name" json:"name" description:"The header field name"`
	Value string `toml:"value" json:"value" description:"The header field value"`
}

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
			Port:        intstr.FromInt32(int32(h.HTTPGet.Port)),
			Path:        h.HTTPGet.Path,
			Scheme:      api.URIScheme(h.HTTPGet.Scheme),
			HTTPHeaders: httpHeaders,
		}
	}
	if h.TCPSocket != nil {
		kubeHandler.TCPSocket = &api.TCPSocketAction{
			Host: h.TCPSocket.Host,
			Port: intstr.FromInt32(int32(h.TCPSocket.Port)),
		}
	}

	return kubeHandler
}

type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `toml:"node_selector_terms" json:"node_selector_terms,omitempty"`
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
	MatchExpressions []NodeSelectorRequirement `toml:"match_expressions,omitempty" json:"match_expressions,omitempty"`
	MatchFields      []NodeSelectorRequirement `toml:"match_fields,omitempty" json:"match_fields,omitempty"`
}

type NodeSelectorRequirement struct {
	Key      string   `toml:"key,omitempty" json:"key"`
	Operator string   `toml:"operator,omitempty" json:"operator"`
	Values   []string `toml:"values,omitempty" json:"values,omitempty"`
}

type PodAffinityTerm struct {
	LabelSelector     *LabelSelector `toml:"label_selector,omitempty" json:"label_selector,omitempty"`
	Namespaces        []string       `toml:"namespaces,omitempty" json:"namespaces,omitempty"`
	TopologyKey       string         `toml:"topology_key,omitempty" json:"topology_key"`
	NamespaceSelector *LabelSelector `toml:"namespace_selector,omitempty" json:"namespace_selector,omitempty"`
}

type LabelSelector struct {
	MatchLabels      map[string]string         `toml:"match_labels,omitempty" json:"match_labels,omitempty"`
	MatchExpressions []NodeSelectorRequirement `toml:"match_expressions,omitempty" json:"match_expressions,omitempty"`
}

type Service struct {
	Name        string   `toml:"name" long:"name" description:"The image path for the service"`
	Alias       string   `toml:"alias,omitempty" long:"alias" description:"Space or comma-separated aliases of the service."`
	Command     []string `toml:"command" json:",omitempty" long:"command" description:"Command or script that should be used as the container’s command. Syntax is similar to https://docs.docker.com/engine/reference/builder/#cmd"`
	Entrypoint  []string `toml:"entrypoint" json:",omitempty" long:"entrypoint" description:"Command or script that should be executed as the container’s entrypoint. syntax is similar to https://docs.docker.com/engine/reference/builder/#entrypoint"`
	Environment []string `toml:"environment,omitempty" json:"environment,omitempty" long:"env" description:"Custom environment variables injected to service environment"`
}

func (s *Service) Aliases() []string { return strings.Fields(strings.ReplaceAll(s.Alias, ",", " ")) }

func (s *Service) ToImageDefinition() spec.Image {
	image := spec.Image{
		Name:       s.Name,
		Alias:      s.Alias,
		Command:    s.Command,
		Entrypoint: s.Entrypoint,
	}

	for _, environment := range s.Environment {
		if variable, err := parseVariable(environment); err == nil {
			variable.Internal = true
			image.Variables = append(image.Variables, variable)
		}
	}

	return image
}

type RunnerCredentials struct {
	URL             string    `toml:"url" json:"url" short:"u" long:"url" env:"CI_SERVER_URL" required:"true" description:"GitLab instance URL" jsonschema:"minLength=1"`
	ID              int64     `toml:"id" json:"id" description:"Runner ID"`
	Token           string    `toml:"token" json:"token" short:"t" long:"token" env:"CI_SERVER_TOKEN" required:"true" description:"Runner token" jsonschema:"minLength=1"`
	TokenObtainedAt time.Time `toml:"token_obtained_at" json:"token_obtained_at" description:"When the runner authentication token was obtained"`
	TokenExpiresAt  time.Time `toml:"token_expires_at" json:"token_expires_at" description:"Runner token expiration time"`
	TLSCAFile       string    `toml:"tls-ca-file,omitempty" json:"tls-ca-file" long:"tls-ca-file" env:"CI_SERVER_TLS_CA_FILE" description:"File containing the certificates to verify the peer when using HTTPS"`
	TLSCertFile     string    `toml:"tls-cert-file,omitempty" json:"tls-cert-file" long:"tls-cert-file" env:"CI_SERVER_TLS_CERT_FILE" description:"File containing certificate for TLS client auth when using HTTPS"`
	TLSKeyFile      string    `toml:"tls-key-file,omitempty" json:"tls-key-file" long:"tls-key-file" env:"CI_SERVER_TLS_KEY_FILE" description:"File containing private key for TLS client auth when using HTTPS"`

	Logger logrus.FieldLogger `toml:"-" json:",omitempty"`
}

type CacheGCSCredentials struct {
	AccessID   string `toml:"AccessID,omitempty" long:"access-id" env:"CACHE_GCS_ACCESS_ID" description:"ID of GCP Service Account used to access the storage"`
	PrivateKey string `toml:"PrivateKey,omitempty" long:"private-key" env:"CACHE_GCS_PRIVATE_KEY" description:"Private key used to sign GCS requests"`
}

type CacheGCSConfig struct {
	CacheGCSCredentials
	CredentialsFile string `toml:"CredentialsFile,omitempty" long:"credentials-file" env:"GOOGLE_APPLICATION_CREDENTIALS" description:"File with GCP credentials, containing AccessID and PrivateKey"`
	BucketName      string `toml:"BucketName,omitempty" long:"bucket-name" env:"CACHE_GCS_BUCKET_NAME" description:"Name of the bucket where cache will be stored"`
	UniverseDomain  string `toml:"UniverseDomain,omitempty" long:"universe-domain" env:"CACHE_GCS_UNIVERSE_DOMAIN" description:"Universe Domain for GCS requests (e.g., googleapis.com for public cloud, or a custom universe domain)"`
}

type CacheS3Config struct {
	ServerAddress             string     `toml:"ServerAddress,omitempty" long:"server-address" env:"CACHE_S3_SERVER_ADDRESS" description:"A host:port to the used S3-compatible server"`
	AccessKey                 string     `toml:"AccessKey,omitempty" long:"access-key" env:"CACHE_S3_ACCESS_KEY" description:"S3 Access Key"`
	SecretKey                 string     `toml:"SecretKey,omitempty" long:"secret-key" env:"CACHE_S3_SECRET_KEY" description:"S3 Secret Key"`
	SessionToken              string     `toml:"SessionToken,omitempty" long:"session-token" env:"CACHE_S3_SESSION_TOKEN" description:"S3 Session Token"`
	BucketName                string     `toml:"BucketName,omitempty" long:"bucket-name" env:"CACHE_S3_BUCKET_NAME" description:"Name of the bucket where cache will be stored"`
	BucketLocation            string     `toml:"BucketLocation,omitempty" long:"bucket-location" env:"CACHE_S3_BUCKET_LOCATION" description:"Name of S3 region"`
	Insecure                  bool       `toml:"Insecure,omitempty" long:"insecure" env:"CACHE_S3_INSECURE" description:"Use insecure mode (without https)"`
	AuthenticationType        S3AuthType `toml:"AuthenticationType,omitempty" long:"authentication_type" env:"CACHE_S3_AUTHENTICATION_TYPE" description:"IAM or credentials"`
	ServerSideEncryption      string     `toml:"ServerSideEncryption,omitempty" long:"server-side-encryption" env:"CACHE_S3_SERVER_SIDE_ENCRYPTION" description:"Server side encryption type (S3, or KMS)"`
	ServerSideEncryptionKeyID string     `toml:"ServerSideEncryptionKeyID,omitempty" long:"server-side-encryption-key-id" env:"CACHE_S3_SERVER_SIDE_ENCRYPTION_KEY_ID" description:"Server side encryption key ID (alias or Key ID or Key ARN)"`
	DualStack                 *bool      `toml:"DualStack,omitempty" long:"dual-stack" env:"CACHE_S3_DUAL_STACK" description:"Enable dual-stack (IPv4 and IPv6) endpoints (default: true)" jsonschema:"oneof_type=boolean;null"`
	PathStyle                 *bool      `toml:"PathStyle,omitempty" long:"path-style" env:"CACHE_S3_PATH_STYLE" description:"Use path style access (default: false)" jsonschema:"oneof_type=boolean;null"`
	Accelerate                bool       `toml:"Accelerate,omitempty" long:"accelerate" env:"CACHE_S3_ACCELERATE" description:"Enable S3 Transfer Acceleration"`
	RoleARN                   string     `toml:"RoleARN,omitempty" long:"role-arn" env:"CACHE_S3_ROLE_ARN" description:"Role ARN for transferring cache to S3"`
	UploadRoleARN             string     `toml:"UploadRoleARN,omitempty" long:"upload-role-arn" env:"CACHE_S3_UPLOAD_ROLE_ARN" description:"Role ARN for uploading cache to S3"`
}

type CacheAzureCredentials struct {
	AccountName string `toml:"AccountName,omitempty" long:"account-name" env:"CACHE_AZURE_ACCOUNT_NAME" description:"Account name for Azure Blob Storage"`
	AccountKey  string `toml:"AccountKey,omitempty" long:"account-key" env:"CACHE_AZURE_ACCOUNT_KEY" description:"Access key for Azure Blob Storage"`
}

type CacheAzureConfig struct {
	CacheAzureCredentials
	ContainerName string `toml:"ContainerName,omitempty" long:"container-name" env:"CACHE_AZURE_CONTAINER_NAME" description:"Name of the Azure container where cache will be stored"`
	StorageDomain string `toml:"StorageDomain,omitempty" long:"storage-domain" env:"CACHE_AZURE_STORAGE_DOMAIN" description:"Domain name of the Azure storage (e.g. blob.core.windows.net)"`
}

type CacheConfig struct {
	Type                   string `toml:"Type,omitempty" long:"type" env:"CACHE_TYPE" description:"Select caching method"`
	Path                   string `toml:"Path,omitempty" long:"path" env:"CACHE_PATH" description:"Name of the path to prepend to the cache URL"`
	Shared                 bool   `toml:"Shared,omitempty" long:"shared" env:"CACHE_SHARED" description:"Enable cache sharing between runners."`
	MaxUploadedArchiveSize int64  `toml:"MaxUploadedArchiveSize,omitempty" long:"max_uploaded_archive_size" env:"CACHE_MAXIMUM_UPLOADED_ARCHIVE_SIZE" description:"Limit the size of the cache archive being uploaded to cloud storage, in bytes."`

	S3    *CacheS3Config    `toml:"s3,omitempty" json:"s3,omitempty" namespace:"s3"`
	GCS   *CacheGCSConfig   `toml:"gcs,omitempty" json:"gcs,omitempty" namespace:"gcs"`
	Azure *CacheAzureConfig `toml:"azure,omitempty" json:"azure,omitempty" namespace:"azure"`
}

type RunnerSettings struct {
	Labels Labels `toml:"labels,omitempty" json:"labels,omitempty" description:"Custom labels for the runner worker. Duplicate keys will override any global defaults in this scope."`

	Executor  string `toml:"executor" json:"executor" long:"executor" env:"RUNNER_EXECUTOR" required:"true" description:"Select executor, eg. shell, docker, etc."`
	BuildsDir string `toml:"builds_dir,omitempty" json:"builds_dir" long:"builds-dir" env:"RUNNER_BUILDS_DIR" description:"Directory where builds are stored"`
	CacheDir  string `toml:"cache_dir,omitempty" json:"cache_dir" long:"cache-dir" env:"RUNNER_CACHE_DIR" description:"Directory where build cache is stored"`
	CloneURL  string `toml:"clone_url,omitempty" json:"clone_url" long:"clone-url" env:"CLONE_URL" description:"Overwrite the default URL used to clone or fetch the git ref"`

	Environment []string `toml:"environment,omitempty" json:"environment,omitempty" long:"env" env:"RUNNER_ENV" description:"Custom environment variables injected to build environment"`

	ProxyExec *bool `toml:"proxy_exec,omitempty" json:"proxy_exec,omitempty" long:"proxy-exec" env:"RUNNER_PROXY_EXEC" description:"(Experimental) Proxy execution via helper binary"`

	PreGetSourcesScript  string `toml:"pre_get_sources_script,omitempty" json:"pre_get_sources_script" long:"pre-get-sources-script" env:"RUNNER_PRE_GET_SOURCES_SCRIPT" description:"Runner-specific commands to be executed on the runner before updating the Git repository and updating submodules."`
	PostGetSourcesScript string `toml:"post_get_sources_script,omitempty" json:"post_get_sources_script" long:"post-get-sources-script" env:"RUNNER_POST_GET_SOURCES_SCRIPT" description:"Runner-specific commands to be executed on the runner after updating the Git repository and updating submodules."`

	PreBuildScript  string `toml:"pre_build_script,omitempty" json:"pre_build_script" long:"pre-build-script" env:"RUNNER_PRE_BUILD_SCRIPT" description:"Runner-specific command script executed just before build executes"`
	PostBuildScript string `toml:"post_build_script,omitempty" json:"post_build_script" long:"post-build-script" env:"RUNNER_POST_BUILD_SCRIPT" description:"Runner-specific command script executed just after build executes"`

	DebugTraceDisabled bool `toml:"debug_trace_disabled,omitempty" json:"debug_trace_disabled" long:"debug-trace-disabled" env:"RUNNER_DEBUG_TRACE_DISABLED" description:"When set to true Runner will disable the possibility of using the CI_DEBUG_TRACE feature"`

	SafeDirectoryCheckout *bool `toml:"safe_directory_checkout,omitempty" json:"safe_directory_checkout,omitempty" long:"safe-directory-checkout" env:"RUNNER_SAFE_DIRECTORY_CHECKOUT" description:"When set to true, Git global configuration will get a safe.directory directive pointing the job's working directory'"`
	CleanGitConfig        *bool `toml:"clean_git_config,omitempty" json:"clean_git_config,omitempty" long:"clean-git-config" env:"RUNNER_CLEAN_GIT_CONFIG" description:"Clean git configuration before and after the build. Defaults to true, except the shell executor is used or the git strategy is \"none\""`

	Shell          string           `toml:"shell,omitempty" json:"shell" long:"shell" env:"RUNNER_SHELL" description:"Select bash, sh, cmd, pwsh or powershell" jsonschema:"enum=bash,enum=sh,enum=cmd,enum=pwsh,enum=powershell,enum="`
	CustomBuildDir CustomBuildDir   `toml:"custom_build_dir,omitempty" json:"custom_build_dir,omitempty" group:"custom build dir configuration" namespace:"custom_build_dir"`
	Referees       *referees.Config `toml:"referees,omitempty" json:"referees,omitempty" group:"referees configuration" namespace:"referees"`
	Cache          *CacheConfig     `toml:"cache,omitempty" json:"cache,omitempty" group:"cache configuration" namespace:"cache"`

	// GracefulKillTimeout and ForceKillTimeout aren't exposed to the users yet
	// because not every executor supports it. We also have to keep in mind that
	// the CustomConfig has its configuration fields for termination so when
	// every executor supports graceful termination we should expose this single
	// configuration for all executors.
	GracefulKillTimeout *int `toml:"-" json:",omitempty"`
	ForceKillTimeout    *int `toml:"-" json:",omitempty"`

	FeatureFlags map[string]bool `toml:"feature_flags" json:"feature_flags,omitempty" long:"feature-flags" env:"FEATURE_FLAGS" description:"Enable/Disable feature flags https://docs.gitlab.com/runner/configuration/feature-flags/"`

	Monitoring *runner.Monitoring `toml:"monitoring,omitempty" json:"monitoring,omitempty" long:"runner-monitoring" description:"(Experimental) Monitoring configuration specific to this runner"`

	// Slot-based cgroup configuration
	UseSlotCgroups     bool   `toml:"use_slot_cgroups,omitempty" json:"use_slot_cgroups" long:"use-slot-cgroups" env:"RUNNER_USE_SLOT_CGROUPS" description:"Use slot-derived cgroup names for resource isolation"`
	SlotCgroupTemplate string `toml:"slot_cgroup_template,omitempty" json:"slot_cgroup_template" long:"slot-cgroup-template" env:"RUNNER_SLOT_CGROUP_TEMPLATE" description:"Template for slot-derived cgroup names (use ${slot} placeholder)"`

	Instance   *InstanceConfig   `toml:"instance,omitempty" json:"instance,omitempty"`
	SSH        *SshConfig        `toml:"ssh,omitempty" json:"ssh,omitempty" group:"ssh executor" namespace:"ssh"`
	Docker     *DockerConfig     `toml:"docker,omitempty" json:"docker,omitempty" group:"docker executor" namespace:"docker"`
	Parallels  *ParallelsConfig  `toml:"parallels,omitempty" json:"parallels,omitempty" group:"parallels executor" namespace:"parallels"`
	VirtualBox *VirtualBoxConfig `toml:"virtualbox,omitempty" json:"virtualbox,omitempty" group:"virtualbox executor" namespace:"virtualbox"`
	Machine    *DockerMachine    `toml:"machine,omitempty" json:"machine,omitempty" group:"docker machine provider" namespace:"machine"`
	Kubernetes *KubernetesConfig `toml:"kubernetes,omitempty" json:"kubernetes,omitempty" group:"kubernetes executor" namespace:"kubernetes"`
	Custom     *CustomConfig     `toml:"custom,omitempty" json:"custom,omitempty" group:"custom executor" namespace:"custom"`

	Autoscaler *AutoscalerConfig `toml:"autoscaler,omitempty" json:",omitempty"`

	StepRunnerImage string `toml:"step_runner_image,omitempty" json:"step_runner_image" long:"step-runner-image" env:"STEP_RUNNER_IMAGE" description:"[ADVANCED] Override the default step-runner image used to inject the step-runner binary into the build container"`

	// this is the combined labels from global defaults and this specific runner's labels
	labels Labels
}

type RunnerConfig struct {
	Name                string `toml:"name" json:"name" short:"name" long:"description" env:"RUNNER_NAME" description:"Runner name"`
	Limit               int    `toml:"limit,omitzero" json:"limit" long:"limit" env:"RUNNER_LIMIT" description:"Maximum number of builds processed by this runner"`
	OutputLimit         int    `toml:"output_limit,omitzero" long:"output-limit" env:"RUNNER_OUTPUT_LIMIT" description:"Maximum build trace size in kilobytes"`
	RequestConcurrency  int    `toml:"request_concurrency,omitzero" long:"request-concurrency" env:"RUNNER_REQUEST_CONCURRENCY" description:"Maximum concurrency for job requests" jsonschema:"min=1"`
	StrictCheckInterval *bool  `toml:"strict_check_interval,omitzero" json:",omitempty" long:"strict-check-interval" env:"RUNNER_STRICT_CHECK_INTERVAL" description:"When you set StrictCheckInterval to true, the runner disables the faster-than-check_interval re-polling loop that occurs when a runner receives a job. Instead, the runner waits <check_interval> seconds before it polls again, even if additional jobs are available."`

	UnhealthyRequestsLimit         int            `toml:"unhealthy_requests_limit,omitzero" long:"unhealthy-requests-limit" env:"RUNNER_UNHEALTHY_REQUESTS_LIMIT" description:"The number of unhealthy responses to new job requests after which a runner worker is turned off."`
	UnhealthyInterval              *time.Duration `toml:"unhealthy_interval,omitzero" json:",omitempty" long:"unhealthy-interval" ENV:"RUNNER_UNHEALTHY_INTERVAL" description:"Duration that the runner worker is turned off after it exceeds the unhealthy requests limit. Supports syntax like '3600s' and '1h30min'."`
	JobStatusFinalUpdateRetryLimit int            `toml:"job_status_final_update_retry_limit,omitzero" json:"job_status_final_update_retry_limit,omitzero" long:"job-status-final-update-retry-limit" env:"RUNNER_job_status_final_update_retry_limit" description:"The maximum number of times GitLab Runner can retry to push the final job status to the GitLab instance."`

	SystemID       string    `toml:"-" json:",omitempty"`
	ConfigLoadedAt time.Time `toml:"-" json:",omitempty"`
	ConfigDir      string    `toml:"-" json:",omitempty"`

	RunnerCredentials
	RunnerSettings
}

type SessionServer struct {
	ListenAddress    string `toml:"listen_address,omitempty" json:"listen_address" description:"Address that the runner will communicate directly with"`
	AdvertiseAddress string `toml:"advertise_address,omitempty" json:"advertise_address" description:"Address the runner will expose to the world to connect to the session server"`
	SessionTimeout   int    `toml:"session_timeout,omitempty" json:"session_timeout" description:"How long a terminal session can be active after a build completes, in seconds"`
}

type Config struct {
	ListenAddress string        `toml:"listen_address,omitempty" json:"listen_address"`
	SessionServer SessionServer `toml:"session_server,omitempty" json:"session_server"`

	Labels Labels `toml:"labels,omitempty" json:"labels,omitempty" description:"Default custom labels for all runners."`

	Concurrent       int             `toml:"concurrent" json:"concurrent"`
	CheckInterval    int             `toml:"check_interval" json:"check_interval" description:"Define active checking interval of jobs"`
	LogLevel         *string         `toml:"log_level" json:"log_level,omitempty" description:"Define log level (one of: panic, fatal, error, warning, info, debug)"`
	LogFormat        *string         `toml:"log_format" json:"log_format,omitempty" description:"Define log format (one of: runner, text, json)"`
	User             string          `toml:"user,omitempty" json:"user"`
	Runners          []*RunnerConfig `toml:"runners" json:"runners,omitempty"`
	SentryDSN        *string         `toml:"sentry_dsn" json:",omitempty"`
	ConnectionMaxAge *time.Duration  `toml:"connection_max_age,omitempty" json:"connection_max_age,omitempty"`
	ModTime          time.Time       `toml:"-"`
	Loaded           bool            `toml:"-"`

	Experimental *Experimental `toml:"experimental" json:"experimental,omitempty"`

	ShutdownTimeout int `toml:"shutdown_timeout,omitempty" json:"shutdown_timeout" description:"Number of seconds until the forceful shutdown operation times out and exits the process"`

	ConfigSaver ConfigSaver `toml:"-"`
}

type Experimental struct {
	UsageLogger UsageLogger `toml:"usage_logger" json:"usage_logger,omitempty"`
}

type UsageLogger struct {
	Enabled        bool              `toml:"enabled" json:"enabled"`
	LogDir         string            `toml:"log_dir,omitempty" json:"log_dir,omitempty"`
	MaxBackupFiles *int64            `toml:"max_backup_files,omitempty" json:"max_backup_files,omitempty"`
	MaxRotationAge *time.Duration    `toml:"max_rotation_age,omitempty" json:"max_rotation_age,omitempty"`
	Labels         map[string]string `toml:"labels,omitempty" json:"labels,omitempty"`
}

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

type CustomBuildDir struct {
	Enabled *bool `toml:"enabled,omitempty" json:"enabled,omitempty" long:"enabled" env:"CUSTOM_BUILD_DIR_ENABLED" description:"Enable job specific build directories"`
}

type S3AuthType string

const (
	S3AuthTypeAccessKey S3AuthType = "access-key"
	S3AuthTypeIAM       S3AuthType = "iam"
)

type S3EncryptionType string

const (
	S3EncryptionTypeNone    S3EncryptionType = ""
	S3EncryptionTypeAes256  S3EncryptionType = "S3"
	S3EncryptionTypeKms     S3EncryptionType = "KMS"
	S3EncryptionTypeDsseKms S3EncryptionType = "DSSE-KMS"
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

func (c *CacheS3Config) EncryptionType() S3EncryptionType {
	encryptionType := S3EncryptionType(strings.ToUpper(c.ServerSideEncryption))

	switch encryptionType {
	case "":
		return S3EncryptionTypeNone
	case "S3", "AES256":
		return S3EncryptionTypeAes256
	case "KMS", "AWS:KMS":
		return S3EncryptionTypeKms
	case "DSSE-KMS", "AWS:KMS:DSSE":
		return S3EncryptionTypeDsseKms
	}

	logrus.Warnf("unknown ServerSideEncryption value: %s", encryptionType)
	return S3EncryptionTypeNone
}

func (c *CacheS3Config) GetEndpoint() string {
	if c.ServerAddress == "" {
		return ""
	}

	scheme := "https"
	if c.Insecure {
		scheme = "http"
	}

	host, port, err := net.SplitHostPort(c.ServerAddress)
	if err != nil {
		// If SplitHostPort fails, it means there's no port specified
		// so we can use the ServerAddress as-is.
		return fmt.Sprintf("%s://%s", scheme, c.ServerAddress)
	}

	// Omit canonical ports
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		return fmt.Sprintf("%s://%s", scheme, host)
	}

	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

func (c *CacheS3Config) GetEndpointURL() *url.URL {
	endpoint := c.GetEndpoint()
	if endpoint == "" {
		return nil
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		logrus.Errorf("error parsing endpoint URL: %v", err)
		return nil
	}

	return u
}

// PathStyleEnabled() will return true if the endpoint needs to use
// the legacy, path-style access to S3. If the value is not specified,
// it will auto-detect and return false if the server address appears
// to be for AWS or Google. Otherwise, PathStyleEnabled() will return false.
func (c *CacheS3Config) PathStyleEnabled() bool {
	// Preserve the previous behavior of auto-detection by default
	if c.PathStyle == nil {
		u := c.GetEndpointURL()
		if u == nil {
			return false
		}

		return !s3utils.IsVirtualHostSupported(*u, c.BucketName)
	}

	return *c.PathStyle
}

func (c *CacheS3Config) DualStackEnabled() bool {
	if c.DualStack == nil {
		return true
	}
	return *c.DualStack
}

func (c *CacheConfig) GetPath() string {
	return c.Path
}

func (c *CacheConfig) GetShared() bool {
	return c.Shared
}

func (r *RunnerSettings) ComputeLabels(globalDefaults Labels) {
	r.labels = make(Labels)

	for k, v := range globalDefaults {
		r.labels[k] = v
	}

	for k, v := range r.Labels {
		r.labels[k] = v
	}
}

func (r *RunnerSettings) ComputedLabels() Labels {
	return r.labels
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

type SshConfig struct {
	User                         string `toml:"user,omitempty" json:"user,omitempty" long:"user" env:"SSH_USER" description:"User name"`
	Password                     string `toml:"password,omitempty" json:"password,omitempty" long:"password" env:"SSH_PASSWORD" description:"User password"`
	Host                         string `toml:"host,omitempty" json:"host,omitempty" long:"host" env:"SSH_HOST" description:"Remote host"`
	Port                         string `toml:"port,omitempty" json:"port,omitempty" long:"port" env:"SSH_PORT" description:"Remote host port"`
	IdentityFile                 string `toml:"identity_file,omitempty" json:"identity_file,omitempty" long:"identity-file" env:"SSH_IDENTITY_FILE" description:"Identity file to be used"`
	DisableStrictHostKeyChecking *bool  `toml:"disable_strict_host_key_checking,omitempty" json:"disable_strict_host_key_checking,omitempty" long:"disable-strict-host-key-checking" env:"DISABLE_STRICT_HOST_KEY_CHECKING" description:"Disable SSH strict host key checking"`
	KnownHostsFile               string `toml:"known_hosts_file,omitempty" json:"known_hosts_file,omitempty" long:"known-hosts-file" env:"KNOWN_HOSTS_FILE" description:"Location of known_hosts file. Defaults to ~/.ssh/known_hosts"`
}

func (c *SshConfig) ShouldDisableStrictHostKeyChecking() bool {
	return c.DisableStrictHostKeyChecking != nil && *c.DisableStrictHostKeyChecking
}

func (c *DockerConfig) computeNanoCPUs(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}

	cpu, ok := new(big.Rat).SetString(value)
	if !ok {
		return 0, fmt.Errorf("failed to parse %s as a rational number", value)
	}

	nano, _ := cpu.Mul(cpu, big.NewRat(1e9, 1)).Float64()

	return int64(nano), nil
}

func (c *DockerConfig) GetNanoCPUs() (int64, error) {
	return c.computeNanoCPUs(c.CPUS)
}

func (c *DockerConfig) GetServiceNanoCPUs() (int64, error) {
	return c.computeNanoCPUs(c.ServiceCPUS)
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

func (c *DockerConfig) GetServiceMemory() int64 {
	return c.getMemoryBytes(c.ServiceMemory, "service_memory")
}

func (c *DockerConfig) GetServiceMemorySwap() int64 {
	return c.getMemoryBytes(c.ServiceMemorySwap, "service_memory_swap")
}

func (c *DockerConfig) GetServiceMemoryReservation() int64 {
	return c.getMemoryBytes(c.ServiceMemoryReservation, "service_memory_reservation")
}

func (c *DockerConfig) GetOomKillDisable() *bool {
	return &c.OomKillDisable
}

func getExpandedServices(services []Service, vars spec.Variables) []Service {
	result := []Service{}
	for _, s := range services {
		s.Name = vars.ExpandValue(s.Name)
		s.Alias = vars.ExpandValue(s.Alias)
		result = append(result, s)
	}
	return result
}

// GetExpandedServices returns the executor-configured services, with the values expanded. This is necessary because
// some of the values in service definition can point to job variables, so the final value is job-dependant.
// See: https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29499
func (c *DockerConfig) GetExpandedServices(vars spec.Variables) []Service {
	return getExpandedServices(c.Services, vars)
}

func (c *DockerConfig) GetServicesLimit() int {
	if c.ServicesLimit == nil {
		return -1
	}

	return *c.ServicesLimit
}

// GetLogConfig returns the LogConfig for build containers
func (c *DockerConfig) GetLogConfig() (container.LogConfig, error) {
	logConfig := container.LogConfig{
		Type: "json-file",
	}

	if c == nil || len(c.LogOptions) == 0 {
		return logConfig, nil
	}

	var invalidKeys []string
	var allowedKeys = []string{"env", "labels"}

	for key := range c.LogOptions {
		if !slices.Contains(allowedKeys, key) {
			invalidKeys = append(invalidKeys, key)
		}
	}

	slices.Sort(invalidKeys) // to get stable error outputs

	if len(invalidKeys) > 0 {
		return logConfig, fmt.Errorf("invalid log options: only %q are allowed, but found: %q", allowedKeys, invalidKeys)
	}

	logConfig.Config = c.LogOptions

	return logConfig, nil
}

func (c *KubernetesConfig) GetPollTimeout() int {
	if c.PollTimeout <= 0 {
		c.PollTimeout = KubernetesPollTimeout
	}
	return c.PollTimeout
}

func (c *KubernetesConfig) GetPollInterval() int {
	if c.PollInterval <= 0 {
		c.PollInterval = KubernetesPollInterval
	}
	return c.PollInterval
}

func (c *KubernetesConfig) GetPollAttempts() int {
	return c.GetPollTimeout() / c.GetPollInterval()
}

func (c *KubernetesConfig) GetCleanupResourcesTimeout() time.Duration {
	if c.CleanupResourcesTimeout == nil || c.CleanupResourcesTimeout.Seconds() <= 0 {
		return KubernetesCleanupResourcesTimeout
	}

	return *c.CleanupResourcesTimeout
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
		expression := metav1.LabelSelectorRequirement{
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
	nodeSelectorTerm := api.NodeSelectorTerm{}
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

// GetExpandedServices returns the executor-configured services, with the values expanded. This is necessary because
// some of the values in service definition can point to job variables, so the final value is job-dependant.
// See: https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29499
func (c *KubernetesConfig) GetExpandedServices(vars spec.Variables) []Service {
	return getExpandedServices(c.Services, vars)
}

func (c *KubernetesConfig) GetPrintPodWarningEvents() bool {
	if c.PrintPodWarningEvents == nil {
		return true
	}

	return *c.PrintPodWarningEvents
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
			"https://docs.gitlab.com/runner/configuration/autoscale/#off-peak-time-mode-configuration-deprecated")
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
	// Shorten the token to ensure that it won't be exposed in logged messages.
	token := helpers.ShortenToken(c.Token)
	return c.URL + token
}

func (c *RunnerCredentials) SameAs(other *RunnerCredentials) bool {
	if c.Token != other.Token {
		return false
	}
	if wildcardURL(c.URL) || wildcardURL(other.URL) {
		return true
	}
	return c.URL == other.URL
}

func (c *RunnerConfig) String() string {
	return fmt.Sprintf("%v url=%v token=%v executor=%v", c.Name, c.URL, c.Token, c.Executor)
}

func (c *RunnerConfig) WarnOnLegacyCIURL() {
	if strings.HasSuffix(strings.TrimRight(c.URL, "/"), "/ci") {
		c.Log().Warning("The runner URL contains a legacy '/ci' suffix.\n" +
			"  This suffix is deprecated and should be removed from the configuration.\n" +
			"  Git submodules may fail to clone with authentication errors if this suffix is present.\n" +
			"  Please update the 'url' field in your config.toml to remove the '/ci' suffix.\n" +
			"  See https://docs.gitlab.com/runner/configuration/advanced-configuration.html#legacy-ci-url-suffix")
	}
}

func (c *RunnerConfig) GetSystemID() string {
	if c.SystemID == "" {
		return UnknownSystemID
	}

	return c.SystemID
}

func (c *RunnerConfig) GetUnhealthyRequestsLimit() int {
	if c.UnhealthyRequestsLimit < 1 {
		return DefaultUnhealthyRequestsLimit
	}

	return c.UnhealthyRequestsLimit
}

func (c *RunnerConfig) GetJobStatusFinalUpdateRetryLimit() int {
	if c.JobStatusFinalUpdateRetryLimit < 1 {
		return DefaultFinalUpdateRetryLimit
	}

	return c.JobStatusFinalUpdateRetryLimit
}

func (c *RunnerConfig) GetUnhealthyInterval() time.Duration {
	if c.UnhealthyInterval == nil {
		return DefaultUnhealthyInterval
	}

	return *c.UnhealthyInterval
}

func (c *RunnerConfig) GetRequestConcurrency() int {
	return max(1, c.RequestConcurrency)
}

func (c *RunnerConfig) GetStrictCheckInterval() bool {
	if c.StrictCheckInterval == nil {
		return false
	}

	return *c.StrictCheckInterval
}

func (c *RunnerConfig) GetVariables() spec.Variables {
	variables := spec.Variables{
		{Key: "CI_RUNNER_SHORT_TOKEN", Value: c.ShortDescription(), Public: true, Internal: true, File: false},
	}

	for _, environment := range c.Environment {
		if variable, err := parseVariable(environment); err == nil {
			variable.Internal = true
			variables = append(variables, variable)
		}
	}

	return variables
}

func (c *RunnerConfig) IsProxyExec() bool {
	if c.ProxyExec != nil {
		return *c.ProxyExec
	}

	return false
}

func (c *RunnerConfig) Log() *logrus.Entry {
	logger := c.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	entry := logger.WithFields(logrus.Fields{})

	if c.ShortDescription() != "" {
		entry = entry.WithField("runner", c.ShortDescription())
	}
	if c.Name != "" {
		entry = entry.WithField("runner_name", c.Name)
	}

	return entry
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

	r.SystemID = c.SystemID
	r.ConfigLoadedAt = c.ConfigLoadedAt
	r.ConfigDir = c.ConfigDir

	if r.Monitoring != nil {
		err = r.Monitoring.Compile()
		if err != nil {
			return nil, fmt.Errorf("compiling monitoring sections: %w", err)
		}
	}

	return &r, err
}

// mask masks all sensitive fields on a Runner.
// This should only run against a deep copy of a RunnerConfig.
func (r *RunnerConfig) mask() {
	if r == nil {
		return
	}

	maskField(&r.Token)
	if k8s := r.Kubernetes; k8s != nil {
		maskField(&k8s.BearerToken)
	}
	if cache := r.Cache; cache != nil {
		if s3 := cache.S3; s3 != nil {
			maskField(&s3.AccessKey)
			maskField(&s3.SecretKey)
			maskField(&s3.SessionToken)
		}
		if gcs := cache.GCS; gcs != nil {
			maskField(&gcs.PrivateKey)
		}
		if azure := cache.Azure; azure != nil {
			maskField(&azure.AccountKey)
		}
	}
}

func NewConfigWithSaver(s ConfigSaver) *Config {
	c := NewConfig()
	c.ConfigSaver = s

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

// DeepCopy returns a deep clone of the config struct.
func (c *Config) DeepCopy() (*Config, error) {
	var d Config
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("serialize config: %w", err)
	}
	if err = json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("deserialize config: %w", err)
	}
	return &d, nil
}

// Masked returns a copy of the config struct with sensitive fields masked.
func (c *Config) Masked() (*Config, error) {
	m, err := c.DeepCopy()
	if err != nil {
		return nil, fmt.Errorf("deep copy config: %w", err)
	}

	for _, r := range m.Runners {
		r.mask()
	}
	return m, nil
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
		return fmt.Errorf("decoding configuration file: %w", err)
	}

	for _, r := range c.Runners {
		err := r.loadConfig(c)
		if err != nil {
			return fmt.Errorf("loading coniguration for %s runner: %w", r.Name, err)
		}
	}

	// config built-in validation is blocking when doesn't pass
	err = c.Validate()
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	c.ModTime = info.ModTime()

	if c.ConnectionMaxAge == nil {
		defaultValue := DefaultConnectionMaxAge
		c.ConnectionMaxAge = &defaultValue
	}

	c.Loaded = true

	return nil
}

func (c *RunnerConfig) loadConfig(globalCfg *Config) error {
	if c.Machine != nil {
		err := c.Machine.CompilePeriods()
		if err != nil {
			return fmt.Errorf("compiling docker machine autoscaling periods: %w", err)
		}
		c.Machine.logDeprecationWarning()
	}

	if c.Monitoring != nil {
		err := c.Monitoring.Compile()
		if err != nil {
			return fmt.Errorf("compiling monitoring sections: %w", err)
		}
	}

	c.RunnerSettings.ComputeLabels(globalCfg.Labels)

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

	if c.ConfigSaver == nil {
		c.ConfigSaver = new(defaultConfigSaver)
	}

	if err := c.ConfigSaver.Save(configFile, newConfig.Bytes()); err != nil {
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

// maskField masks the content of a string field
// if it is not empty.
func maskField(field *string) {
	if field != nil && *field != "" {
		*field = mask
	}
}

func (c *Config) RunnerByName(name string) (*RunnerConfig, error) {
	for _, runner := range c.Runners {
		if runner.Name == name {
			return runner, nil
		}
	}

	return nil, fmt.Errorf("could not find a runner with the name '%s'", name)
}

func (c *Config) RunnerByToken(token string) (*RunnerConfig, error) {
	for _, runner := range c.Runners {
		if runner.Token == token {
			return runner, nil
		}
	}

	return nil, fmt.Errorf("could not find a runner with the token '%s'", helpers.ShortenToken(token))
}

func (c *Config) RunnerByURLAndID(url string, id int64) (*RunnerConfig, error) {
	for _, runner := range c.Runners {
		if runner.URL == url && runner.ID == id {
			return runner, nil
		}
	}

	return nil, fmt.Errorf("could not find a runner with the URL %q and ID %d", url, id)
}

func (c *Config) RunnerByNameAndToken(name string, token string) (*RunnerConfig, error) {
	for _, runner := range c.Runners {
		if runner.Name == name && runner.Token == token {
			return runner, nil
		}
	}

	return nil, fmt.Errorf("could not find a runner with the Name '%s' and Token '%s'", name, token)
}

func (c *Config) Validate() error {
	for vn, v := range map[string]func() error{
		"global labels": c.validateLabels,
	} {
		err := v()
		if err != nil {
			return fmt.Errorf("validating %s: %w", vn, err)
		}
	}

	for _, r := range c.Runners {
		err := r.Validate()
		if err != nil {
			return fmt.Errorf("validating runner %s: %w", r.Name, err)
		}
	}

	return nil
}

func (c *Config) validateLabels() error {
	return c.Labels.validatePatterns()
}

func (c *RunnerConfig) Validate() error {
	for vn, v := range map[string]func() error{
		"labels":                    c.validateLabels,
		"computed labels":           c.validateComputedLabels,
		"slot cgroups":              c.validateSlotCgroups,
		"machine options with name": c.validateMachineOptionsWithName,
	} {
		err := v()
		if err != nil {
			return fmt.Errorf("validating %s: %w", vn, err)
		}
	}

	return nil
}

func (c *RunnerConfig) validateLabels() error {
	return c.Labels.validatePatterns()
}

func (c *RunnerConfig) validateComputedLabels() error {
	return c.labels.validateCount()
}

func (c *RunnerConfig) validateSlotCgroups() error {
	if !c.UseSlotCgroups {
		return nil
	}

	// Validate main slot cgroup template
	template := c.SlotCgroupTemplate
	if template == "" {
		template = DefaultSlotCgroupTemplate
	}
	validateSlotCgroupTemplate(template, "slot_cgroup_template")

	// Validate service slot cgroup template if configured
	if c.Docker != nil && c.Docker.ServiceSlotCgroupTemplate != "" {
		validateSlotCgroupTemplate(c.Docker.ServiceSlotCgroupTemplate, "service_slot_cgroup_template")
	}

	return nil
}

func (c *RunnerConfig) validateMachineOptionsWithName() error {
	if c.Machine == nil {
		return nil
	}

	for _, opt := range c.Machine.MachineOptionsWithName {
		if !strings.Contains(opt, "%s") {
			return fmt.Errorf("machine option with name %q must contain %%s placeholder", opt)
		}
	}
	return nil
}

const DefaultSlotCgroupTemplate = "gitlab-runner/slot-${slot}"

// GetSlot extracts the slot number from ExecutorData if available, otherwise returns -1
func GetSlot(data ExecutorData) int {
	if s, ok := data.(interface{ AcquisitionSlot() int }); ok {
		return s.AcquisitionSlot()
	}
	logrus.WithField("data_type", fmt.Sprintf("%T", data)).
		Debug("ExecutorData does not implement AcquisitionSlot() interface")
	return -1
}

// GetSlotCgroupPath returns the cgroup path for the given slot and ExecutorData
func (c *RunnerConfig) GetSlotCgroupPath(data ExecutorData) string {
	if !c.UseSlotCgroups {
		return ""
	}

	slot := GetSlot(data)
	if slot < 0 {
		return ""
	}

	template := c.SlotCgroupTemplate
	if template == "" {
		template = DefaultSlotCgroupTemplate
	}

	return expandSlotTemplate(template, slot)
}

// GetServiceSlotCgroupPath returns the cgroup path for service containers
func (c *RunnerConfig) GetServiceSlotCgroupPath(data ExecutorData) string {
	if !c.UseSlotCgroups {
		return ""
	}

	slot := GetSlot(data)
	if slot < 0 {
		return ""
	}

	var template string
	if c.Docker != nil && c.Docker.ServiceSlotCgroupTemplate != "" {
		template = c.Docker.ServiceSlotCgroupTemplate
	} else {
		template = c.SlotCgroupTemplate
		if template == "" {
			template = DefaultSlotCgroupTemplate
		}
	}

	return expandSlotTemplate(template, slot)
}

// validateSlotCgroupTemplate checks if the template contains the ${slot} placeholder and logs a warning if not
func validateSlotCgroupTemplate(template string, configName string) {
	if !strings.Contains(template, "${slot}") && !strings.Contains(template, "$slot") {
		logrus.WithFields(logrus.Fields{
			"template":    template,
			"config_name": configName,
		}).Warning("Slot cgroup template does not contain ${slot} placeholder. " +
			"All jobs will use the same cgroup, defeating the purpose of slot-based isolation. " +
			"Consider using a template like 'gitlab-runner/slot-${slot}'")
	}
}

// expandSlotTemplate replaces ${slot} placeholder with actual slot number using os.Expand
func expandSlotTemplate(template string, slot int) string {
	slotStr := strconv.Itoa(slot)
	return os.Expand(template, func(name string) string {
		if name == "slot" {
			return slotStr
		}
		return ""
	})
}

func parseVariable(text string) (variable spec.Variable, err error) {
	keyValue := strings.SplitN(text, "=", 2)
	if len(keyValue) != 2 {
		err = errors.New("missing =")
		return
	}
	variable = spec.Variable{
		Key:   keyValue[0],
		Value: keyValue[1],
	}
	return
}

// wildcardURL checks if the URL is a wildcard URL
func wildcardURL(url string) bool {
	switch url {
	case "", "*":
		return true
	default:
		return false
	}
}

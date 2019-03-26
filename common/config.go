package common

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/timeperiod"
)

type DockerPullPolicy string
type DockerSysCtls map[string]string

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
	Hostname                   string            `toml:"hostname,omitempty" json:"hostname" long:"hostname" env:"DOCKER_HOSTNAME" description:"Custom container hostname"`
	Image                      string            `toml:"image" json:"image" long:"image" env:"DOCKER_IMAGE" description:"Docker image to be used"`
	Runtime                    string            `toml:"runtime,omitempty" json:"runtime" long:"runtime" env:"DOCKER_RUNTIME" description:"Docker runtime to be used"`
	Memory                     string            `toml:"memory,omitempty" json:"memory" long:"memory" env:"DOCKER_MEMORY" description:"Memory limit (format: <number>[<unit>]). Unit can be one of b, k, m, or g. Minimum is 4M."`
	MemorySwap                 string            `toml:"memory_swap,omitempty" json:"memory_swap" long:"memory-swap" env:"DOCKER_MEMORY_SWAP" description:"Total memory limit (memory + swap, format: <number>[<unit>]). Unit can be one of b, k, m, or g."`
	MemoryReservation          string            `toml:"memory_reservation,omitempty" json:"memory_reservation" long:"memory-reservation" env:"DOCKER_MEMORY_RESERVATION" description:"Memory soft limit (format: <number>[<unit>]). Unit can be one of b, k, m, or g."`
	CPUSetCPUs                 string            `toml:"cpuset_cpus,omitempty" json:"cpuset_cpus" long:"cpuset-cpus" env:"DOCKER_CPUSET_CPUS" description:"String value containing the cgroups CpusetCpus to use"`
	CPUS                       string            `toml:"cpus,omitempty" json:"cpus" long:"cpus" env:"DOCKER_CPUS" description:"Number of CPUs"`
	DNS                        []string          `toml:"dns,omitempty" json:"dns" long:"dns" env:"DOCKER_DNS" description:"A list of DNS servers for the container to use"`
	DNSSearch                  []string          `toml:"dns_search,omitempty" json:"dns_search" long:"dns-search" env:"DOCKER_DNS_SEARCH" description:"A list of DNS search domains"`
	Privileged                 bool              `toml:"privileged,omitzero" json:"privileged" long:"privileged" env:"DOCKER_PRIVILEGED" description:"Give extended privileges to container"`
	DisableEntrypointOverwrite bool              `toml:"disable_entrypoint_overwrite,omitzero" json:"disable_entrypoint_overwrite" long:"disable-entrypoint-overwrite" env:"DOCKER_DISABLE_ENTRYPOINT_OVERWRITE" description:"Disable the possibility for a container to overwrite the default image entrypoint"`
	UsernsMode                 string            `toml:"userns_mode,omitempty" json:"userns_mode" long:"userns" env:"DOCKER_USERNS_MODE" description:"User namespace to use"`
	CapAdd                     []string          `toml:"cap_add" json:"cap_add" long:"cap-add" env:"DOCKER_CAP_ADD" description:"Add Linux capabilities"`
	CapDrop                    []string          `toml:"cap_drop" json:"cap_drop" long:"cap-drop" env:"DOCKER_CAP_DROP" description:"Drop Linux capabilities"`
	OomKillDisable             bool              `toml:"oom_kill_disable,omitzero" json:"oom_kill_disable" long:"oom-kill-disable" env:"DOCKER_OOM_KILL_DISABLE" description:"Do not kill processes in a container if an out-of-memory (OOM) error occurs"`
	SecurityOpt                []string          `toml:"security_opt" json:"security_opt" long:"security-opt" env:"DOCKER_SECURITY_OPT" description:"Security Options"`
	Devices                    []string          `toml:"devices" json:"devices" long:"devices" env:"DOCKER_DEVICES" description:"Add a host device to the container"`
	DisableCache               bool              `toml:"disable_cache,omitzero" json:"disable_cache" long:"disable-cache" env:"DOCKER_DISABLE_CACHE" description:"Disable all container caching"`
	Volumes                    []string          `toml:"volumes,omitempty" json:"volumes" long:"volumes" env:"DOCKER_VOLUMES" description:"Bind-mount a volume and create it if it doesn't exist prior to mounting. Can be specified multiple times once per mountpoint, e.g. --docker-volumes 'test0:/test0' --docker-volumes 'test1:/test1'"`
	VolumeDriver               string            `toml:"volume_driver,omitempty" json:"volume_driver" long:"volume-driver" env:"DOCKER_VOLUME_DRIVER" description:"Volume driver to be used"`
	CacheDir                   string            `toml:"cache_dir,omitempty" json:"cache_dir" long:"cache-dir" env:"DOCKER_CACHE_DIR" description:"Directory where to store caches"`
	ExtraHosts                 []string          `toml:"extra_hosts,omitempty" json:"extra_hosts" long:"extra-hosts" env:"DOCKER_EXTRA_HOSTS" description:"Add a custom host-to-IP mapping"`
	VolumesFrom                []string          `toml:"volumes_from,omitempty" json:"volumes_from" long:"volumes-from" env:"DOCKER_VOLUMES_FROM" description:"A list of volumes to inherit from another container"`
	NetworkMode                string            `toml:"network_mode,omitempty" json:"network_mode" long:"network-mode" env:"DOCKER_NETWORK_MODE" description:"Add container to a custom network"`
	Links                      []string          `toml:"links,omitempty" json:"links" long:"links" env:"DOCKER_LINKS" description:"Add link to another container"`
	Services                   []string          `toml:"services,omitempty" json:"services" long:"services" env:"DOCKER_SERVICES" description:"Add service that is started with container"`
	WaitForServicesTimeout     int               `toml:"wait_for_services_timeout,omitzero" json:"wait_for_services_timeout" long:"wait-for-services-timeout" env:"DOCKER_WAIT_FOR_SERVICES_TIMEOUT" description:"How long to wait for service startup"`
	AllowedImages              []string          `toml:"allowed_images,omitempty" json:"allowed_images" long:"allowed-images" env:"DOCKER_ALLOWED_IMAGES" description:"Whitelist allowed images"`
	AllowedServices            []string          `toml:"allowed_services,omitempty" json:"allowed_services" long:"allowed-services" env:"DOCKER_ALLOWED_SERVICES" description:"Whitelist allowed services"`
	PullPolicy                 DockerPullPolicy  `toml:"pull_policy,omitempty" json:"pull_policy" long:"pull-policy" env:"DOCKER_PULL_POLICY" description:"Image pull policy: never, if-not-present, always"`
	ShmSize                    int64             `toml:"shm_size,omitempty" json:"shm_size" long:"shm-size" env:"DOCKER_SHM_SIZE" description:"Shared memory size for docker images (in bytes)"`
	Tmpfs                      map[string]string `toml:"tmpfs,omitempty" json:"tmpfs" long:"tmpfs" env:"DOCKER_TMPFS" description:"A toml table/json object with the format key=values. When set this will mount the specified path in the key as a tmpfs volume in the main container, using the options specified as key. For the supported options, see the documentation for the unix 'mount' command"`
	ServicesTmpfs              map[string]string `toml:"services_tmpfs,omitempty" json:"services_tmpfs" long:"services-tmpfs" env:"DOCKER_SERVICES_TMPFS" description:"A toml table/json object with the format key=values. When set this will mount the specified path in the key as a tmpfs volume in all the service containers, using the options specified as key. For the supported options, see the documentation for the unix 'mount' command"`
	SysCtls                    DockerSysCtls     `toml:"sysctls,omitempty" json:"sysctls" long:"sysctls" env:"DOCKER_SYSCTLS" description:"Sysctl options, a toml table/json object of key=value. Value is expected to be a string."`
	HelperImage                string            `toml:"helper_image,omitempty" json:"helper_image" long:"helper-image" env:"DOCKER_HELPER_IMAGE" description:"[ADVANCED] Override the default helper image used to clone repos and upload artifacts"`
}

type DockerMachine struct {
	IdleCount      int      `long:"idle-nodes" env:"MACHINE_IDLE_COUNT" description:"Maximum idle machines"`
	IdleTime       int      `toml:"IdleTime,omitzero" long:"idle-time" env:"MACHINE_IDLE_TIME" description:"Minimum time after node can be destroyed"`
	MaxBuilds      int      `toml:"MaxBuilds,omitzero" long:"max-builds" env:"MACHINE_MAX_BUILDS" description:"Maximum number of builds processed by machine"`
	MachineDriver  string   `long:"machine-driver" env:"MACHINE_DRIVER" description:"The driver to use when creating machine"`
	MachineName    string   `long:"machine-name" env:"MACHINE_NAME" description:"The template for machine name (needs to include %s)"`
	MachineOptions []string `long:"machine-options" env:"MACHINE_OPTIONS" description:"Additional machine creation options"`

	OffPeakPeriods   []string `long:"off-peak-periods" env:"MACHINE_OFF_PEAK_PERIODS" description:"Time periods when the scheduler is in the OffPeak mode"`
	OffPeakTimezone  string   `long:"off-peak-timezone" env:"MACHINE_OFF_PEAK_TIMEZONE" description:"Timezone for the OffPeak periods (defaults to Local)"`
	OffPeakIdleCount int      `long:"off-peak-idle-count" env:"MACHINE_OFF_PEAK_IDLE_COUNT" description:"Maximum idle machines when the scheduler is in the OffPeak mode"`
	OffPeakIdleTime  int      `long:"off-peak-idle-time" env:"MACHINE_OFF_PEAK_IDLE_TIME" description:"Minimum time after machine can be destroyed when the scheduler is in the OffPeak mode"`

	offPeakTimePeriods *timeperiod.TimePeriod
}

type ParallelsConfig struct {
	BaseName         string `toml:"base_name" json:"base_name" long:"base-name" env:"PARALLELS_BASE_NAME" description:"VM name to be used"`
	TemplateName     string `toml:"template_name,omitempty" json:"template_name" long:"template-name" env:"PARALLELS_TEMPLATE_NAME" description:"VM template to be created"`
	DisableSnapshots bool   `toml:"disable_snapshots,omitzero" json:"disable_snapshots" long:"disable-snapshots" env:"PARALLELS_DISABLE_SNAPSHOTS" description:"Disable snapshoting to speedup VM creation"`
	TimeServer       string `toml:"time_server,omitempty" json:"time_server" long:"time-server" env:"PARALLELS_TIME_SERVER" description:"Timeserver to sync the guests time from. Defaults to time.apple.com"`
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
	Host                           string                       `toml:"host" json:"host" long:"host" env:"KUBERNETES_HOST" description:"Optional Kubernetes master host URL (auto-discovery attempted if not specified)"`
	CertFile                       string                       `toml:"cert_file,omitempty" json:"cert_file" long:"cert-file" env:"KUBERNETES_CERT_FILE" description:"Optional Kubernetes master auth certificate"`
	KeyFile                        string                       `toml:"key_file,omitempty" json:"key_file" long:"key-file" env:"KUBERNETES_KEY_FILE" description:"Optional Kubernetes master auth private key"`
	CAFile                         string                       `toml:"ca_file,omitempty" json:"ca_file" long:"ca-file" env:"KUBERNETES_CA_FILE" description:"Optional Kubernetes master auth ca certificate"`
	BearerTokenOverwriteAllowed    bool                         `toml:"bearer_token_overwrite_allowed" json:"bearer_token_overwrite_allowed" long:"bearer_token_overwrite_allowed" env:"KUBERNETES_BEARER_TOKEN_OVERWRITE_ALLOWED" description:"Bool to authorize builds to specify their own bearer token for creation."`
	BearerToken                    string                       `toml:"bearer_token,omitempty" json:"bearer_token" long:"bearer_token" env:"KUBERNETES_BEARER_TOKEN" description:"Optional Kubernetes service account token used to start build pods."`
	Image                          string                       `toml:"image" json:"image" long:"image" env:"KUBERNETES_IMAGE" description:"Default docker image to use for builds when none is specified"`
	Namespace                      string                       `toml:"namespace" json:"namespace" long:"namespace" env:"KUBERNETES_NAMESPACE" description:"Namespace to run Kubernetes jobs in"`
	NamespaceOverwriteAllowed      string                       `toml:"namespace_overwrite_allowed" json:"namespace_overwrite_allowed" long:"namespace_overwrite_allowed" env:"KUBERNETES_NAMESPACE_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_NAMESPACE_OVERWRITE' value"`
	Privileged                     bool                         `toml:"privileged,omitzero" json:"privileged" long:"privileged" env:"KUBERNETES_PRIVILEGED" description:"Run all containers with the privileged flag enabled"`
	CPULimit                       string                       `toml:"cpu_limit,omitempty" json:"cpu_limit" long:"cpu-limit" env:"KUBERNETES_CPU_LIMIT" description:"The CPU allocation given to build containers"`
	MemoryLimit                    string                       `toml:"memory_limit,omitempty" json:"memory_limit" long:"memory-limit" env:"KUBERNETES_MEMORY_LIMIT" description:"The amount of memory allocated to build containers"`
	ServiceCPULimit                string                       `toml:"service_cpu_limit,omitempty" json:"service_cpu_limit" long:"service-cpu-limit" env:"KUBERNETES_SERVICE_CPU_LIMIT" description:"The CPU allocation given to build service containers"`
	ServiceMemoryLimit             string                       `toml:"service_memory_limit,omitempty" json:"service_memory_limit" long:"service-memory-limit" env:"KUBERNETES_SERVICE_MEMORY_LIMIT" description:"The amount of memory allocated to build service containers"`
	HelperCPULimit                 string                       `toml:"helper_cpu_limit,omitempty" json:"helper_cpu_limit" long:"helper-cpu-limit" env:"KUBERNETES_HELPER_CPU_LIMIT" description:"The CPU allocation given to build helper containers"`
	HelperMemoryLimit              string                       `toml:"helper_memory_limit,omitempty" json:"helper_memory_limit" long:"helper-memory-limit" env:"KUBERNETES_HELPER_MEMORY_LIMIT" description:"The amount of memory allocated to build helper containers"`
	CPURequest                     string                       `toml:"cpu_request,omitempty" json:"cpu_request" long:"cpu-request" env:"KUBERNETES_CPU_REQUEST" description:"The CPU allocation requested for build containers"`
	MemoryRequest                  string                       `toml:"memory_request,omitempty" json:"memory_request" long:"memory-request" env:"KUBERNETES_MEMORY_REQUEST" description:"The amount of memory requested from build containers"`
	ServiceCPURequest              string                       `toml:"service_cpu_request,omitempty" json:"service_cpu_request" long:"service-cpu-request" env:"KUBERNETES_SERVICE_CPU_REQUEST" description:"The CPU allocation requested for build service containers"`
	ServiceMemoryRequest           string                       `toml:"service_memory_request,omitempty" json:"service_memory_request" long:"service-memory-request" env:"KUBERNETES_SERVICE_MEMORY_REQUEST" description:"The amount of memory requested for build service containers"`
	HelperCPURequest               string                       `toml:"helper_cpu_request,omitempty" json:"helper_cpu_request" long:"helper-cpu-request" env:"KUBERNETES_HELPER_CPU_REQUEST" description:"The CPU allocation requested for build helper containers"`
	HelperMemoryRequest            string                       `toml:"helper_memory_request,omitempty" json:"helper_memory_request" long:"helper-memory-request" env:"KUBERNETES_HELPER_MEMORY_REQUEST" description:"The amount of memory requested for build helper containers"`
	PullPolicy                     KubernetesPullPolicy         `toml:"pull_policy,omitempty" json:"pull_policy" long:"pull-policy" env:"KUBERNETES_PULL_POLICY" description:"Policy for if/when to pull a container image (never, if-not-present, always). The cluster default will be used if not set"`
	NodeSelector                   map[string]string            `toml:"node_selector,omitempty" json:"node_selector" long:"node-selector" env:"KUBERNETES_NODE_SELECTOR" description:"A toml table/json object of key=value. Value is expected to be a string. When set this will create pods on k8s nodes that match all the key=value pairs."`
	NodeTolerations                map[string]string            `toml:"node_tolerations,omitempty" json:"node_tolerations" long:"node-tolerations" env:"KUBERNETES_NODE_TOLERATIONS" description:"A toml table/json object of key=value:effect. Value and effect are expected to be strings. When set, pods will tolerate the given taints. Only one toleration is supported through environment variable configuration."`
	ImagePullSecrets               []string                     `toml:"image_pull_secrets,omitempty" json:"image_pull_secrets" long:"image-pull-secrets" env:"KUBERNETES_IMAGE_PULL_SECRETS" description:"A list of image pull secrets that are used for pulling docker image"`
	HelperImage                    string                       `toml:"helper_image,omitempty" json:"helper_image" long:"helper-image" env:"KUBERNETES_HELPER_IMAGE" description:"[ADVANCED] Override the default helper image used to clone repos and upload artifacts"`
	TerminationGracePeriodSeconds  int64                        `toml:"terminationGracePeriodSeconds,omitzero" json:"terminationGracePeriodSeconds" long:"terminationGracePeriodSeconds" env:"KUBERNETES_TERMINATIONGRACEPERIODSECONDS" description:"Duration after the processes running in the pod are sent a termination signal and the time when the processes are forcibly halted with a kill signal."`
	PollInterval                   int                          `toml:"poll_interval,omitzero" json:"poll_interval" long:"poll-interval" env:"KUBERNETES_POLL_INTERVAL" description:"How frequently, in seconds, the runner will poll the Kubernetes pod it has just created to check its status"`
	PollTimeout                    int                          `toml:"poll_timeout,omitzero" json:"poll_timeout" long:"poll-timeout" env:"KUBERNETES_POLL_TIMEOUT" description:"The total amount of time, in seconds, that needs to pass before the runner will timeout attempting to connect to the pod it has just created (useful for queueing more builds that the cluster can handle at a time)"`
	PodLabels                      map[string]string            `toml:"pod_labels,omitempty" json:"pod_labels" long:"pod-labels" description:"A toml table/json object of key-value. Value is expected to be a string. When set, this will create pods with the given pod labels. Environment variables will be substituted for values here."`
	ServiceAccount                 string                       `toml:"service_account,omitempty" json:"service_account" long:"service-account" env:"KUBERNETES_SERVICE_ACCOUNT" description:"Executor pods will use this Service Account to talk to kubernetes API"`
	ServiceAccountOverwriteAllowed string                       `toml:"service_account_overwrite_allowed" json:"service_account_overwrite_allowed" long:"service_account_overwrite_allowed" env:"KUBERNETES_SERVICE_ACCOUNT_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_SERVICE_ACCOUNT' value"`
	PodAnnotations                 map[string]string            `toml:"pod_annotations,omitempty" json:"pod_annotations" long:"pod-annotations" description:"A toml table/json object of key-value. Value is expected to be a string. When set, this will create pods with the given annotations. Can be overwritten in build with KUBERNETES_POD_ANNOTATION_* varialbes"`
	PodAnnotationsOverwriteAllowed string                       `toml:"pod_annotations_overwrite_allowed" json:"pod_annotations_overwrite_allowed" long:"pod_annotations_overwrite_allowed" env:"KUBERNETES_POD_ANNOTATIONS_OVERWRITE_ALLOWED" description:"Regex to validate 'KUBERNETES_POD_ANNOTATIONS_*' values"`
	PodSecurityContext             KubernetesPodSecurityContext `toml:"pod_security_context,omitempty" long:"pod-security-context" env:"POD_SECURITY_CONTEXT" description:"A security context attached to each build pod"`
	Volumes                        KubernetesVolumes            `toml:"volumes"`
}

type KubernetesVolumes struct {
	HostPaths  []KubernetesHostPath  `toml:"host_path" description:"The host paths which will be mounted"`
	PVCs       []KubernetesPVC       `toml:"pvc" description:"The persistent volume claims that will be mounted"`
	ConfigMaps []KubernetesConfigMap `toml:"config_map" description:"The config maps which will be mounted as volumes"`
	Secrets    []KubernetesSecret    `toml:"secret" description:"The secret maps which will be mounted"`
	EmptyDirs  []KubernetesEmptyDir  `toml:"empty_dir" description:"The empty dirs which will be mounted"`
}

type KubernetesConfigMap struct {
	Name      string            `toml:"name" json:"name" description:"The name of the volume and ConfigMap to use"`
	MountPath string            `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	ReadOnly  bool              `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	Items     map[string]string `toml:"items,omitempty" description:"Key-to-path mapping for keys from the config map that should be used."`
}

type KubernetesHostPath struct {
	Name      string `toml:"name" json:"name" description:"The name of the volume"`
	MountPath string `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	ReadOnly  bool   `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	HostPath  string `toml:"host_path,omitempty" description:"Path from the host that should be mounted as a volume"`
}

type KubernetesPVC struct {
	Name      string `toml:"name" json:"name" description:"The name of the volume and PVC to use"`
	MountPath string `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	ReadOnly  bool   `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
}

type KubernetesSecret struct {
	Name      string            `toml:"name" json:"name" description:"The name of the volume and Secret to use"`
	MountPath string            `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	ReadOnly  bool              `toml:"read_only,omitempty" description:"If this volume should be mounted read only"`
	Items     map[string]string `toml:"items,omitempty" description:"Key-to-path mapping for keys from the secret that should be used."`
}

type KubernetesEmptyDir struct {
	Name      string `toml:"name" json:"name" description:"The name of the volume and EmptyDir to use"`
	MountPath string `toml:"mount_path" description:"Path where volume should be mounted inside of container"`
	Medium    string `toml:"medium,omitempty" description:"Set to 'Memory' to have a tmpfs"`
}

type KubernetesPodSecurityContext struct {
	FSGroup            int64   `toml:"fs_group,omitempty" long:"fs-group" env:"KUBERNETES_POD_SECURITY_CONTEXT_FS_GROUP" description:"A special supplemental group that applies to all containers in a pod"`
	RunAsGroup         int64   `toml:"run_as_group,omitempty" long:"run-as-group" env:"KUBERNETES_POD_SECURITY_CONTEXT_RUN_AS_GROUP" description:"The GID to run the entrypoint of the container process"`
	RunAsNonRoot       bool    `toml:"run_as_non_root,omitempty" long:"run-as-non-root" env:"KUBERNETES_POD_SECURITY_CONTEXT_RUN_AS_NON_ROOT" description:"Indicates that the container must run as a non-root user"`
	RunAsUser          int64   `toml:"run_as_user,omitempty" long:"run-as-user" env:"KUBERNETES_POD_SECURITY_CONTEXT_RUN_AS_USER" description:"The UID to run the entrypoint of the container process"`
	SupplementalGroups []int64 `toml:"supplemental_groups,omitempty" long:"supplemental-groups" description:"A list of groups applied to the first process run in each container, in addition to the container's primary GID"`
}

type RunnerCredentials struct {
	URL         string `toml:"url" json:"url" short:"u" long:"url" env:"CI_SERVER_URL" required:"true" description:"Runner URL"`
	Token       string `toml:"token" json:"token" short:"t" long:"token" env:"CI_SERVER_TOKEN" required:"true" description:"Runner token"`
	TLSCAFile   string `toml:"tls-ca-file,omitempty" json:"tls-ca-file" long:"tls-ca-file" env:"CI_SERVER_TLS_CA_FILE" description:"File containing the certificates to verify the peer when using HTTPS"`
	TLSCertFile string `toml:"tls-cert-file,omitempty" json:"tls-cert-file" long:"tls-cert-file" env:"CI_SERVER_TLS_CERT_FILE" description:"File containing certificate for TLS client auth when using HTTPS"`
	TLSKeyFile  string `toml:"tls-key-file,omitempty" json:"tls-key-file" long:"tls-key-file" env:"CI_SERVER_TLS_KEY_FILE" description:"File containing private key for TLS client auth when using HTTPS"`
}

type CacheGCSCredentials struct {
	AccessID   string `toml:"AccessID,omitempty" long:"access-id" env:"CACHE_GCS_ACCESS_ID" description:"ID of GCP Service Account used to access the storage"`
	PrivateKey string `toml:"PrivateKey,omitempty" long:"private-key" env:"CACHE_GCS_PRIVATE_KEY" description:"Private key used to sign GCS requests"`
}

type CacheGCSConfig struct {
	CacheGCSCredentials
	CredentialsFile string `toml:"CredentialsFile,omitempty" long:"credentials-file" env:"GOOGLE_APPLICATION_CREDENTIALS" description:"File with GCP credentials, containing AccessID and PrivateKey"`
	BucketName      string `toml:"BucketName,omitempty" long:"bucket-name" env:"CACHE_GCS_BUCKET_NAME" description:"Name of the bucket where cache will be stored"`
}

type CacheS3Config struct {
	ServerAddress  string `toml:"ServerAddress,omitempty" long:"server-address" env:"CACHE_S3_SERVER_ADDRESS" description:"A host:port to the used S3-compatible server"`
	AccessKey      string `toml:"AccessKey,omitempty" long:"access-key" env:"CACHE_S3_ACCESS_KEY" description:"S3 Access Key"`
	SecretKey      string `toml:"SecretKey,omitempty" long:"secret-key" env:"CACHE_S3_SECRET_KEY" description:"S3 Secret Key"`
	BucketName     string `toml:"BucketName,omitempty" long:"bucket-name" env:"CACHE_S3_BUCKET_NAME" description:"Name of the bucket where cache will be stored"`
	BucketLocation string `toml:"BucketLocation,omitempty" long:"bucket-location" env:"CACHE_S3_BUCKET_LOCATION" description:"Name of S3 region"`
	Insecure       bool   `toml:"Insecure,omitempty" long:"insecure" env:"CACHE_S3_INSECURE" description:"Use insecure mode (without https)"`
}

type CacheConfig struct {
	Type   string `toml:"Type,omitempty" long:"type" env:"CACHE_TYPE" description:"Select caching method"`
	Path   string `toml:"Path,omitempty" long:"path" env:"CACHE_PATH" description:"Name of the path to prepend to the cache URL"`
	Shared bool   `toml:"Shared,omitempty" long:"shared" env:"CACHE_SHARED" description:"Enable cache sharing between runners."`

	S3  *CacheS3Config  `toml:"s3,omitempty" json:"s3" namespace:"s3"`
	GCS *CacheGCSConfig `toml:"gcs,omitempty" json:"gcs" namespace:"gcs"`

	// TODO: Remove in 12.0
	S3CachePath    string `toml:"-" long:"s3-cache-path" env:"S3_CACHE_PATH" description:"Name of the path to prepend to the cache URL. DEPRECATED"` // DEPRECATED
	CacheShared    bool   `toml:"-" long:"cache-shared" description:"Enable cache sharing between runners. DEPRECATED"`                              // DEPRECATED
	ServerAddress  string `toml:"ServerAddress,omitempty" description:"A host:port to the used S3-compatible server DEPRECATED"`                     // DEPRECATED
	AccessKey      string `toml:"AccessKey,omitempty" description:"S3 Access Key DEPRECATED"`                                                        // DEPRECATED
	SecretKey      string `toml:"SecretKey,omitempty" description:"S3 Secret Key DEPRECATED"`                                                        // DEPRECATED
	BucketName     string `toml:"BucketName,omitempty" description:"Name of the bucket where cache will be stored DEPRECATED"`                       // DEPRECATED
	BucketLocation string `toml:"BucketLocation,omitempty" description:"Name of S3 region DEPRECATED"`                                               // DEPRECATED
	Insecure       bool   `toml:"Insecure,omitempty" description:"Use insecure mode (without https) DEPRECATED"`                                     // DEPRECATED
}

type RunnerSettings struct {
	Executor  string `toml:"executor" json:"executor" long:"executor" env:"RUNNER_EXECUTOR" required:"true" description:"Select executor, eg. shell, docker, etc."`
	BuildsDir string `toml:"builds_dir,omitempty" json:"builds_dir" long:"builds-dir" env:"RUNNER_BUILDS_DIR" description:"Directory where builds are stored"`
	CacheDir  string `toml:"cache_dir,omitempty" json:"cache_dir" long:"cache-dir" env:"RUNNER_CACHE_DIR" description:"Directory where build cache is stored"`
	CloneURL  string `toml:"clone_url,omitempty" json:"clone_url" long:"clone-url" env:"CLONE_URL" description:"Overwrite the default URL used to clone or fetch the git ref"`

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
	Name               string `toml:"name" json:"name" short:"name" long:"description" env:"RUNNER_NAME" description:"Runner name"`
	Limit              int    `toml:"limit,omitzero" json:"limit" long:"limit" env:"RUNNER_LIMIT" description:"Maximum number of builds processed by this runner"`
	OutputLimit        int    `toml:"output_limit,omitzero" long:"output-limit" env:"RUNNER_OUTPUT_LIMIT" description:"Maximum build trace size in kilobytes"`
	RequestConcurrency int    `toml:"request_concurrency,omitzero" long:"request-concurrency" env:"RUNNER_REQUEST_CONCURRENCY" description:"Maximum concurrency for job requests"`

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

	// TODO: Remove in 12.0
	MetricsServerAddress string `toml:"metrics_server,omitempty" json:"metrics_server"` // DEPRECATED

	Concurrent    int             `toml:"concurrent" json:"concurrent"`
	CheckInterval int             `toml:"check_interval" json:"check_interval" description:"Define active checking interval of jobs"`
	LogLevel      *string         `toml:"log_level" json:"log_level" description:"Define log level (one of: panic, fatal, error, warning, info, debug)"`
	LogFormat     *string         `toml:"log_format" json:"log_format" description:"Define log format (one of: runner, text, json)"`
	User          string          `toml:"user,omitempty" json:"user"`
	Runners       []*RunnerConfig `toml:"runners" json:"runners"`
	SentryDSN     *string         `toml:"sentry_dsn"`
	ModTime       time.Time       `toml:"-"`
	Loaded        bool            `toml:"-"`
}

func getDeprecatedStringSetting(setting string, tomlField string, envVariable string, tomlReplacement string, envReplacement string) string {
	if setting != "" {
		logrus.Warningf("%s setting is deprecated and will be removed in GitLab Runner 12.0. Please use %s instead", tomlField, tomlReplacement)
		return setting
	}

	value := os.Getenv(envVariable)
	if value != "" {
		logrus.Warningf("%s environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use %s instead", envVariable, envReplacement)
	}

	return value
}

func getDeprecatedBoolSetting(setting bool, tomlField string, envVariable string, tomlReplacement string, envReplacement string) bool {
	if setting {
		logrus.Warningf("%s setting is deprecated and will be removed in GitLab Runner 12.0. Please use %s instead", tomlField, tomlReplacement)
		return setting
	}

	value, _ := strconv.ParseBool(os.Getenv(envVariable))
	if value {
		logrus.Warningf("%s environment variables is deprecated and will be removed in GitLab Runner 12.0. Please use %s instead", envVariable, envReplacement)
	}

	return value
}

func (c *CacheS3Config) ShouldUseIAMCredentials() bool {
	return c.ServerAddress == "" || c.AccessKey == "" || c.SecretKey == ""
}

func (c *CacheConfig) GetPath() string {
	if c.Path != "" {
		return c.Path
	}

	// TODO: Remove in 12.0
	if c.S3CachePath != "" {
		logrus.Warning("'--cache-s3-cache-path' command line option and `$S3_CACHE_PATH` environment variables are deprecated and will be removed in GitLab Runner 12.0. Please use '--cache-path' or '$CACHE_PATH' instead")
	}

	return c.S3CachePath
}

func (c *CacheConfig) GetShared() bool {
	if c.Shared {
		return c.Shared
	}

	// TODO: Remove in 12.0
	if c.CacheShared {
		logrus.Warning("'--cache-cache-shared' command line is deprecated and will be removed in GitLab Runner 12.0. Please use '--cache-shared' instead")
	}

	return c.CacheShared
}

// DEPRECATED
// TODO: Remove in 12.0
func (c *CacheConfig) GetServerAddress() string {
	return getDeprecatedStringSetting(
		c.ServerAddress,
		"[runners.cache] ServerAddress",
		"S3_SERVER_ADDRESS",
		"[runners.cache.s3] ServerAddress",
		"CACHE_S3_SERVER_ADDRESS")
}

// DEPRECATED
// TODO: Remove in 12.0
func (c *CacheConfig) GetAccessKey() string {
	return getDeprecatedStringSetting(
		c.AccessKey,
		"[runners.cache] AccessKey",
		"S3_ACCESS_KEY",
		"[runners.cache.s3] AccessKey",
		"CACHE_S3_ACCESS_KEY")
}

// DEPRECATED
// TODO: Remove in 12.0
func (c *CacheConfig) GetSecretKey() string {
	return getDeprecatedStringSetting(
		c.SecretKey,
		"[runners.cache] SecretKey",
		"S3_SECRET_KEY",
		"[runners.cache.s3] SecretKey",
		"CACHE_S3_SECRET_KEY")
}

// DEPRECATED
// TODO: Remove in 12.0
func (c *CacheConfig) GetBucketName() string {
	return getDeprecatedStringSetting(
		c.BucketName,
		"[runners.cache] BucketName",
		"S3_BUCKET_NAME",
		"[runners.cache.s3] BucketName",
		"CACHE_S3_BUCKET_NAME")
}

// DEPRECATED
// TODO: Remove in 12.0
func (c *CacheConfig) GetBucketLocation() string {
	return getDeprecatedStringSetting(
		c.BucketLocation,
		"[runners.cache] BucketLocation",
		"S3_BUCKET_LOCATION",
		"[runners.cache.s3] BucketLocation",
		"CACHE_S3_BUCKET_LOCATION")
}

// DEPRECATED
// TODO: Remove in 12.0
func (c *CacheConfig) GetInsecure() bool {
	return getDeprecatedBoolSetting(
		c.Insecure,
		"[runners.cache] Insecure",
		"S3_CACHE_INSECURE",
		"[runners.cache.s3] Insecure",
		"CACHE_S3_INSECURE")
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

func (c *RunnerConfig) GetRequestConcurrency() int {
	if c.RequestConcurrency <= 0 {
		return 1
	}
	return c.RequestConcurrency
}

func (c *RunnerConfig) GetVariables() JobVariables {
	var variables JobVariables

	for _, environment := range c.Environment {
		if variable, err := ParseVariable(environment); err == nil {
			variable.Internal = true
			variables = append(variables, variable)
		}
	}

	return variables
}

// DeepCopy attempts to make a deep clone of the object
func (c *RunnerConfig) DeepCopy() (*RunnerConfig, error) {
	var r RunnerConfig

	bytes, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("serialization of runner config failed: %v", err)
	}

	err = json.Unmarshal(bytes, &r)
	if err != nil {
		return nil, fmt.Errorf("deserialization of runner config failed: %v", err)
	}

	return &r, err
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
		logrus.Fatalf("Error encoding TOML: %s", err)
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

func (c *Config) ListenOrServerMetricAddress() string {
	if c.ListenAddress != "" {
		return c.ListenAddress
	}

	// TODO: Remove in 12.0
	if c.MetricsServerAddress != "" {
		logrus.Warnln("'metrics_server' configuration entry is deprecated and will be removed in one of future releases; please use 'listen_address' instead")
	}

	return c.MetricsServerAddress
}

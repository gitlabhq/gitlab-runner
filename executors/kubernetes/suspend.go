package kubernetes

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ common.SuspendableExecutor = (*executor)(nil)

const (
	envKeyPVC       = "pvc"
	envKeyNamespace = "namespace"

	suspendPVCMountPath = "/gl_persist"
	suspendPVCVolName   = "gl-runner-suspend-pvc"
	suspendCMVolName    = "gl-runner-overlay-init"
	overlayScriptDir    = "/runner-overlay-init"
	overlayScriptPath   = overlayScriptDir + "/entrypoint.sh"
	overlayScriptKey    = "entrypoint.sh"

	suspendPVCDefaultSize = "20Gi"

	// capSysAdmin is the Linux capability the overlay entrypoint needs to run
	// mount and pivot_root. Granted implicitly by privileged mode.
	capSysAdmin api.Capability = "SYS_ADMIN"
)

// generateSuspendPVCName returns a unique PVC name using 12 hex chars
// from a UUID v4 (positions 0-7 and 9-12, skipping the first hyphen).
// 16^12 ≈ 281 trillion combinations — collision probability is negligible
// even under high concurrency within a single namespace.
func generateSuspendPVCName() string {
	id := uuid.NewString()
	return "gl-runner-env-" + id[:8] + id[9:13]
}

func (s *executor) overlayConfigMapName() string {
	return "gl-runner-overlay-" + strconv.FormatInt(s.Build.ID, 10)
}

// isHostUserEnabled returns false to isolate the linux user namespace of the pod from that of the host.
// Read more https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
func (s *executor) isHostUserEnabled() *bool {
	if !s.usesSuspendResume() {
		return nil
	}
	return new(false)
}

// injectSuspendCap ensures CAP_SYS_ADMIN is present in c's security context
// capabilities. The overlay entrypoint needs it for mount and pivot_root;
// rather than requiring users to configure it, the runner injects it
// automatically for every container that runs the overlay script.
// It is idempotent: if the capability is already present (via privileged mode
// or explicit config) the capabilities slice is left unchanged.
func injectSuspendCap(c *api.Container) {
	if c.SecurityContext == nil {
		c.SecurityContext = &api.SecurityContext{}
	}
	if c.SecurityContext.Capabilities == nil {
		c.SecurityContext.Capabilities = &api.Capabilities{}
	}
	if !slices.Contains(c.SecurityContext.Capabilities.Add, capSysAdmin) {
		c.SecurityContext.Capabilities.Add = append(c.SecurityContext.Capabilities.Add, capSysAdmin)
	}
}

// guardSuspendCompatibility runs every suspend/resume compatibility check
// before any Kubernetes resources are created. It is a no-op for jobs that
// aren't suspendable.
func (s *executor) guardSuspendCompatibility() error {
	if !s.usesSuspendResume() {
		return nil
	}

	if s.Build.Runner.ID <= 0 {
		return errors.New(
			"suspend/resume requires a positive runner ID, but this runner's config.toml has " +
				"no id set (or id = 0): the environment key embeds the runner ID so a resumed " +
				"job can be routed back to the runner that suspended it, and a zero ID is " +
				"rejected when parsing that key. Run `gitlab-runner register` to register this " +
				"runner and populate id in config.toml, or add the id GitLab assigned to this " +
				"runner (visible on the runner's details page) to the [[runners]] section " +
				"manually if it's already registered")
	}

	if s.Build.IsFeatureFlagOn(featureflags.UseConcrete) {
		return errors.New(
			"suspend/resume is not yet supported with FF_CONCRETE: the Concrete pod builder " +
				"does not create the suspend PVC or mount the overlay, so the environment is " +
				"never persisted and a resume silently starts from a clean container. " +
				"Disable FF_CONCRETE to use suspendable environments")
	}

	if s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		return errors.New(
			"suspend/resume requires the attach execution strategy and cannot run with " +
				"FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY enabled: the overlay build " +
				"container relies on attach, and the suspend volumes are not added in the " +
				"legacy code path. Disable that feature flag to use suspendable environments")
	}

	if s.Build.IsFeatureFlagOn(featureflags.KubernetesHonorEntrypoint) {
		for name, svc := range s.options.Services {
			if len(svc.Entrypoint) > 0 || len(svc.Command) > 0 {
				return fmt.Errorf(
					"suspend/resume is not compatible with FF_KUBERNETES_HONOR_ENTRYPOINT when "+
						"a service (%q) declares an explicit entrypoint or command: the flag changes "+
						"how the overlay entrypoint is threaded through the service container, "+
						"causing the service to be unreachable after pivot_root. "+
						"Disable FF_KUBERNETES_HONOR_ENTRYPOINT to use suspendable environments "+
						"with overlay-wrapped service containers", name)
			}
		}
	}

	if err := s.guardSuspendCapabilityDrop(); err != nil {
		return err
	}

	if err := s.guardSuspendSecurityContext(); err != nil {
		return err
	}

	return s.guardSuspendReservedMountPaths()
}

// guardSuspendCapabilityDrop rejects the job if cap_drop would undo the
// SYS_ADMIN capability injectSuspendCap adds
func (s *executor) guardSuspendCapabilityDrop() error {
	if !s.usesSuspendResume() {
		return nil
	}

	sc := s.Config.Kubernetes.GetContainerSecurityContext(
		s.Config.Kubernetes.BuildContainerSecurityContext, s.defaultCapDrop()...)
	if c := droppedSysAdmin(sc); c != "" {
		return fmt.Errorf(
			"suspend/resume requires CAP_SYS_ADMIN for the overlay entrypoint's mount/pivot_root "+
				"calls, but the build container's cap_drop includes %q, which strips it back out "+
				"after it's injected. Remove %q from cap_drop to use suspendable environments", c, c)
	}

	for name, svc := range s.options.Services {
		if len(svc.Entrypoint) == 0 && len(svc.Command) == 0 {
			continue
		}
		svcSC := s.Config.Kubernetes.GetContainerSecurityContext(
			s.Config.Kubernetes.ServiceContainerSecurityContext, s.defaultCapDrop()...)
		if c := droppedSysAdmin(svcSC); c != "" {
			return fmt.Errorf(
				"suspend/resume requires CAP_SYS_ADMIN for the overlay entrypoint's mount/pivot_root "+
					"calls, but service container %q's cap_drop includes %q, which strips it back out "+
					"after it's injected. Remove %q from cap_drop to use suspendable environments", name, c, c)
		}
	}

	return nil
}

// droppedSysAdmin returns the Drop entry that would strip the CAP_SYS_ADMIN
// injectSuspendCap adds, or "" if sc has none. A privileged container gets
// every capability regardless of Drop, so it never conflicts.
func droppedSysAdmin(sc *api.SecurityContext) string {
	if sc.Privileged != nil && *sc.Privileged {
		return ""
	}
	if sc.Capabilities == nil {
		return ""
	}
	for _, c := range sc.Capabilities.Drop {
		if strings.EqualFold(string(c), string(capSysAdmin)) || strings.EqualFold(string(c), "ALL") {
			return string(c)
		}
	}
	return ""
}

// guardSuspendSecurityContext checks that the build container's security
// context is compatible with the overlay entrypoint.
func (s *executor) guardSuspendSecurityContext() error {
	if !s.usesSuspendResume() {
		return nil
	}

	sc := s.Config.Kubernetes.GetContainerSecurityContext(
		s.Config.Kubernetes.BuildContainerSecurityContext, s.defaultCapDrop()...)
	if s.runAsNonRoot(sc) {
		return errors.New(
			"suspend/resume requires the build container to run as root: the overlay " +
				"entrypoint performs mount and pivot_root, which cannot run as a non-root user. " +
				"run_as_non_root is set to true; unset it (or set it to false) on the build " +
				"container or pod security context to use suspendable environments")
	}

	kubernetesOptions := s.options.Image.ExecutorOptions.Kubernetes.Expand(s.Build.GetAllVariables())
	uid, _ := s.getContainerUIDGID(
		string(kubernetesOptions.User), buildContainerName, s.Config.Kubernetes.BuildContainerSecurityContext)
	if uid > 0 {
		return fmt.Errorf(
			"suspend/resume requires the build container to run as root: the overlay entrypoint "+
				"performs mount and pivot_root, which cannot run as a non-root user. the effective "+
				"run_as_user resolves to %d (via build_container_security_context, pod_security_context, "+
				"or the job's image.kubernetes.user); unset it or set it to 0 to use suspendable environments",
			uid)
	}

	// Service containers with a custom entrypoint or command are also wrapped
	// with the overlay script and must run as root for the same reason.
	for name, svc := range s.options.Services {
		if len(svc.Entrypoint) == 0 && len(svc.Command) == 0 {
			continue
		}
		svcSC := s.Config.Kubernetes.GetContainerSecurityContext(
			s.Config.Kubernetes.ServiceContainerSecurityContext, s.defaultCapDrop()...)
		if s.runAsNonRoot(svcSC) {
			return fmt.Errorf(
				"suspend/resume requires service container %q to run as root: the overlay "+
					"entrypoint performs mount and pivot_root, which cannot run as a non-root user. "+
					"run_as_non_root is set to true; unset it on the service container or pod "+
					"security context to use suspendable environments",
				name)
		}
	}

	return nil
}

func (s *executor) runAsNonRoot(sc *api.SecurityContext) bool {
	if sc.RunAsNonRoot != nil {
		return *sc.RunAsNonRoot
	}
	if pod := s.Config.Kubernetes.GetPodSecurityContext(); pod != nil && pod.RunAsNonRoot != nil {
		return *pod.RunAsNonRoot
	}
	return false
}

// guardSuspendReservedMountPaths rejects configurations where a mount path the
// suspend overlay reserves is claimed by something else. The overlay mounts the
// suspend PVC at /gl_persist, the init-script ConfigMap at /runner-overlay-init,
// and the PVC builds subPath at the builds dir (RootDir()). A user volume at any
// of these — or a builds_dir set to one of the other two reserved paths —
// produces a duplicate mount that Kubernetes rejects at pod creation with an
// opaque error. Failing early with a precise message is clearer, and silently
// skipping the user volume would be its own silent-data-loss class.
//
// Mount paths from getVolumeMountsForConfig() are already ExpandValue-expanded,
// so the comparison is against the final resolved paths.
func (s *executor) guardSuspendReservedMountPaths() error {
	if !s.usesSuspendResume() {
		return nil
	}

	buildsDir := s.RootDir()

	// Self-collision: the builds dir must differ from the other two reserved
	// paths, otherwise the suspend feature itself mounts two volumes at one
	// path. Checked separately because it would otherwise collapse two keys of
	// the reserved map into one and be silently dropped.
	if buildsDir == suspendPVCMountPath || buildsDir == overlayScriptDir {
		return fmt.Errorf(
			"suspend/resume reserves %q and %q for its overlay mounts, but builds_dir resolves to %q; "+
				"set builds_dir to a different path to use suspendable environments",
			suspendPVCMountPath, overlayScriptDir, buildsDir)
	}

	reserved := map[string]string{
		buildsDir:           "builds_dir",
		suspendPVCMountPath: "the suspend overlay (" + suspendPVCMountPath + ")",
		overlayScriptDir:    "the suspend overlay init script (" + overlayScriptDir + ")",
	}

	for _, m := range s.getVolumeMountsForConfig() {
		if what, clash := reserved[m.MountPath]; clash {
			return fmt.Errorf(
				"suspend/resume reserves the mount path %q for %s; a configured volume (%q) mounts "+
					"there and would be rejected by Kubernetes as a duplicate. Change the volume's "+
					"mount_path (or builds_dir) to a non-reserved path to use suspendable environments",
				m.MountPath, what, m.Name)
		}
	}

	return nil
}

// suspendRebindPaths lists every path overlay_entrypoint.sh must rebind
// before pivot_root — extra (builds/scripts/logs, empty for services) plus
// every configured volume mount, excluding the two paths steps 1/6 already
// bind. See CI_SUSPEND_REBIND_PATHS in overlay_entrypoint.sh for why.
func (s *executor) suspendRebindPaths(extra ...string) string {
	mounts := s.getVolumeMountsForConfig()
	seen := make(map[string]bool, len(extra)+len(mounts))
	paths := make([]string, 0, len(extra)+len(mounts))

	add := func(p string) {
		// A newline would smuggle an extra path past the newline-joined list below.
		if p == "" || strings.ContainsRune(p, '\n') {
			return
		}
		p = path.Clean(p)
		if p == suspendPVCMountPath || p == overlayScriptDir || seen[p] {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}

	for _, p := range extra {
		add(p)
	}
	for _, m := range mounts {
		add(m.MountPath)
	}

	sort.Strings(paths) // parent before child, so a later bind can't shadow an earlier one

	return strings.Join(paths, "\n")
}

func (s *executor) usesSuspendResume() bool {
	if s.Build == nil {
		return false
	}
	if !s.Build.IsFeatureFlagOn(featureflags.SuspendableEnvironments) {
		return false
	}
	opts := s.Build.Job.SuspendOptions
	return opts.SuspendOnSuccess || opts.SuspendOnFailure || opts.RuntimeEnvironmentKey != ""
}

func (s *executor) parseSuspendPVCSize() (resource.Quantity, error) {
	size := s.Config.Kubernetes.SuspendPVCSize
	if size == "" {
		size = suspendPVCDefaultSize
	}
	qty, err := resource.ParseQuantity(size)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("parse suspend PVC size %q: %w", size, err)
	}
	return qty, nil
}

func (s *executor) ensureSuspendResources(ctx context.Context) error {
	if !s.usesSuspendResume() {
		return nil
	}
	if !s.isResume {
		if err := s.ensureSuspendRootfsPVC(ctx); err != nil {
			return fmt.Errorf("creating suspend PVC: %w", err)
		}
	}
	if err := s.createOverlayConfigMap(ctx); err != nil {
		return fmt.Errorf("creating overlay ConfigMap: %w", err)
	}
	return nil
}

func (s *executor) ensureSuspendRootfsPVC(ctx context.Context) error {
	if s.suspendRootfsPVCName != "" {
		return nil
	}

	qty, err := s.parseSuspendPVCSize()
	if err != nil {
		return err
	}

	name := generateSuspendPVCName()
	pvc := s.buildSuspendPVC(name, qty)

	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: persistentvolumeclaims, create, FF_SUSPENDABLE_ENVIRONMENTS=true
	_, err = s.kubeClient.CoreV1().
		PersistentVolumeClaims(s.configurationOverwrites.namespace).
		Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create rootfs suspend PVC %q: %w", name, err)
	}

	s.suspendRootfsPVCName = name
	return nil
}

// buildSuspendPVC constructs a PVC object with the common suspend settings.
func (s *executor) buildSuspendPVC(name string, storageQty resource.Quantity) *api.PersistentVolumeClaim {
	pvc := &api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: api.PersistentVolumeClaimSpec{
			AccessModes: []api.PersistentVolumeAccessMode{api.ReadWriteOnce},
			Resources: api.VolumeResourceRequirements{
				Requests: api.ResourceList{
					api.ResourceStorage: storageQty,
				},
			},
		},
	}

	if sc := s.Config.Kubernetes.SuspendPVCStorageClass; sc != "" {
		pvc.Spec.StorageClassName = &sc
	}

	return pvc
}

func (s *executor) deleteSuspendRootfsPVC(ctx context.Context) {
	if s.suspendRootfsPVCName == "" {
		return
	}
	if s.suspended {
		return
	}
	// For a resume job, retain the PVC if no pod was created — Prepare failed
	// before the job ran, so the user can retry with the same environment.
	if s.isResume && s.pod == nil {
		return
	}
	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: persistentvolumeclaims, delete, FF_SUSPENDABLE_ENVIRONMENTS=true
	err := s.kubeClient.CoreV1().PersistentVolumeClaims(s.configurationOverwrites.namespace).
		Delete(ctx, s.suspendRootfsPVCName, metav1.DeleteOptions{})
	if err != nil && !kubeerrors.IsNotFound(err) {
		s.BuildLogger.Warningln(fmt.Sprintf("failed to delete rootfs suspend PVC %q: %v", s.suspendRootfsPVCName, err))
	}
}

//go:embed overlay_entrypoint.sh
var overlayEntrypointScript string

// createOverlayConfigMap creates the ConfigMap that carries the overlay
// entrypoint script. The ConfigMap is named s.overlayConfigMapName() and is
// created in s.configurationOverwrites.namespace.
//
// No OwnerReference is set: the ConfigMap is created before the pod exists so
// there is no owner to reference; Cleanup deletes it explicitly for all
// suspendable jobs.
func (s *executor) createOverlayConfigMap(ctx context.Context) error {
	cm := &api.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.overlayConfigMapName(),
		},
		Data: map[string]string{
			overlayScriptKey: overlayEntrypointScript,
		},
	}

	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: configmaps, create, FF_SUSPENDABLE_ENVIRONMENTS=true
	_, err := s.kubeClient.CoreV1().ConfigMaps(s.configurationOverwrites.namespace).
		Create(ctx, cm, metav1.CreateOptions{})
	if kubeerrors.IsAlreadyExists(err) {
		//nolint:gocritic // kubeAPI annotation, not commented-out code
		// kubeAPI: configmaps, get, FF_SUSPENDABLE_ENVIRONMENTS=true
		existing, getErr := s.kubeClient.CoreV1().ConfigMaps(s.configurationOverwrites.namespace).
			Get(ctx, cm.Name, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("get existing overlay ConfigMap %q: %w", cm.Name, getErr)
		}
		cm.ResourceVersion = existing.ResourceVersion

		//nolint:gocritic // kubeAPI annotation, not commented-out code
		// kubeAPI: configmaps, update, FF_SUSPENDABLE_ENVIRONMENTS=true
		_, err = s.kubeClient.CoreV1().ConfigMaps(s.configurationOverwrites.namespace).
			Update(ctx, cm, metav1.UpdateOptions{})
	}
	if err != nil {
		return fmt.Errorf("create overlay ConfigMap %q: %w", cm.Name, err)
	}

	return nil
}

func (s *executor) deleteOverlayConfigMap(ctx context.Context) {
	if s.Build == nil || s.kubeClient == nil || s.configurationOverwrites == nil {
		return
	}
	name := s.overlayConfigMapName()
	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: configmaps, delete, FF_SUSPENDABLE_ENVIRONMENTS=true
	err := s.kubeClient.CoreV1().ConfigMaps(s.configurationOverwrites.namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !kubeerrors.IsNotFound(err) {
		s.BuildLogger.Warningln(fmt.Sprintf("failed to delete overlay ConfigMap %q: %v", name, err))
	}
}

// suspendVolumes returns the 2 volumes required by a suspendable pod:
//  1. Rootfs PVC volume — mounts the single persistent PVC (used as overlayfs upper
//     layer at /gl_persist, and via subPath at /builds).
//  2. ConfigMap volume  — carries the overlay entrypoint script (mode 0755).
func (s *executor) suspendVolumes() []api.Volume {
	cmMode := int32(0o755)
	return []api.Volume{
		{
			Name: suspendPVCVolName,
			VolumeSource: api.VolumeSource{
				PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{
					ClaimName: s.suspendRootfsPVCName,
				},
			},
		},
		{
			Name: suspendCMVolName,
			VolumeSource: api.VolumeSource{
				ConfigMap: &api.ConfigMapVolumeSource{
					LocalObjectReference: api.LocalObjectReference{
						Name: s.overlayConfigMapName(),
					},
					DefaultMode: &cmMode,
				},
			},
		},
	}
}

// Suspend marks the job environment as suspended and returns the fields needed
// for a future resume job to restore the workload state. Pod deletion happens
// in Cleanup() alongside all other resource cleanup.
func (s *executor) Suspend(_ context.Context) (url.Values, error) {
	if s.pod == nil {
		return nil, errors.New("suspend: no pod")
	}
	if s.suspendRootfsPVCName == "" {
		return nil, errors.New("suspend: no rootfs PVC was created")
	}

	s.BuildLogger.Infoln("Suspending job environment on PVC", s.suspendRootfsPVCName)
	s.suspended = true

	fields := url.Values{
		envKeyPVC:       {s.suspendRootfsPVCName},
		envKeyNamespace: {s.pod.Namespace},
	}
	return fields, nil
}

// Resume restores workload state from fields produced by a prior Suspend() call.
// SECURITY: callers must validate RunnerID and SystemID from the full RuntimeEnvironmentKey
// before calling this method. Resume() only validates executor-specific fields
// (namespace, PVC name). Identity validation is the caller's responsibility.
func (s *executor) Resume(ctx context.Context, fields url.Values) error {
	if s.configurationOverwrites == nil {
		return errors.New("resume: executor not initialized")
	}
	ns := fields.Get(envKeyNamespace)
	if ns == "" {
		return errors.New("resume: environment key missing 'namespace' field")
	}
	pvc := fields.Get(envKeyPVC)
	if pvc == "" {
		return errors.New("resume: environment key missing 'pvc' field")
	}

	// The suspended environment's PVC lives in this namespace, so resume
	// cannot proceed without it. This check applies unconditionally — not
	// only when kubernetes.NamespacePerJob manages the namespace's lifecycle
	// — because the PVC's existence is tied to the namespace regardless of
	// who created it or whether the runner owns its teardown. Failing here
	// gives a precise error instead of an opaque failure later during pod
	// creation.
	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: namespaces, get, FF_SUSPENDABLE_ENVIRONMENTS=true
	if _, err := s.kubeClient.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); err != nil {
		if kubeerrors.IsNotFound(err) {
			return fmt.Errorf("resume: namespace %q no longer exists; the suspended environment cannot be resumed", ns)
		}
		return fmt.Errorf("resume: checking namespace %q exists: %w", ns, err)
	}

	// A resume pod cannot honor a configured namespace override: the
	// suspended environment's PVC is bound to the namespace it was created
	// in, so resume must go there regardless of namespace_overwrite_allowed,
	// KUBERNETES_NAMESPACE_OVERWRITE, or NamespacePerJob. Warn instead of
	// silently discarding whatever checkDefaults() had already resolved.
	if s.configurationOverwrites.namespace != "" && s.configurationOverwrites.namespace != ns {
		s.BuildLogger.Warningln(fmt.Sprintf(
			"resume: ignoring configured namespace %q; using %q, where the suspended environment's PVC lives",
			s.configurationOverwrites.namespace, ns))
	}

	s.configurationOverwrites.namespace = ns
	s.suspendRootfsPVCName = pvc
	s.isResume = true
	return nil
}

// prepareResume is called from Prepare() when the build carries an RuntimeEnvironmentKey.
// It parses the key, validates runner and system identity, and delegates state
// restoration to Resume(). It is a no-op for first-run jobs.
func (s *executor) prepareResume(options common.ExecutorPrepareOptions) error {
	raw := options.Build.RuntimeEnvironmentKey()
	if raw == "" {
		return nil
	}

	envKey, err := common.ParseRuntimeEnvironmentKey(raw)
	if err != nil {
		return fmt.Errorf("prepareResume: parse environment key: %w", err)
	}

	if envKey.RunnerID != s.Build.Runner.ID {
		return fmt.Errorf("prepareResume: environment key runner ID %d does not match this runner ID %d",
			envKey.RunnerID, s.Build.Runner.ID)
	}
	if envKey.SystemID != s.Build.Runner.GetSystemID() {
		return fmt.Errorf("prepareResume: environment key system ID %q does not match this system ID %q",
			envKey.SystemID, s.Build.Runner.GetSystemID())
	}

	return s.Resume(options.Context, envKey.Fields)
}

func serviceSuspendSubPath(serviceName string) string {
	return "services/" + serviceName
}

// overlayServiceContainer wraps a service container with the overlay entrypoint,
// mirroring what createBuildAndHelperContainers does for the build container.
// It is only called when the service declares an explicit entrypoint or command
// in the job spec — those become the args passed to exec "$@" after pivot_root.
// Each service gets its own PVC subPath (services/<name>/overlay) so its
// upper/work/merged dirs are isolated from the build container and other
// services — and from that same service's own data mount (see
// suspendServiceDataMount), which uses the sibling path services/<name>/data.
func (s *executor) overlayServiceContainer(c *api.Container, name string, svc *spec.Image) {
	// Combine whatever getCommandAndArgs resolved into a single args slice so
	// the overlay script can exec the service's real process after pivot_root.
	origArgs := append(append([]string{}, c.Command...), c.Args...)

	c.Command = []string{"/bin/sh", overlayScriptPath}
	c.Args = origArgs

	c.VolumeMounts = append(c.VolumeMounts,
		// Per-service subPath so upper/work/merged don't collide with the build
		// container or other services on the same PVC.
		api.VolumeMount{
			Name:      suspendPVCVolName,
			MountPath: suspendPVCMountPath,
			SubPath:   serviceSuspendSubPath(name) + "/overlay",
		},
		api.VolumeMount{Name: suspendCMVolName, MountPath: overlayScriptDir},
	)

	injectSuspendCap(c)

	vars := []spec.Variable{
		{Key: "CI_SUSPEND_IMAGE_TAG", Value: svc.Name},
		{Key: "CI_SUSPEND_PERSIST_DIR", Value: suspendPVCMountPath},
		{Key: "CI_SUSPEND_REBIND_PATHS", Value: s.suspendRebindPaths()},
	}
	c.Env = append(c.Env, buildVariables(vars)...)
}

// suspendServiceDataMount returns a VolumeMount that persists a service
// container's data directory across suspend/resume cycles. It reuses the
// existing suspendPVCVolName volume (already declared by suspendVolumes when
// usesSuspendResume is true) — no new volume is required.
//
// SubPath is services/<name>/data — a sibling of the overlay mount's
// services/<name>/overlay (see overlayServiceContainer), not a parent of it.
//
// Only SubPath is set, never SubPathExpr: Kubernetes rejects a VolumeMount
// that sets both, since they are mutually exclusive.
func suspendServiceDataMount(serviceName, dataPath string) api.VolumeMount {
	return api.VolumeMount{
		Name:      suspendPVCVolName,
		MountPath: dataPath,
		SubPath:   serviceSuspendSubPath(serviceName) + "/data",
	}
}

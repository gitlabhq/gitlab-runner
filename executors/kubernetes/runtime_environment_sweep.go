package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var _ common.RuntimeEnvironmentSweeper = executorProvider{}

// runtimeEnvironmentPVCLabel ties an overlay ConfigMap to the PVC of the
// environment it belongs to, so releaseRuntimeEnvironment can remove it
// without the job.
const runtimeEnvironmentPVCLabel = common.RuntimeEnvironmentLabelPrefix + "pvc"

// SweepRuntimeEnvironments removes this runner's suspended environments whose
// PVC is sweepable at now. The PVC is created while the environment's first
// job is prepared and survives every suspend and resume (only the pod is
// recreated), so the expiry stamped on it then is the environment's expiry.
func (p executorProvider) SweepRuntimeEnvironments(
	ctx context.Context,
	config *common.RunnerConfig,
	now time.Time,
) error {
	if config.Kubernetes == nil {
		return nil
	}

	kubeConfig, err := p.getKubeConfig(config.Kubernetes, &overwrites{})
	if err != nil {
		return fmt.Errorf("getting Kubernetes config: %w", err)
	}

	client, err := p.newKubeClient(kubeConfig)
	if err != nil {
		return fmt.Errorf("creating Kubernetes client: %w", err)
	}
	defer closeKubeClient(client)

	logger := logrus.WithField("runner", config.ShortDescription())
	return sweepRuntimeEnvironments(ctx, client, config, now, logger)
}

func runtimeEnvironmentSelector(runnerID int64) string {
	return common.RuntimeEnvironmentRunnerIDLabel + "=" + strconv.FormatInt(runnerID, 10)
}

// sweepRuntimeEnvironments lists the PVCs labelled with the runner's ID and
// releases those past their expiry and grace that no pod mounts. With
// NamespacePerJob it then deletes the runner's labelled namespaces that no
// longer hold a PVC. Environments without the label predate the TTL and are
// never listed, so never removed.
func sweepRuntimeEnvironments(
	ctx context.Context,
	client kubernetes.Interface,
	config *common.RunnerConfig,
	now time.Time,
	logger logrus.FieldLogger,
) error {
	selector := runtimeEnvironmentSelector(config.ID)
	namespace := runtimeEnvironmentNamespace(config.Kubernetes)

	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: persistentvolumeclaims, list, FF_SUSPENDABLE_ENVIRONMENTS=true
	pvcs, err := client.CoreV1().PersistentVolumeClaims(namespace).
		List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("listing suspended environment PVCs in namespace %q: %w", namespace, err)
	}

	errs := []error{releaseExpiredPVCs(ctx, client, selector, pvcs.Items, now, logger)}
	if config.Kubernetes.NamespacePerJob {
		errs = append(errs, deleteEmptyRuntimeEnvironmentNamespaces(ctx, client, selector, logger))
	}

	return errors.Join(errs...)
}

// runtimeEnvironmentNamespace is where the runner's suspend PVCs live: the
// configured namespace, or every namespace with NamespacePerJob.
func runtimeEnvironmentNamespace(config *common.KubernetesConfig) string {
	switch {
	case config.NamespacePerJob:
		return metav1.NamespaceAll
	case config.Namespace == "":
		return DefaultResourceIdentifier
	default:
		return config.Namespace
	}
}

// releaseExpiredPVCs releases the PVCs sweepable at now that no pod mounts.
// A PVC without a valid expiry stamp is kept: its expiry is unknown. A PVC
// whose namespace's pods cannot be listed is kept: it is unclear whether a job
// still holds it.
func releaseExpiredPVCs(
	ctx context.Context,
	client kubernetes.Interface,
	selector string,
	pvcs []api.PersistentVolumeClaim,
	now time.Time,
	logger logrus.FieldLogger,
) error {
	var errs []error
	mounted := map[string]map[string]bool{}
	unlistable := map[string]bool{}
	for i := range pvcs {
		pvc := &pvcs[i]
		expiresAt, ok := sweepablePVC(pvc, now, logger)
		if !ok {
			continue
		}

		if unlistable[pvc.Namespace] {
			continue
		}
		claims, ok := mounted[pvc.Namespace]
		if !ok {
			var err error
			if claims, err = mountedClaims(ctx, client, pvc.Namespace); err != nil {
				errs = append(errs, err)
				unlistable[pvc.Namespace] = true
				continue
			}
			mounted[pvc.Namespace] = claims
		}

		log := logger.WithFields(logrus.Fields{
			"namespace":  pvc.Namespace,
			"pvc":        pvc.Name,
			"expires_at": expiresAt,
		})
		if claims[pvc.Name] {
			log.Debugln("Expired suspended environment is in use by a pod; keeping it")
			continue
		}

		fields := url.Values{envKeyNamespace: {pvc.Namespace}, envKeyPVC: {pvc.Name}}
		if err := releaseRuntimeEnvironment(ctx, client, selector, fields); err != nil {
			errs = append(errs, err)
			continue
		}
		log.Infoln("Released expired suspended environment")
	}

	return errors.Join(errs...)
}

// sweepablePVC returns the PVC's stamped expiry and whether the PVC is
// sweepable at now. A PVC already being deleted or without a valid stamp is
// not.
func sweepablePVC(pvc *api.PersistentVolumeClaim, now time.Time, logger logrus.FieldLogger) (time.Time, bool) {
	if pvc.DeletionTimestamp != nil {
		return time.Time{}, false
	}

	expiresAt, err := common.ParseRuntimeEnvironmentExpiry(pvc.Labels[common.RuntimeEnvironmentExpiresAtLabel])
	if err != nil {
		logger.WithFields(logrus.Fields{"namespace": pvc.Namespace, "pvc": pvc.Name}).
			WithError(err).Debugln("Suspended environment has no valid expiry stamp; keeping it")
		return time.Time{}, false
	}
	return expiresAt, common.RuntimeEnvironmentSweepable(expiresAt, now)
}

// mountedClaims returns the PVC names referenced by non-terminal pods in ns.
// A pending pod counts: a resume that passed its expiry check holds the PVC as
// soon as its pod exists, before it runs.
func mountedClaims(ctx context.Context, client kubernetes.Interface, ns string) (map[string]bool, error) {
	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: pods, list, FF_SUSPENDABLE_ENVIRONMENTS=true
	pods, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods in namespace %q: %w", ns, err)
	}

	claims := map[string]bool{}
	for _, pod := range pods.Items {
		if pod.Status.Phase == api.PodSucceeded || pod.Status.Phase == api.PodFailed {
			continue
		}
		for _, v := range pod.Spec.Volumes {
			if v.PersistentVolumeClaim != nil {
				claims[v.PersistentVolumeClaim.ClaimName] = true
			}
		}
	}
	return claims, nil
}

// releaseRuntimeEnvironment removes what Suspend preserved, given only the
// environment key fields: the PVC holding the overlay upper layer, the builds
// directory and service data, plus any overlay ConfigMap a job failed to
// delete in Cleanup (for example after a manager crash). ConfigMaps go first
// so a failure keeps the PVC and the next tick retries the whole release.
// Resources that are already gone count as released; another manager of the
// same registration may have removed them first. ConfigMaps must also match
// selector, so a manager never deletes another runner's ConfigMaps.
//
// The blueprint puts Release on SuspendableExecutor. Here it is a package
// function the provider's sweep calls, because nothing calls an executor-level
// Release and an executor has no client or config before Prepare.
func releaseRuntimeEnvironment(ctx context.Context, client kubernetes.Interface, selector string, fields url.Values) error {
	ns := fields.Get(envKeyNamespace)
	if ns == "" {
		return errors.New("release: environment key missing 'namespace' field")
	}
	pvc := fields.Get(envKeyPVC)
	if pvc == "" {
		return errors.New("release: environment key missing 'pvc' field")
	}

	if err := deleteRuntimeEnvironmentConfigMaps(ctx, client, selector, ns, pvc); err != nil {
		return err
	}

	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: persistentvolumeclaims, delete, FF_SUSPENDABLE_ENVIRONMENTS=true
	err := client.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, pvc, metav1.DeleteOptions{})
	if err != nil && !kubeerrors.IsNotFound(err) {
		return fmt.Errorf("release: deleting PVC %s/%s: %w", ns, pvc, err)
	}
	return nil
}

func deleteRuntimeEnvironmentConfigMaps(ctx context.Context, client kubernetes.Interface, selector, ns, pvc string) error {
	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: configmaps, list, FF_SUSPENDABLE_ENVIRONMENTS=true
	cms, err := client.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{
		LabelSelector: runtimeEnvironmentPVCLabel + "=" + pvc + "," + selector,
	})
	if err != nil {
		return fmt.Errorf("release: listing overlay ConfigMaps for PVC %s/%s: %w", ns, pvc, err)
	}

	for _, cm := range cms.Items {
		//nolint:gocritic // kubeAPI annotation, not commented-out code
		// kubeAPI: configmaps, delete, FF_SUSPENDABLE_ENVIRONMENTS=true
		err := client.CoreV1().ConfigMaps(ns).Delete(ctx, cm.Name, metav1.DeleteOptions{})
		if err != nil && !kubeerrors.IsNotFound(err) {
			return fmt.Errorf("release: deleting overlay ConfigMap %s/%s: %w", ns, cm.Name, err)
		}
	}
	return nil
}

// deleteEmptyRuntimeEnvironmentNamespaces deletes the runner's labelled
// per-job namespaces that hold no PVC. A namespace is labelled only after its
// PVC exists, so one that is still being set up is never selected.
func deleteEmptyRuntimeEnvironmentNamespaces(
	ctx context.Context,
	client kubernetes.Interface,
	selector string,
	logger logrus.FieldLogger,
) error {
	//nolint:gocritic // kubeAPI annotation, not commented-out code
	// kubeAPI: namespaces, list, FF_SUSPENDABLE_ENVIRONMENTS=true,kubernetes.NamespacePerJob=true
	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("listing suspended environment namespaces: %w", err)
	}

	var errs []error
	for _, ns := range namespaces.Items {
		if ns.DeletionTimestamp != nil {
			continue
		}

		//nolint:gocritic // kubeAPI annotation, not commented-out code
		// kubeAPI: persistentvolumeclaims, list, FF_SUSPENDABLE_ENVIRONMENTS=true
		pvcs, err := client.CoreV1().PersistentVolumeClaims(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("listing PVCs in namespace %q: %w", ns.Name, err))
			continue
		}
		if holdsLivePVC(pvcs.Items) {
			continue
		}

		//nolint:gocritic // kubeAPI annotation, not commented-out code
		// kubeAPI: namespaces, delete, kubernetes.NamespacePerJob=true
		err = client.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{})
		if err != nil && !kubeerrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("deleting namespace %q: %w", ns.Name, err))
			continue
		}
		logger.WithField("namespace", ns.Name).Infoln("Deleted namespace of released suspended environment")
	}

	return errors.Join(errs...)
}

func holdsLivePVC(pvcs []api.PersistentVolumeClaim) bool {
	for _, pvc := range pvcs {
		if pvc.DeletionTimestamp == nil {
			return true
		}
	}
	return false
}

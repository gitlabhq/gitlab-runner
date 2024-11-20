package kubernetes

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8s "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

// GetKubeClientConfig is used to export the getKubeClientConfig function for integration tests
func GetKubeClientConfig(config *common.KubernetesConfig) (kubeConfig *restclient.Config, err error) {
	return getKubeClientConfig(config, new(overwrites))
}

// NewDefaultExecutorForTest is used to expose the executor to integration tests
func NewDefaultExecutorForTest() common.Executor {
	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
	}
	e.getKubeConfig = getKubeClientConfig
	e.newKubeClient = func(config *restclient.Config) (kubernetes.Interface, error) {
		return kubernetes.NewForConfig(config)
	}
	return e
}

func SkipKubectlIntegrationTests(t *testing.T, cmd ...string) {
	// In CI don't run the command, it's already run by the CI job.
	// this will speed up the test run and will not require us to give more permissions to the kubernetes service account.
	if os.Getenv("GITLAB_CI") == "true" {
		return
	}

	helpers.SkipIntegrationTests(t, cmd...)
}

func CreateTestKubernetesResource[T metav1.Object](ctx context.Context, client *k8s.Clientset, defaultNamespace string, resource T) (T, error) {
	if resource.GetName() == "" {
		resource.SetName(fmt.Sprintf("test-unknown-%d", rand.Uint64()))
	}

	if resource.GetNamespace() == "" {
		resource.SetNamespace(defaultNamespace)
	}

	resource.SetLabels(map[string]string{
		"test.k8s.gitlab.com/name": resource.GetName(),
	})

	var res any
	var err error
	switch any(resource).(type) {
	case *v1.ServiceAccount:
		res, err = client.CoreV1().ServiceAccounts(resource.GetNamespace()).Create(ctx, any(resource).(*v1.ServiceAccount), metav1.CreateOptions{})
	case *v1.Secret:
		res, err = client.CoreV1().Secrets(resource.GetNamespace()).Create(ctx, any(resource).(*v1.Secret), metav1.CreateOptions{})
	default:
		return *new(T), fmt.Errorf("unsupported resource type: %T", resource)
	}

	if err != nil {
		return *new(T), err
	}

	return res.(T), nil
}

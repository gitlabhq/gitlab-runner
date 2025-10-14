package kubernetes

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

// GetKubeClientConfig is used to export the getKubeClientConfig function for integration tests
func GetKubeClientConfig(config *common.KubernetesConfig) (kubeConfig *rest.Config, err error) {
	return getKubeClientConfig(config, new(overwrites))
}

func SkipKubectlIntegrationTests(t *testing.T, cmd ...string) {
	// In CI don't run the command, it's already run by the CI job.
	// this will speed up the test run and will not require us to give more permissions to the kubernetes service account.
	if os.Getenv("GITLAB_CI") == "true" {
		return
	}

	helpers.SkipIntegrationTests(t, cmd...)
}

func CreateTestKubernetesResource[T metav1.Object](ctx context.Context, client *kubernetes.Clientset, defaultNamespace string, resource T) (T, error) {
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

// FakeClient wraps around a standard client, allowing to overwrite certain methods.
//
// While FakeClient can wrap around any kubernetes client, the default use-case for this wrapper is to wrap around
// *fake.ClientSet, to be able to test things the *fake.Clientset and its standard reactor pattern does not support.
// Examples for that are tests against the discoveryClient or the RESTClient.
//
// Example: set up a fake discovery client
//
//	fakeClient := &FakeClient{
//		Interface: fake.NewSimpleClientset(),
//		FakeDiscovery: &FakeDiscovery{
//			FakeVersion:    "some version",
//			FakeVersionErr: fmt.Errorf("some api error"),
//		},
//	}
//	testSomethingOnDiscovery(fakeClient)
//
// Example: set up a fake RESTClient for the corev1 APIs
//
//	fakeClientSet := fake.NewSimpleClientset()
//	fakeRESTClient := &fakerest.RESTClient{
//		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
//		Client: fakerest.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
//			return &http.Response{ ... }, nil
//		},
//	},
//	fakeClient := &FakeClient{
//		Interface: fakeClientSet,
//		FakeCoreV1: fake.FakeCoreV1{
//		 	CoreV1Interface: fakeClientSet.CoreV1(),
//			FakeRESTClient: fakeRestClient,
//		}
//	})
//	testSomethingWithTheCoreV1RESTClient(fakeClient)
//
// If you want to ensure a certain test only uses the injected fake DiscoveryClient but nothing else, you can omit
// setting the inner client, or set it to `nil` explicitly. With that, the wrapper still implements a standard
// kubernetes client, but calling anything not from the faked discovery client will fail, in fact: panic.
//
// Example: ensure only the DiscoveryClient is used
//
//	fakeClient := &FakeClient{
//		Interface: nil, // or omit setting `Interface`
//		FakeDiscovery: &FakeDiscovery{
//			FakeVersion:    "some version",
//			FakeVersionErr: fmt.Errorf("some api error"),
//		},
//	}
//	testSomethingOnDiscovery(fakeClient) // any call not to the faked Discovery Client will panic
//
// A similar approach can be taken by not setting the CoreV1Interface on the FakeCoreV1, which would mean that any
// interaction with CoreV1 that is not explicitly faked out would fail.
//
// Note: This wrapper should only be used when *fake.Clientset does not support what we want to test for; else you can
// use the *fake.Clientset directly, there is no need to wrap it.
//
// Note: For now, only FakeDiscovery and FakeCoreV1 are implemented and able to be faked out. We know we interact with
// those, and the *fake.Clientset does not have support to handle those. If we find other things we need to support, we
// can adapt the FakeClient et al as needed.
type FakeClient struct {
	kubernetes.Interface

	FakeDiscovery discovery.DiscoveryInterface
	FakeCoreV1    corev1.CoreV1Interface
}

var _ kubernetes.Interface = &FakeClient{}

func (fc *FakeClient) Discovery() discovery.DiscoveryInterface {
	if f := fc.FakeDiscovery; f != nil {
		return f
	}
	return fc.Interface.Discovery()
}

func (fc *FakeClient) CoreV1() corev1.CoreV1Interface {
	if f := fc.FakeCoreV1; f != nil {
		return f
	}
	return fc.Interface.CoreV1()
}

// FakeDiscovery wraps around the DiscoveryInterface, to be able to fake out certain parts of it.
type FakeDiscovery struct {
	discovery.DiscoveryInterface

	FakeVersion    *version.Info
	FakeVersionErr error
}

var _ discovery.DiscoveryInterface = &FakeDiscovery{}

func (fd *FakeDiscovery) ServerVersion() (*version.Info, error) {
	return fd.FakeVersion, fd.FakeVersionErr
}

// FakeCoreV1 wraps around the CoreV1Interface to be able to fake out certain parts of it.
type FakeCoreV1 struct {
	corev1.CoreV1Interface

	FakeRESTClient rest.Interface
}

func (fcv1 *FakeCoreV1) RESTClient() rest.Interface {
	if f := fcv1.FakeRESTClient; f != nil {
		return f
	}
	return fcv1.CoreV1Interface.RESTClient()
}

var _ corev1.CoreV1Interface = &FakeCoreV1{}

// FakeRESTClient wraps around the RESTClient to be able to fake out certain parts of it.
type FakeRESTClient struct {
	rest.Interface

	fakePostRequest *rest.Request
}

func (frc *FakeRESTClient) Post() *rest.Request {
	return frc.fakePostRequest
}

var _ rest.Interface = &FakeRESTClient{}

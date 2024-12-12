package testdata

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func testFnArg(client kubernetes.Interface) {
	_, _ = client.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

func testFnArgAnnotated(client kubernetes.Interface) {
	// kubeAPI: pods, get
	_, _ = client.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

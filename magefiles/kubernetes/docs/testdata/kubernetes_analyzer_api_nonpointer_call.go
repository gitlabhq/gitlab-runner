package testdata

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type test2 struct {
	client kubernetes.Interface
}

func (c test2) testNonPointerCall() {
	_, _ = c.client.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

func (c test2) testNonPointerCallAnnotated() {
	// kubeAPI: pods, get
	_, _ = c.client.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

package testdata

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type test1 struct {
	client kubernetes.Interface
}

func (c *test1) testPointerCall() {
	_, _ = c.client.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

func (c *test1) testPointerCallAnnotated() {
	// kubeAPI: pods, get
	_, _ = c.client.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

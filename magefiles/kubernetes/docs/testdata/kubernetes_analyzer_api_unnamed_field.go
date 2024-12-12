package testdata

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type test3 struct {
	kubernetes.Interface
}

func (c *test3) testUnnamedFieldCall() {
	_, _ = c.Interface.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

func (c *test3) testUnnamedFieldCallAnnotated() {
	// kubeAPI: pods, get
	_, _ = c.Interface.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

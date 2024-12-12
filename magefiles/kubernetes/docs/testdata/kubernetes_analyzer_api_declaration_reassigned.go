package testdata

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var client kubernetes.Interface

func testDeclarationReassigned() {
	c := client
	_, _ = c.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

func testDeclarationReassignedAnnotated() {
	c := client
	// kubeAPI: pods, get
	_, _ = c.CoreV1().Pods("").Get(nil, "", metav1.GetOptions{})
}

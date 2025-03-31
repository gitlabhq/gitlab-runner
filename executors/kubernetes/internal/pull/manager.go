package pull

import (
	"errors"
	"fmt"
	"sync"

	api "k8s.io/api/core/v1"
)

// Manager defines the interface for a state machine which keeps track of the appropriate pull policy to use
// for each image definition
type Manager interface {
	// GetPullPolicyFor returns the pull policy that should be used for the subsequent pull operation
	// for the specified container
	GetPullPolicyFor(container string) (api.PullPolicy, error)
	// UpdatePolicyForContainer updates the pull policy for the container designated in the specified error,
	// and returns whether a new pull operation with a different pull policy can be attempted
	UpdatePolicyForContainer(attempt int, imagePullErr *ImagePullError) bool
}

type pullLogger interface {
	Infoln(args ...interface{})
	Warningln(args ...interface{})
}

type manager struct {
	logger       pullLogger
	pullPolicies map[string][]api.PullPolicy

	mu         sync.Mutex
	failureMap map[string]int
}

func NewPullManager(pullPolicies map[string][]api.PullPolicy, logger pullLogger) Manager {
	return &manager{
		pullPolicies: pullPolicies,
		failureMap:   map[string]int{},
		logger:       logger,
	}
}

func (m *manager) GetPullPolicyFor(container string) (api.PullPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	failureCount := m.failureMap[container]
	pullPolicies, ok := m.pullPolicies[container]
	if !ok {
		pullPolicies = []api.PullPolicy{""}
	}

	if failureCount < len(pullPolicies) {
		return pullPolicies[failureCount], nil
	}

	return "", errors.New("pull failed")
}

func (m *manager) UpdatePolicyForContainer(attempt int, imagePullErr *ImagePullError) bool {
	pullPolicy, _ := m.GetPullPolicyFor(imagePullErr.Container)

	m.markPullFailureFor(imagePullErr.Container)

	m.logger.Warningln(fmt.Sprintf(
		"Failed to pull image %q for container %q with policy %q: %v",
		imagePullErr.Image,
		imagePullErr.Container,
		pullPolicy,
		imagePullErr.Message,
	))

	nextPullPolicy, errPull := m.GetPullPolicyFor(imagePullErr.Container)
	if errPull == nil {
		m.logger.Infoln(fmt.Sprintf(
			"Attempt #%d: Trying %q pull policy for %q image for container %q",
			attempt+1,
			nextPullPolicy,
			imagePullErr.Image,
			imagePullErr.Container,
		))
		return true
	}

	return false
}

// markPullFailureFor informs of a failure to pull the specified image
func (m *manager) markPullFailureFor(container string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failureMap[container]++
}

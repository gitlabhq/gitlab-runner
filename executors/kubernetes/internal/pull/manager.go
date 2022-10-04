package pull

import (
	"errors"
	"fmt"
	"sync"

	api "k8s.io/api/core/v1"
)

// Manager defines the interface for a state machine which keeps track of the appropriate pull policy to use
// for each image definition
//
//go:generate mockery --name=Manager --inpackage
type Manager interface {
	// GetPullPolicyFor returns the pull policy that should be used for the subsequent pull operation
	// for the specified image
	GetPullPolicyFor(image string) (api.PullPolicy, error)
	// UpdatePolicyForImage updates the pull policy for the image designated in the specified error,
	// and returns whether a new pull operation with a different pull policy can be attempted
	UpdatePolicyForImage(attempt int, imagePullErr *ImagePullError) bool
}

//go:generate mockery --name=pullLogger --inpackage
type pullLogger interface {
	Infoln(args ...interface{})
	Warningln(args ...interface{})
}

type manager struct {
	logger       pullLogger
	pullPolicies []api.PullPolicy

	mu         sync.Mutex
	failureMap map[string]int
}

func NewPullManager(pullPolicies []api.PullPolicy, logger pullLogger) Manager {
	if len(pullPolicies) == 0 {
		pullPolicies = []api.PullPolicy{""}
	}

	return &manager{
		pullPolicies: pullPolicies,
		failureMap:   map[string]int{},
		logger:       logger,
	}
}

func (m *manager) GetPullPolicyFor(image string) (api.PullPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	failureCount := m.failureMap[image]
	if failureCount < len(m.pullPolicies) {
		return m.pullPolicies[failureCount], nil
	}

	return "", errors.New("pull failed")
}

func (m *manager) UpdatePolicyForImage(attempt int, imagePullErr *ImagePullError) bool {
	pullPolicy, _ := m.GetPullPolicyFor(imagePullErr.Image)

	m.markPullFailureFor(imagePullErr.Image)

	m.logger.Warningln(fmt.Sprintf(
		"Failed to pull image with policy %q: %v",
		pullPolicy,
		imagePullErr.Message,
	))

	nextPullPolicy, errPull := m.GetPullPolicyFor(imagePullErr.Image)
	if errPull == nil {
		m.logger.Infoln(fmt.Sprintf(
			"Attempt #%d: Trying %q pull policy for %q image",
			attempt+1,
			nextPullPolicy,
			imagePullErr.Image,
		))
		return true
	}

	return false
}

// markPullFailureFor informs of a failure to pull the specified image
func (m *manager) markPullFailureFor(image string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failureMap[image]++
}

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
	"golang.org/x/sync/errgroup"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// executorStateMetadata is a struct that holds the metadata of the executor state.
// the struct is transformed through the common.JobStateToEncoded and common.JobStateFromEncoded interfaces
// in order to store less data in the underlying store.
// The struct must be synchronized in a specific manner.
// The only concurrent access happens from the common.StatefulJobManager struct that updates
// the store in the background. For this reason MarshalInto must be synchronized as it can be called
// while the executor is initializing and the state is being updated.
// On the other hand any write access to the field must be synchronized as it can happen
// during the executor's initialization.
// All other read access to the fields in the executor do not need to be synchronized as they happen
// after the executorStateMetadata has already been initialized.
type executorStateMetadata struct {
	sync.Mutex

	pod         *api.Pod
	credentials *api.Secret
	services    []api.Service
	offset      int64
}

func (e *executorStateMetadata) setPod(pod *api.Pod) {
	e.Lock()
	defer e.Unlock()
	e.pod = pod
}

func (e *executorStateMetadata) setCredentials(credentials *api.Secret) {
	e.Lock()
	defer e.Unlock()
	e.credentials = credentials
}

func (e *executorStateMetadata) setServices(services []api.Service) {
	e.Lock()
	defer e.Unlock()
	e.services = services
}

func (e *executorStateMetadata) setOffset(offset int64) {
	e.Lock()
	defer e.Unlock()
	e.offset = offset
}

var _ common.JobStateToEncoded[any] = (*executorStateMetadata)(nil)

type encodedExecutorStateMetadata struct {
	Pod         string
	Credentials string
	Services    []string
	Namespace   string
	Offset      int64
}

var _ common.JobStateFromEncoded[any] = (*encodedExecutorStateMetadata)(nil)

func init() {
	common.RegisterJobState(&encodedExecutorStateMetadata{})
}

func (e *executorStateMetadata) ToEncoded() (any, error) {
	if e == nil {
		return nil, nil
	}

	e.Lock()
	defer e.Unlock()

	if e.pod == nil {
		return nil, nil
	}

	var credentialsName string
	if e.credentials != nil {
		credentialsName = e.credentials.Name
	}

	return &encodedExecutorStateMetadata{
		Pod:         e.pod.Name,
		Credentials: credentialsName,
		Services: func() []string {
			services := make([]string, len(e.services))
			for i, service := range e.services {
				services[i] = service.Name
			}

			return services
		}(),
		Offset:    e.offset,
		Namespace: e.pod.Namespace,
	}, nil
}

func (from *encodedExecutorStateMetadata) FromEncoded() (any, error) {
	if from == nil {
		return nil, nil
	}

	into := &executorStateMetadata{}

	into.pod = &api.Pod{}
	into.pod.Name = from.Pod
	into.pod.Namespace = from.Namespace

	if from.Credentials != "" {
		into.credentials = &api.Secret{}
		into.credentials.Name = from.Credentials
		into.credentials.Namespace = from.Namespace
	}

	into.services = make([]api.Service, len(from.Services))
	for i, service := range from.Services {
		into.services[i] = api.Service{}
		into.services[i].Name = service
		into.services[i].Namespace = from.Namespace
	}

	into.offset = from.Offset
	return into, nil
}

func (s *executor) GetState() any {
	return s.state
}

func (s *executor) SetState(state any) bool {
	m, ok := state.(*executorStateMetadata)
	if ok {
		s.state = m
	}

	return ok
}

func (s *executor) canRestore() bool {
	return !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) && s.Config.Store.IsConfigured()
}

func (s *executor) restore(ctx context.Context) error {
	if !s.canRestore() {
		return errors.New("cannot restore state with legacy kubernetes execution strategy or store is not configured properly")
	}

	s.Build.Log().Println("Restoring state from a previous run...")

	var pod *api.Pod
	var credentials *api.Secret
	var services []api.Service

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		pod, err = retry.WithValueFn(s, func() (*api.Pod, error) {
			// kubeAPI: pods, get
			return s.kubeClient.CoreV1().
				Pods(s.state.pod.Namespace).
				Get(ctx, s.state.pod.Name, metav1.GetOptions{})
		}).Run()
		if err != nil {
			return fmt.Errorf("failed to restore pod: %w", err)
		}

		return nil
	})

	eg.Go(func() error {
		if s.state.credentials == nil {
			return nil
		}

		var err error
		credentials, err = retry.WithValueFn(s, func() (*api.Secret, error) {
			// kubeAPI: secrets, get
			return s.kubeClient.CoreV1().
				Secrets(s.state.credentials.Namespace).
				Get(ctx, s.state.credentials.Name, metav1.GetOptions{})
		}).Run()
		if err != nil {
			return fmt.Errorf("failed to restore credentials: %w", err)
		}

		return nil
	})

	eg.Go(func() error {
		services = make([]api.Service, len(s.state.services))
		for i, service := range s.state.services {
			srv, err := retry.WithValueFn(s, func() (*api.Service, error) {
				// kubeAPI: services, get
				return s.kubeClient.CoreV1().
					Services(service.Namespace).
					Get(ctx, service.Name, metav1.GetOptions{})
			}).Run()
			if err != nil {
				return fmt.Errorf("failed to restore service: %w", err)
			}

			services[i] = *srv
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	s.state.setPod(pod)
	s.state.setCredentials(credentials)
	s.state.setServices(services)

	return nil
}

func (s *executor) Resume(cmd common.ExecutorCommand) error {
	ctx := cmd.Context

	if err := s.restore(ctx); err != nil {
		return err
	}

	s.initializeLogsProcessing(ctx)

	podStatusCh := s.watchPodStatus(ctx, &podContainerStatusChecker{})
	select {
	case err := <-s.listenForCommandExit():
		return err
	case err := <-podStatusCh:
		return s.handlePodStatusError(err)
	case <-ctx.Done():
		return fmt.Errorf("build aborted")
	}
}

//go:build !integration

package autoscaler

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	api "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewPausePodManager(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	tests := []struct {
		name    string
		config  PausePodManagerConfig
		wantErr bool
	}{
		{
			name: "valid config with policies",
			config: PausePodManagerConfig{
				Namespace:        "default",
				RunnerShortToken: "test-runner",
				SystemID:         "s_testsystem",
				Policies: PolicyList{
					{Periods: []string{"* * * * *"}, IdleCount: 2},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config without policies",
			config: PausePodManagerConfig{
				Namespace:        "default",
				RunnerShortToken: "test-runner",
				SystemID:         "s_testsystem",
			},
			wantErr: false,
		},
		{
			name: "invalid policy period",
			config: PausePodManagerConfig{
				Namespace:        "default",
				RunnerShortToken: "test-runner",
				SystemID:         "s_testsystem",
				Policies: PolicyList{
					{Periods: []string{"invalid"}, IdleCount: 2},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewPausePodManager(client, tt.config, log, nil)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, manager)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, manager)
			}
		})
	}
}

func TestPausePodManager_ActiveJobs(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	manager, err := NewPausePodManager(client, PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
	}, log, nil)
	require.NoError(t, err)

	// Initial state
	assert.Equal(t, 0, manager.getActiveJobs())

	// Increment
	manager.IncrementActiveJobs()
	assert.Equal(t, 1, manager.getActiveJobs())

	manager.IncrementActiveJobs()
	assert.Equal(t, 2, manager.getActiveJobs())

	// Decrement
	manager.DecrementActiveJobs()
	assert.Equal(t, 1, manager.getActiveJobs())

	// Set directly
	manager.SetActiveJobs(5)
	assert.Equal(t, 5, manager.getActiveJobs())

	// Decrement doesn't go negative
	manager.SetActiveJobs(0)
	manager.DecrementActiveJobs()
	assert.Equal(t, 0, manager.getActiveJobs())
}

func TestPausePodManager_DeploymentName(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	manager, err := NewPausePodManager(client, PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "Wg8IWvTxZ",
		SystemID:         "s_testsystem",
	}, log, nil)
	require.NoError(t, err)

	// Runner IDs are lowercased to comply with RFC 1123
	assert.Equal(t, "runner-pause-wg8iwvtxz-stestsystem", manager.deploymentName())
}

func TestPausePodManager_BuildDeployment(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	runtimeClass := "gvisor"
	config := PausePodManagerConfig{
		Namespace:          "test-ns",
		RunnerShortToken:   "runner-123",
		SystemID:           "s_testsystem",
		PriorityClassName:  "low-priority",
		Image:              "custom-registry/pause:latest",
		RuntimeClassName:   &runtimeClass,
		ServiceAccountName: "runner-sa",
		NodeSelector:       map[string]string{"node-type": "runner"},
		Tolerations: []api.Toleration{
			{Key: "dedicated", Operator: api.TolerationOpEqual, Value: "runner"},
		},
		ResourceRequests: api.ResourceList{
			api.ResourceCPU:    resource.MustParse("100m"),
			api.ResourceMemory: resource.MustParse("128Mi"),
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	deployment := manager.buildDeployment(3)

	// Check metadata
	assert.Equal(t, "test-ns", deployment.Namespace)
	assert.Equal(t, "runner-pause-runner-123-stestsystem", deployment.Name)
	assert.Equal(t, pausePodLabelValue, deployment.Labels[pausePodLabel])
	assert.Equal(t, "runner-123", deployment.Labels[runnerIDLabel])

	// Check spec
	assert.Equal(t, int32(3), *deployment.Spec.Replicas)

	// Check selector
	assert.Equal(t, pausePodLabelValue, deployment.Spec.Selector.MatchLabels[pausePodLabel])
	assert.Equal(t, "runner-123", deployment.Spec.Selector.MatchLabels[runnerIDLabel])

	// Check pod template
	podSpec := deployment.Spec.Template.Spec
	assert.Equal(t, "low-priority", podSpec.PriorityClassName)
	assert.Equal(t, &runtimeClass, podSpec.RuntimeClassName)
	assert.Equal(t, "runner-sa", podSpec.ServiceAccountName)
	assert.Equal(t, map[string]string{"node-type": "runner"}, podSpec.NodeSelector)
	assert.Len(t, podSpec.Tolerations, 1)
	assert.Equal(t, int64(0), *podSpec.TerminationGracePeriodSeconds)

	// Check container
	require.Len(t, podSpec.Containers, 1)
	container := podSpec.Containers[0]
	assert.Equal(t, "pause", container.Name)
	assert.Equal(t, "custom-registry/pause:latest", container.Image)
	assert.Equal(t, resource.MustParse("100m"), container.Resources.Requests[api.ResourceCPU])
	assert.Equal(t, resource.MustParse("128Mi"), container.Resources.Requests[api.ResourceMemory])
}

func TestPausePodManager_BuildDeployment_DefaultImage(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "runner-123",
		SystemID:         "s_testsystem",
		// Image not set - should use default
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	deployment := manager.buildDeployment(1)
	assert.Equal(t, defaultPausePodImage, deployment.Spec.Template.Spec.Containers[0].Image)
}

func TestPausePodManager_CreateDeployment(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 3},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Create deployment
	err = manager.createDeployment(ctx, 3)
	require.NoError(t, err)

	// Verify deployment created
	deployment, err := client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(3), *deployment.Spec.Replicas)
	assert.Equal(t, pausePodLabelValue, deployment.Labels[pausePodLabel])
}

func TestPausePodManager_UpdateDeploymentReplicas(t *testing.T) {
	// Create initial deployment
	initialReplicas := int32(2)
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runner-pause-test-runner-stestsystem",
			Namespace: "default",
			Labels: map[string]string{
				pausePodLabel: pausePodLabelValue,
				runnerIDLabel: "test-runner",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &initialReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pausePodLabel: pausePodLabelValue,
					runnerIDLabel: "test-runner",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						pausePodLabel: pausePodLabelValue,
						runnerIDLabel: "test-runner",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{{Name: "pause", Image: defaultPausePodImage}},
				},
			},
		},
	}

	client := fake.NewClientset(existingDeployment)
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Update replicas
	err = manager.updateDeploymentReplicas(ctx, 5)
	require.NoError(t, err)

	// Verify update
	deployment, err := client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(5), *deployment.Spec.Replicas)
}

func TestPausePodManager_DeleteDeployment(t *testing.T) {
	// Create initial deployment
	replicas := int32(2)
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runner-pause-test-runner-stestsystem",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	client := fake.NewClientset(existingDeployment)
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Delete deployment
	err = manager.deleteDeployment(ctx)
	require.NoError(t, err)

	// Verify deleted
	_, err = client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	assert.True(t, err != nil)
}

func TestPausePodManager_DeleteDeployment_NotFound(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Delete non-existent deployment should not error
	err = manager.deleteDeployment(ctx)
	require.NoError(t, err)
}

func TestPausePodManager_Reconcile_CreatesDeployment(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 2},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// First reconcile should create deployment with 2 replicas
	manager.reconcile(ctx)

	deployment, err := client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(2), *deployment.Spec.Replicas)
}

func TestPausePodManager_Reconcile_UpdatesReplicas(t *testing.T) {
	// Create initial deployment with 2 replicas
	initialReplicas := int32(2)
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runner-pause-test-runner-stestsystem",
			Namespace: "default",
			Labels: map[string]string{
				pausePodLabel: pausePodLabelValue,
				runnerIDLabel: "test-runner",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &initialReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pausePodLabel: pausePodLabelValue,
					runnerIDLabel: "test-runner",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						pausePodLabel: pausePodLabelValue,
						runnerIDLabel: "test-runner",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{{Name: "pause", Image: defaultPausePodImage}},
				},
			},
		},
	}

	client := fake.NewClientset(existingDeployment)
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 5},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Reconcile should update to 5 replicas
	manager.reconcile(ctx)

	deployment, err := client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(5), *deployment.Spec.Replicas)
}

func TestPausePodManager_Reconcile_ScalesDown(t *testing.T) {
	// Create initial deployment with 5 replicas
	initialReplicas := int32(5)
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runner-pause-test-runner-stestsystem",
			Namespace: "default",
			Labels: map[string]string{
				pausePodLabel: pausePodLabelValue,
				runnerIDLabel: "test-runner",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &initialReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pausePodLabel: pausePodLabelValue,
					runnerIDLabel: "test-runner",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						pausePodLabel: pausePodLabelValue,
						runnerIDLabel: "test-runner",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{{Name: "pause", Image: defaultPausePodImage}},
				},
			},
		},
	}

	client := fake.NewClientset(existingDeployment)
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 2},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Reconcile should scale down to 2 replicas
	manager.reconcile(ctx)

	deployment, err := client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(2), *deployment.Spec.Replicas)
}

func TestPausePodManager_Reconcile_NoOpWhenAtTarget(t *testing.T) {
	// Create initial deployment with correct replicas
	initialReplicas := int32(3)
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runner-pause-test-runner-stestsystem",
			Namespace: "default",
			Labels: map[string]string{
				pausePodLabel: pausePodLabelValue,
				runnerIDLabel: "test-runner",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &initialReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pausePodLabel: pausePodLabelValue,
					runnerIDLabel: "test-runner",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						pausePodLabel: pausePodLabelValue,
						runnerIDLabel: "test-runner",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{{Name: "pause", Image: defaultPausePodImage}},
				},
			},
		},
	}

	client := fake.NewClientset(existingDeployment)
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 3},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Reconcile should not change anything
	manager.reconcile(ctx)

	deployment, err := client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(3), *deployment.Spec.Replicas)
}

func TestPausePodManager_Reconcile_DoesNotCreateWhenZeroReplicas(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 0},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Reconcile should not create deployment when 0 replicas needed
	manager.reconcile(ctx)

	_, err = client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	assert.True(t, err != nil) // Should not exist
}

func TestPausePodManager_Stop(t *testing.T) {
	// Create initial deployment
	replicas := int32(2)
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runner-pause-test-runner-stestsystem",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	client := fake.NewClientset(existingDeployment)
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Stop should clean up deployment
	manager.Stop(ctx)

	_, err = client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	assert.True(t, err != nil) // Should be deleted

	// Double stop should be safe
	manager.Stop(ctx)
}

func TestBuildResourceRequests(t *testing.T) {
	tests := []struct {
		name           string
		cpuRequest     string
		memoryRequest  string
		expectedCPU    string
		expectedMemory string
	}{
		{
			name:           "both set",
			cpuRequest:     "500m",
			memoryRequest:  "256Mi",
			expectedCPU:    "500m",
			expectedMemory: "256Mi",
		},
		{
			name:           "only cpu",
			cpuRequest:     "1",
			memoryRequest:  "",
			expectedCPU:    "1",
			expectedMemory: "",
		},
		{
			name:           "only memory",
			cpuRequest:     "",
			memoryRequest:  "1Gi",
			expectedCPU:    "",
			expectedMemory: "1Gi",
		},
		{
			name:           "neither set",
			cpuRequest:     "",
			memoryRequest:  "",
			expectedCPU:    "",
			expectedMemory: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := BuildResourceRequests(tt.cpuRequest, tt.memoryRequest)

			if tt.expectedCPU != "" {
				assert.Equal(t, resource.MustParse(tt.expectedCPU), resources[api.ResourceCPU])
			} else {
				_, exists := resources[api.ResourceCPU]
				assert.False(t, exists)
			}

			if tt.expectedMemory != "" {
				assert.Equal(t, resource.MustParse(tt.expectedMemory), resources[api.ResourceMemory])
			} else {
				_, exists := resources[api.ResourceMemory]
				assert.False(t, exists)
			}
		})
	}
}

func TestApplyScaleDownCooldown(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 5, IdleTime: 1 * time.Minute},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	policy := config.Policies[0]
	now := time.Now()

	// Initial scale up to 5
	result := manager.applyScaleDownCooldown(5, policy, now)
	assert.Equal(t, 5, result)

	// Scale up to 10 should be immediate
	result = manager.applyScaleDownCooldown(10, policy, now)
	assert.Equal(t, 10, result)

	// Scale down to 3 should be blocked (cooldown)
	result = manager.applyScaleDownCooldown(3, policy, now)
	assert.Equal(t, 10, result, "scale down should be blocked during cooldown")

	// Simulate cooldown expiry
	manager.mu.Lock()
	manager.scaleDownAllowedAt = time.Now().Add(-1 * time.Second)
	manager.mu.Unlock()

	// Now scale down should work
	result = manager.applyScaleDownCooldown(3, policy, time.Now())
	assert.Equal(t, 3, result, "scale down should work after cooldown")
}

func TestCleanupOrphanedDeployments(t *testing.T) {
	log := logrus.NewEntry(logrus.New())

	tests := []struct {
		name              string
		deployments       []*appsv1.Deployment
		expectedDeleted   []string
		expectedRemaining []string
	}{
		{
			name:              "no deployments",
			deployments:       nil,
			expectedDeleted:   nil,
			expectedRemaining: nil,
		},
		{
			name: "deployment without heartbeat annotation is skipped",
			deployments: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "runner-pause-no-heartbeat",
						Namespace: "default",
						Labels: map[string]string{
							pausePodLabel: pausePodLabelValue,
						},
					},
				},
			},
			expectedDeleted:   nil,
			expectedRemaining: []string{"runner-pause-no-heartbeat"},
		},
		{
			name: "deployment with recent heartbeat is kept",
			deployments: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "runner-pause-recent",
						Namespace: "default",
						Labels: map[string]string{
							pausePodLabel: pausePodLabelValue,
						},
						Annotations: map[string]string{
							heartbeatAnnotation: time.Now().UTC().Format(time.RFC3339),
						},
					},
				},
			},
			expectedDeleted:   nil,
			expectedRemaining: []string{"runner-pause-recent"},
		},
		{
			name: "deployment with stale heartbeat is deleted",
			deployments: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "runner-pause-stale",
						Namespace: "default",
						Labels: map[string]string{
							pausePodLabel: pausePodLabelValue,
						},
						Annotations: map[string]string{
							heartbeatAnnotation: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
						},
					},
				},
			},
			expectedDeleted:   []string{"runner-pause-stale"},
			expectedRemaining: nil,
		},
		{
			name: "mixed deployments - only stale ones deleted",
			deployments: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "runner-pause-recent",
						Namespace: "default",
						Labels: map[string]string{
							pausePodLabel: pausePodLabelValue,
						},
						Annotations: map[string]string{
							heartbeatAnnotation: time.Now().UTC().Format(time.RFC3339),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "runner-pause-stale",
						Namespace: "default",
						Labels: map[string]string{
							pausePodLabel: pausePodLabelValue,
						},
						Annotations: map[string]string{
							heartbeatAnnotation: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "runner-pause-no-annotation",
						Namespace: "default",
						Labels: map[string]string{
							pausePodLabel: pausePodLabelValue,
						},
					},
				},
			},
			expectedDeleted:   []string{"runner-pause-stale"},
			expectedRemaining: []string{"runner-pause-recent", "runner-pause-no-annotation"},
		},
		{
			name: "non-pause-pod deployment is ignored",
			deployments: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-deployment",
						Namespace: "default",
						Labels: map[string]string{
							"app": "something-else",
						},
						Annotations: map[string]string{
							heartbeatAnnotation: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
						},
					},
				},
			},
			expectedDeleted:   nil,
			expectedRemaining: []string{"other-deployment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset()

			// Create test deployments
			for _, d := range tt.deployments {
				_, err := client.AppsV1().Deployments("default").Create(t.Context(), d, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			// Run cleanup
			err := CleanupOrphanedDeployments(t.Context(), client, "default", log)
			require.NoError(t, err)

			// Verify deleted deployments are gone
			for _, name := range tt.expectedDeleted {
				_, err := client.AppsV1().Deployments("default").Get(t.Context(), name, metav1.GetOptions{})
				assert.True(t, err != nil, "deployment %s should have been deleted", name)
			}

			// Verify remaining deployments still exist
			for _, name := range tt.expectedRemaining {
				_, err := client.AppsV1().Deployments("default").Get(t.Context(), name, metav1.GetOptions{})
				assert.NoError(t, err, "deployment %s should still exist", name)
			}
		})
	}
}

func TestPausePodManager_CalculateDesiredReplicas_ScaleFactor(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 2, ScaleFactor: 0.5},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	policy := config.Policies[0]

	// No active jobs: desired = max(idle_count=2, 0*0.5=0) = 2
	manager.SetActiveJobs(0)
	assert.Equal(t, 2, manager.calculateDesiredReplicas(policy))

	// 3 active jobs: desired = max(2, ceil(3*0.5)=2) = 2
	manager.SetActiveJobs(3)
	assert.Equal(t, 2, manager.calculateDesiredReplicas(policy))

	// 10 active jobs: desired = max(2, ceil(10*0.5)=5) = 5
	manager.SetActiveJobs(10)
	assert.Equal(t, 5, manager.calculateDesiredReplicas(policy))

	// 1 active job: desired = max(2, ceil(1*0.5)=1) = 2
	manager.SetActiveJobs(1)
	assert.Equal(t, 2, manager.calculateDesiredReplicas(policy))
}

func TestPausePodManager_CalculateDesiredReplicas_ScaleFactorLimit(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 2, ScaleFactor: 0.5, ScaleFactorLimit: 4},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	policy := config.Policies[0]

	// 10 active jobs: scaled = ceil(10*0.5) = 5, capped at limit 4. desired = max(2, 4) = 4
	manager.SetActiveJobs(10)
	assert.Equal(t, 4, manager.calculateDesiredReplicas(policy))

	// 100 active jobs: scaled = ceil(100*0.5) = 50, capped at 4. desired = max(2, 4) = 4
	manager.SetActiveJobs(100)
	assert.Equal(t, 4, manager.calculateDesiredReplicas(policy))

	// 3 active jobs: scaled = ceil(3*0.5) = 2, under limit. desired = max(2, 2) = 2
	manager.SetActiveJobs(3)
	assert.Equal(t, 2, manager.calculateDesiredReplicas(policy))
}

func TestPausePodManager_CalculateDesiredReplicas_MaxPausePods(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		MaxPausePods:     3,
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 5},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	policy := config.Policies[0]

	// idle_count=5 but max_pause_pods=3, should be capped
	assert.Equal(t, 3, manager.calculateDesiredReplicas(policy))
}

func TestPausePodManager_CalculateDesiredReplicas_MaxPausePods_WithScaleFactor(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		MaxPausePods:     6,
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 2, ScaleFactor: 0.5},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	policy := config.Policies[0]

	// 20 active jobs: scaled = ceil(20*0.5) = 10, capped by max_pause_pods=6
	manager.SetActiveJobs(20)
	assert.Equal(t, 6, manager.calculateDesiredReplicas(policy))

	// 4 active jobs: scaled = ceil(4*0.5) = 2, desired = max(2,2) = 2, under max
	manager.SetActiveJobs(4)
	assert.Equal(t, 2, manager.calculateDesiredReplicas(policy))
}

func TestPausePodManager_EnsurePriorityClass_Creates(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:         "default",
		RunnerShortToken:  "test-runner",
		SystemID:          "s_testsystem",
		PriorityClassName: defaultPriorityClassName,
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Should create the PriorityClass
	err = manager.ensurePriorityClass(ctx)
	require.NoError(t, err)

	// Verify it was created
	pc, err := client.SchedulingV1().PriorityClasses().Get(ctx, defaultPriorityClassName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(defaultPriorityClassValue), pc.Value)
	assert.False(t, pc.GlobalDefault)
}

func TestPausePodManager_EnsurePriorityClass_AlreadyExists(t *testing.T) {
	// Pre-create the PriorityClass
	existingPC := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultPriorityClassName,
		},
		Value:         defaultPriorityClassValue,
		GlobalDefault: false,
	}
	client := fake.NewClientset(existingPC)
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:         "default",
		RunnerShortToken:  "test-runner",
		SystemID:          "s_testsystem",
		PriorityClassName: defaultPriorityClassName,
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Should succeed without error (no-op)
	err = manager.ensurePriorityClass(ctx)
	require.NoError(t, err)

	// PriorityClass should still exist with original values
	pc, err := client.SchedulingV1().PriorityClasses().Get(ctx, defaultPriorityClassName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, int32(defaultPriorityClassValue), pc.Value)
}

func TestPausePodManager_EnsurePriorityClass_CustomNameSkipsCreation(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:         "default",
		RunnerShortToken:  "test-runner",
		SystemID:          "s_testsystem",
		PriorityClassName: "my-custom-priority",
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	// Should not create anything
	err = manager.ensurePriorityClass(ctx)
	require.NoError(t, err)

	// Default PriorityClass should not exist
	_, err = client.SchedulingV1().PriorityClasses().Get(ctx, defaultPriorityClassName, metav1.GetOptions{})
	assert.Error(t, err)
}

func TestPausePodManager_Heartbeat(t *testing.T) {
	// Create a deployment so heartbeat has something to update
	replicas := int32(2)
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runner-pause-test-runner-stestsystem",
			Namespace: "default",
			Labels: map[string]string{
				pausePodLabel: pausePodLabelValue,
				runnerIDLabel: "test-runner",
			},
			Annotations: map[string]string{
				heartbeatAnnotation: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pausePodLabel: pausePodLabelValue,
					runnerIDLabel: "test-runner",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						pausePodLabel: pausePodLabelValue,
						runnerIDLabel: "test-runner",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{{Name: "pause", Image: defaultPausePodImage}},
				},
			},
		},
	}

	client := fake.NewClientset(existingDeployment)
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	before := time.Now()
	err = manager.updateHeartbeat(ctx)
	require.NoError(t, err)

	// Verify annotation was updated
	deployment, err := client.AppsV1().Deployments("default").Get(ctx, "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	require.NoError(t, err)

	heartbeatStr := deployment.Annotations[heartbeatAnnotation]
	require.NotEmpty(t, heartbeatStr)

	heartbeatTime, err := time.Parse(time.RFC3339, heartbeatStr)
	require.NoError(t, err)
	assert.False(t, heartbeatTime.Before(before.UTC().Truncate(time.Second)),
		"heartbeat should be at or after test start")

	// Verify internal tracking was updated
	assert.False(t, manager.lastHeartbeat.IsZero())
}

func TestPausePodManager_Start_InitialReconciliation(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 3},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	// Start in a goroutine and cancel immediately after initial reconciliation.
	done := make(chan struct{})
	go func() {
		manager.Start(ctx)
		close(done)
	}()

	// Wait for the deployment to be created by the initial reconciliation.
	require.Eventually(t, func() bool {
		deployment, err := client.AppsV1().Deployments("default").Get(
			t.Context(), "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
		return err == nil && deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 3
	}, 5*time.Second, 50*time.Millisecond, "deployment should be created with 3 replicas")

	cancel()
	<-done
}

func TestPausePodManager_Start_EnsuresPriorityClass(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 1},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		manager.Start(ctx)
		close(done)
	}()

	// Wait for PriorityClass to be created.
	require.Eventually(t, func() bool {
		_, err := client.SchedulingV1().PriorityClasses().Get(
			t.Context(), defaultPriorityClassName, metav1.GetOptions{})
		return err == nil
	}, 5*time.Second, 50*time.Millisecond, "PriorityClass should be created")

	cancel()
	<-done
}

func TestPausePodManager_Start_StopsOnContextCancel(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 1},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		manager.Start(ctx)
		close(done)
	}()

	// Let it start up, then cancel
	cancel()

	select {
	case <-done:
		// Start returned — success
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestPausePodManager_Start_StopsViaStopMethod(t *testing.T) {
	client := fake.NewClientset()
	log := logrus.NewEntry(logrus.New())

	config := PausePodManagerConfig{
		Namespace:        "default",
		RunnerShortToken: "test-runner",
		SystemID:         "s_testsystem",
		Policies: PolicyList{
			{Periods: []string{"* * * * *"}, IdleCount: 2},
		},
	}

	manager, err := NewPausePodManager(client, config, log, nil)
	require.NoError(t, err)

	ctx := t.Context()

	done := make(chan struct{})
	go func() {
		manager.Start(ctx)
		close(done)
	}()

	// Wait for initial reconciliation to create the deployment.
	require.Eventually(t, func() bool {
		_, err := client.AppsV1().Deployments("default").Get(
			t.Context(), "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
		return err == nil
	}, 5*time.Second, 50*time.Millisecond)

	// Stop should cause Start to return and clean up the deployment.
	manager.Stop(ctx)

	select {
	case <-done:
		// Start returned — success
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after Stop")
	}

	// Deployment should be deleted by Stop.
	_, err = client.AppsV1().Deployments("default").Get(
		t.Context(), "runner-pause-test-runner-stestsystem", metav1.GetOptions{})
	assert.Error(t, err, "deployment should be deleted after Stop")
}

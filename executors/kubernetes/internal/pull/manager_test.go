//go:build !integration

package pull

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
)

const buildContainer = "build"

func TestNewPullManager(t *testing.T) {
	m := NewPullManager(map[string][]api.PullPolicy{}, nil)
	assert.NotNil(t, m)
}

func TestGetPullPolicyFor(t *testing.T) {
	m := newPullManagerForTest(t, nil)

	pullPolicy, err := m.GetPullPolicyFor(buildContainer)
	assert.NoError(t, err)
	assert.Equal(t, api.PullAlways, pullPolicy)
}

func TestMarkPullFailureFor(t *testing.T) {
	t.Run("fails on fallback with no pull policies", func(t *testing.T) {
		l := newMockPullLogger(t)
		m := NewPullManager(map[string][]api.PullPolicy{}, l)

		pullPolicy, err := m.GetPullPolicyFor(buildContainer)
		assert.NoError(t, err)
		assert.Equal(t, api.PullPolicy(""), pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg("some image", buildContainer, ""),
		).Once()
		repeat := m.UpdatePolicyForContainer(1, &ImagePullError{Container: buildContainer, Image: "some image", Message: "server down"})
		assert.False(t, repeat, "UpdatePolicyForImage should return false")

		_, err = m.GetPullPolicyFor(buildContainer)
		assert.Error(t, err)
	})

	t.Run("succeeds on fallback with two pull policies", func(t *testing.T) {
		l := newMockPullLogger(t)
		m := newPullManagerForTest(t, l)

		pullPolicy, err := m.GetPullPolicyFor(buildContainer)
		assert.NoError(t, err)
		assert.Equal(t, api.PullAlways, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg("some image", buildContainer, "Always"),
		).Once()
		l.On(
			"Infoln",
			fmt.Sprintf(`Attempt #2: Trying "IfNotPresent" pull policy for %q image for container %q`, "some image", buildContainer),
		).Once()
		repeat := m.UpdatePolicyForContainer(1, &ImagePullError{Image: "some image", Container: buildContainer, Message: "server down"})
		assert.True(t, repeat, "UpdatePolicyForImage should return true")

		pullPolicy, err = m.GetPullPolicyFor(buildContainer)
		assert.NoError(t, err)
		assert.Equal(t, api.PullIfNotPresent, pullPolicy)
	})

	t.Run("succeeds on fallback with multiple images", func(t *testing.T) {
		l := newMockPullLogger(t)
		m := newPullManagerForTest(t, l)

		pullPolicy, err := m.GetPullPolicyFor(buildContainer)
		assert.NoError(t, err)
		assert.Equal(t, api.PullAlways, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg("some image", buildContainer, "Always"),
		).Once()
		l.On(
			"Infoln",
			fmt.Sprintf(`Attempt #2: Trying "IfNotPresent" pull policy for %q image for container %q`, "some image", buildContainer),
		).Once()
		repeat := m.UpdatePolicyForContainer(1, &ImagePullError{Image: "some image", Container: buildContainer, Message: "server down"})
		assert.True(t, repeat, "UpdatePolicyForImage should return true")

		pullPolicy, err = m.GetPullPolicyFor("helper")
		assert.NoError(t, err)
		assert.Equal(t, api.PullAlways, pullPolicy)

		pullPolicy, err = m.GetPullPolicyFor(buildContainer)
		assert.NoError(t, err)
		assert.Equal(t, api.PullIfNotPresent, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg("some other image", "helper", "Always"),
		).Once()
		l.On(
			"Infoln",
			fmt.Sprintf(`Attempt #2: Trying "IfNotPresent" pull policy for %q image for container %q`, "some other image", "helper"),
		).Once()
		repeat = m.UpdatePolicyForContainer(1, &ImagePullError{Image: "some other image", Container: "helper", Message: "server down"})
		assert.True(t, repeat, "UpdatePolicyForImage should return true")

		pullPolicy, err = m.GetPullPolicyFor("helper")
		assert.NoError(t, err)
		assert.Equal(t, api.PullIfNotPresent, pullPolicy)
	})

	t.Run("fails after second fallback", func(t *testing.T) {
		l := newMockPullLogger(t)
		m := newPullManagerForTest(t, l)

		pullPolicy, err := m.GetPullPolicyFor(buildContainer)
		assert.NoError(t, err)
		assert.Equal(t, api.PullAlways, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg("some image", buildContainer, "Always"),
		).Once()
		l.On(
			"Infoln",
			fmt.Sprintf(`Attempt #2: Trying "IfNotPresent" pull policy for %q image for container %q`, "some image", buildContainer),
		).Once()
		repeat := m.UpdatePolicyForContainer(1, &ImagePullError{Image: "some image", Container: buildContainer, Message: "server down"})
		assert.True(t, repeat, "UpdatePolicyForImage should return true")

		pullPolicy, err = m.GetPullPolicyFor(buildContainer)
		assert.NoError(t, err)
		assert.Equal(t, api.PullIfNotPresent, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg("some image", buildContainer, "IfNotPresent"),
		).Once()
		repeat = m.UpdatePolicyForContainer(2, &ImagePullError{Image: "some image", Container: buildContainer, Message: "server down"})
		assert.False(t, repeat, "UpdatePolicyForImage should return false")

		_, err = m.GetPullPolicyFor(buildContainer)
		assert.Error(t, err)
	})
}

func TestMultipleImagesConcurrently(t *testing.T) {
	l := newMockPullLogger(t)

	imagePolicies := map[string][]api.PullPolicy{
		"svc-0": {api.PullAlways, api.PullIfNotPresent, "", api.PullNever},
		"svc-1": {api.PullIfNotPresent, api.PullNever},
	}

	m := NewPullManager(imagePolicies, l)
	require.NotNil(t, m)

	l.On("Infoln", `Attempt #1: Trying "IfNotPresent" pull policy for "some image" image for container "svc-0"`)
	l.On("Infoln", `Attempt #2: Trying "" pull policy for "some image" image for container "svc-0"`)
	l.On("Infoln", `Attempt #3: Trying "Never" pull policy for "some image" image for container "svc-0"`)
	l.On("Infoln", `Attempt #1: Trying "Never" pull policy for "some image" image for container "svc-1"`)

	for container, policies := range imagePolicies {
		t.Run(container, func(t *testing.T) {
			t.Parallel()

			nrOfPolicies := len(policies)
			for i, policy := range policies {
				l.On("Warningln", failedToPullMsg("some image", container, string(policy))).Once()

				curPolicy, err := m.GetPullPolicyFor(container)
				assert.NoError(t, err)
				assert.Equal(t, policy, curPolicy, "expected image %q to currently have the policy %q, but has %q", container, policy, curPolicy)

				hasAnotherPolicy := m.UpdatePolicyForContainer(i, &ImagePullError{Image: "some image", Container: container, Message: "server down"})
				if i == nrOfPolicies-1 {
					assert.False(t, hasAnotherPolicy, "expected to stop on attempt %d", i)
				} else {
					assert.True(t, hasAnotherPolicy, "expected to continue on attempt %d", i)
				}
			}
		})
	}
}

func failedToPullMsg(img, container, policy string) string {
	return fmt.Sprintf(`Failed to pull image %q for container %q with policy %q: server down`, img, container, policy)
}

func newPullManagerForTest(t *testing.T, l *mockPullLogger) Manager {
	m := NewPullManager(map[string][]api.PullPolicy{
		buildContainer: {api.PullAlways, api.PullIfNotPresent},
		"helper":       {api.PullAlways, api.PullIfNotPresent},
	}, l)
	require.NotNil(t, m)
	return m
}

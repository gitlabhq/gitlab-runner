//go:build !integration

package pull

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
)

const buildImage = "alpine:latest"

func TestNewPullManager(t *testing.T) {
	m := NewPullManager(map[string][]api.PullPolicy{}, nil)
	assert.NotNil(t, m)
}

func TestGetPullPolicyFor(t *testing.T) {
	m := newPullManagerForTest(t, nil)

	pullPolicy, err := m.GetPullPolicyFor(buildImage)
	assert.NoError(t, err)
	assert.Equal(t, api.PullAlways, pullPolicy)
}

func TestMarkPullFailureFor(t *testing.T) {
	t.Run("fails on fallback with no pull policies", func(t *testing.T) {
		l := new(mockPullLogger)
		defer l.AssertExpectations(t)

		m := NewPullManager(map[string][]api.PullPolicy{}, l)
		require.NotNil(t, m)

		pullPolicy, err := m.GetPullPolicyFor(buildImage)
		assert.NoError(t, err)
		assert.Equal(t, api.PullPolicy(""), pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg(buildImage, ""),
		).Once()
		repeat := m.UpdatePolicyForImage(1, &ImagePullError{Image: buildImage, Message: "server down"})
		assert.False(t, repeat, "UpdatePolicyForImage should return false")

		_, err = m.GetPullPolicyFor(buildImage)
		assert.Error(t, err)
	})

	t.Run("succeeds on fallback with two pull policies", func(t *testing.T) {
		l := new(mockPullLogger)
		defer l.AssertExpectations(t)

		m := newPullManagerForTest(t, l)

		pullPolicy, err := m.GetPullPolicyFor(buildImage)
		assert.NoError(t, err)
		assert.Equal(t, api.PullAlways, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg(buildImage, "Always"),
		).Once()
		l.On(
			"Infoln",
			fmt.Sprintf(`Attempt #2: Trying "IfNotPresent" pull policy for %q image`, buildImage),
		).Once()
		repeat := m.UpdatePolicyForImage(1, &ImagePullError{Image: buildImage, Message: "server down"})
		assert.True(t, repeat, "UpdatePolicyForImage should return true")

		pullPolicy, err = m.GetPullPolicyFor(buildImage)
		assert.NoError(t, err)
		assert.Equal(t, api.PullIfNotPresent, pullPolicy)
	})

	t.Run("succeeds on fallback with multiple images", func(t *testing.T) {
		l := new(mockPullLogger)
		m := newPullManagerForTest(t, l)

		pullPolicy, err := m.GetPullPolicyFor(buildImage)
		assert.NoError(t, err)
		assert.Equal(t, api.PullAlways, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg(buildImage, "Always"),
		).Once()
		l.On(
			"Infoln",
			fmt.Sprintf(`Attempt #2: Trying "IfNotPresent" pull policy for %q image`, buildImage),
		).Once()
		repeat := m.UpdatePolicyForImage(1, &ImagePullError{Image: buildImage, Message: "server down"})
		assert.True(t, repeat, "UpdatePolicyForImage should return true")

		pullPolicy, err = m.GetPullPolicyFor("helper")
		assert.NoError(t, err)
		assert.Equal(t, api.PullAlways, pullPolicy)

		pullPolicy, err = m.GetPullPolicyFor(buildImage)
		assert.NoError(t, err)
		assert.Equal(t, api.PullIfNotPresent, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg("helper", "Always"),
		).Once()
		l.On(
			"Infoln",
			fmt.Sprintf(`Attempt #2: Trying "IfNotPresent" pull policy for %q image`, "helper"),
		).Once()
		repeat = m.UpdatePolicyForImage(1, &ImagePullError{Image: "helper", Message: "server down"})
		assert.True(t, repeat, "UpdatePolicyForImage should return true")

		pullPolicy, err = m.GetPullPolicyFor("helper")
		assert.NoError(t, err)
		assert.Equal(t, api.PullIfNotPresent, pullPolicy)
	})

	t.Run("fails after second fallback", func(t *testing.T) {
		l := new(mockPullLogger)
		m := newPullManagerForTest(t, l)

		pullPolicy, err := m.GetPullPolicyFor(buildImage)
		assert.NoError(t, err)
		assert.Equal(t, api.PullAlways, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg(buildImage, "Always"),
		).Once()
		l.On(
			"Infoln",
			fmt.Sprintf(`Attempt #2: Trying "IfNotPresent" pull policy for %q image`, buildImage),
		).Once()
		repeat := m.UpdatePolicyForImage(1, &ImagePullError{Image: buildImage, Message: "server down"})
		assert.True(t, repeat, "UpdatePolicyForImage should return true")

		pullPolicy, err = m.GetPullPolicyFor(buildImage)
		assert.NoError(t, err)
		assert.Equal(t, api.PullIfNotPresent, pullPolicy)

		l.On(
			"Warningln",
			failedToPullMsg(buildImage, "IfNotPresent"),
		).Once()
		repeat = m.UpdatePolicyForImage(2, &ImagePullError{Image: buildImage, Message: "server down"})
		assert.False(t, repeat, "UpdatePolicyForImage should return false")

		_, err = m.GetPullPolicyFor(buildImage)
		assert.Error(t, err)
	})
}

func TestMultipleImagesConcurrently(t *testing.T) {
	l := new(mockPullLogger)
	defer l.AssertExpectations(t)

	imagePolicies := map[string][]api.PullPolicy{
		"img1": {api.PullAlways, api.PullIfNotPresent, "", api.PullNever},
		"img2": {api.PullIfNotPresent, api.PullNever},
	}

	m := NewPullManager(imagePolicies, l)
	require.NotNil(t, m)

	for img, policies := range imagePolicies {
		t.Run(img, func(t *testing.T) {
			t.Parallel()

			nrOfPolicies := len(policies)
			for i, policy := range policies {
				l.On("Warningln", failedToPullMsg(img, string(policy))).Once()

				curPolicy, err := m.GetPullPolicyFor(img)
				assert.NoError(t, err)
				assert.Equal(t, policy, curPolicy, "expected image %q to currently have the policy %q, but has %q", img, policy, curPolicy)

				nextPolicy := policies[nrOfPolicies-1]
				if i < nrOfPolicies-1 {
					nextPolicy = imagePolicies[img][i+1]
				}
				l.On("Infoln", fmt.Sprintf("Attempt #%d: Trying %q pull policy for %q image", i+1, nextPolicy, img)).Once()

				hasAnotherPolicy := m.UpdatePolicyForImage(i, &ImagePullError{Image: img, Message: "server down"})
				if i == nrOfPolicies-1 {
					assert.False(t, hasAnotherPolicy, "expected to stop on attempt %d", i)
				} else {
					assert.True(t, hasAnotherPolicy, "expected to continue on attempt %d", i)
				}
			}
		})
	}
}

func failedToPullMsg(img, policy string) string {
	return fmt.Sprintf(`Failed to pull image %q with policy %q: server down`, img, policy)
}

func newPullManagerForTest(t *testing.T, l *mockPullLogger) Manager {
	m := NewPullManager(map[string][]api.PullPolicy{
		buildImage: {api.PullAlways, api.PullIfNotPresent},
		"helper":   {api.PullAlways, api.PullIfNotPresent},
	}, l)
	require.NotNil(t, m)
	return m
}

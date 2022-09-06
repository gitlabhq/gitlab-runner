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
	m := NewPullManager([]api.PullPolicy{}, nil)
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

		m := NewPullManager([]api.PullPolicy{}, l)
		require.NotNil(t, m)

		pullPolicy, err := m.GetPullPolicyFor(buildImage)
		assert.NoError(t, err)
		assert.Equal(t, api.PullPolicy(""), pullPolicy)

		l.On("Warningln", `Failed to pull image with policy "": server down`).
			Once()
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

		l.On("Warningln", `Failed to pull image with policy "Always": server down`).
			Once()
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

		l.On("Warningln", `Failed to pull image with policy "Always": server down`).
			Once()
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

		l.On("Warningln", `Failed to pull image with policy "Always": server down`).
			Once()
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

		l.On("Warningln", `Failed to pull image with policy "Always": server down`).
			Once()
		l.On(
			"Infoln",
			fmt.Sprintf(`Attempt #2: Trying "IfNotPresent" pull policy for %q image`, buildImage),
		).Once()
		repeat := m.UpdatePolicyForImage(1, &ImagePullError{Image: buildImage, Message: "server down"})
		assert.True(t, repeat, "UpdatePolicyForImage should return true")

		pullPolicy, err = m.GetPullPolicyFor(buildImage)
		assert.NoError(t, err)
		assert.Equal(t, api.PullIfNotPresent, pullPolicy)

		l.On("Warningln", `Failed to pull image with policy "IfNotPresent": server down`).
			Once()
		repeat = m.UpdatePolicyForImage(2, &ImagePullError{Image: buildImage, Message: "server down"})
		assert.False(t, repeat, "UpdatePolicyForImage should return false")

		_, err = m.GetPullPolicyFor(buildImage)
		assert.Error(t, err)
	})
}

func newPullManagerForTest(t *testing.T, l *mockPullLogger) Manager {
	m := NewPullManager([]api.PullPolicy{api.PullAlways, api.PullIfNotPresent}, l)
	require.NotNil(t, m)
	return m
}

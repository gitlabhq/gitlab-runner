package common

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheCheckPolicy(t *testing.T) {
	for num, tc := range []struct {
		object      CachePolicy
		subject     CachePolicy
		expected    bool
		expectErr   bool
		description string
	}{
		{CachePolicyPullPush, CachePolicyPull, true, false, "pull-push allows pull"},
		{CachePolicyPullPush, CachePolicyPush, true, false, "pull-push allows push"},
		{CachePolicyUndefined, CachePolicyPull, true, false, "undefined allows pull"},
		{CachePolicyUndefined, CachePolicyPush, true, false, "undefined allows push"},
		{CachePolicyPull, CachePolicyPull, true, false, "pull allows pull"},
		{CachePolicyPull, CachePolicyPush, false, false, "pull forbids push"},
		{CachePolicyPush, CachePolicyPull, false, false, "push forbids pull"},
		{CachePolicyPush, CachePolicyPush, true, false, "push allows push"},
		{"unknown", CachePolicyPull, false, true, "unknown raises error on pull"},
		{"unknown", CachePolicyPush, false, true, "unknown raises error on push"},
	} {
		cache := Cache{Policy: tc.object}

		result, err := cache.CheckPolicy(tc.subject)
		if tc.expectErr {
			assert.Errorf(t, err, "case %d: %s", num, tc.description)
		} else {
			assert.NoErrorf(t, err, "case %d: %s", num, tc.description)
		}

		assert.Equal(t, tc.expected, result, "case %d: %s", num, tc.description)
	}
}

func TestShouldCache(t *testing.T) {
	for _, params := range []struct {
		jobSuccess          bool
		when                CacheWhen
		expectedShouldCache bool
	}{
		{true, CacheWhenOnSuccess, true},
		{true, CacheWhenAlways, true},
		{true, CacheWhenOnFailure, false},
		{false, CacheWhenOnSuccess, false},
		{false, CacheWhenAlways, true},
		{false, CacheWhenOnFailure, true},
	} {
		tn := "jobSuccess=" + strconv.FormatBool(params.jobSuccess) + ",when=" + string(params.when)

		t.Run(tn, func(t *testing.T) {
			expected := params.expectedShouldCache

			actual := params.when.ShouldCache(params.jobSuccess)

			assert.Equal(
				t,
				actual,
				expected,
				"Value returned from ShouldCache was not as expected",
			)
		})
	}
}

func TestSecrets_expandVariables(t *testing.T) {
	jobJWT := "job-jwt"
	testRole := "role"

	variables := JobVariables{
		{
			Key:   "CI_JOB_JWT",
			Value: jobJWT,
		},
	}

	tests := map[string]struct {
		secrets       Secrets
		assertSecrets func(t *testing.T, secrets Secrets)
	}{
		"no secrets defined": {
			secrets: nil,
			assertSecrets: func(t *testing.T, secrets Secrets) {
				assert.Nil(t, secrets)
			},
		},
		"nil vault secret": {
			secrets: Secrets{
				"VAULT": Secret{
					Vault: nil,
				},
			},
			assertSecrets: func(t *testing.T, secrets Secrets) {
				assert.Nil(t, secrets["VAULT"].Vault)
			},
		},
		"vault missing data": {
			secrets: Secrets{
				"VAULT": Secret{
					Vault: &VaultSecret{},
				},
			},
			assertSecrets: func(t *testing.T, secrets Secrets) {
				assert.NotNil(t, secrets["VAULT"].Vault)
			},
		},
		"vault missing jwt data": {
			secrets: Secrets{
				"VAULT": Secret{
					Vault: &VaultSecret{
						Server: VaultServer{
							Auth: VaultAuth{
								Data: map[string]interface{}{
									"role": testRole,
								},
							},
						},
					},
				},
			},
			assertSecrets: func(t *testing.T, secrets Secrets) {
				require.NotNil(t, secrets["VAULT"].Vault)
				assert.Equal(t, testRole, secrets["VAULT"].Vault.Server.Auth.Data["role"])
			},
		},
		"vault secret defined": {
			secrets: Secrets{
				"VAULT": Secret{
					Vault: &VaultSecret{
						Server: VaultServer{
							Auth: VaultAuth{
								Path: "path ${CI_JOB_JWT}",
								Data: map[string]interface{}{
									"jwt":  "test ${CI_JOB_JWT}",
									"role": "role ${CI_JOB_JWT}",
								},
							},
						},
					},
				},
			},
			assertSecrets: func(t *testing.T, secrets Secrets) {
				require.NotNil(t, secrets["VAULT"].Vault)
				assert.Equal(
					t,
					fmt.Sprintf("path %s", jobJWT),
					secrets["VAULT"].Vault.Server.Auth.Path,
				)
				assert.Equal(
					t,
					fmt.Sprintf("test %s", jobJWT),
					secrets["VAULT"].Vault.Server.Auth.Data["jwt"],
				)
				assert.Equal(
					t,
					fmt.Sprintf("role %s", jobJWT),
					secrets["VAULT"].Vault.Server.Auth.Data["role"],
				)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.NotPanics(t, func() {
				tt.secrets.expandVariables(variables)
				tt.assertSecrets(t, tt.secrets)
			})
		})
	}
}

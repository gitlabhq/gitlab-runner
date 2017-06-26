package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			assert.Error(t, err, "case %d: %s", num, tc.description)
		} else {
			assert.NoError(t, err, "case %d: %s", num, tc.description)
		}

		assert.Equal(t, tc.expected, result, "case %d: %s", num, tc.description)
	}

}

package buildtest

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

func RunBuildWithMasking(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	resp, err := common.GetRemoteSuccessfulBuildPrintVars(
		config.Shell,
		"MASKED_KEY",
		"CLEARTEXT_KEY",
		"MASKED_KEY_OTHER",
		"URL_MASKED_PARAM",
		"TOKEN_REVEALS",
	)
	require.NoError(t, err)

	resp.Features.TokenMaskPrefixes = []string{"glpat-", "mytoken:", "foobar-"}

	build := &common.Build{
		JobResponse: resp,
		Runner:      config,
	}

	build.Runner.FeatureFlags = map[string]bool{featureflags.UseImprovedURLMasking: true}

	build.Variables = append(
		build.Variables,
		common.JobVariable{Key: "MASKED_KEY", Value: "MASKED_VALUE", Masked: true},
		common.JobVariable{Key: "CLEARTEXT_KEY", Value: "CLEARTEXT_VALUE", Masked: false},
		common.JobVariable{Key: "MASKED_KEY_OTHER", Value: "MASKED_VALUE_OTHER", Masked: true},
		common.JobVariable{Key: "URL_MASKED_PARAM", Value: "https://example.com/?x-amz-credential=foobar"},

		common.JobVariable{Key: "TOKEN_REVEALS", Value: "glpat-abcdef mytoken:ghijklmno foobar-pqrstuvwxyz"},
	)

	if setup != nil {
		setup(build)
	}

	buf, err := trace.New()
	require.NoError(t, err)
	defer buf.Close()

	err = build.Run(&common.Config{}, &common.Trace{Writer: buf})
	assert.NoError(t, err)

	buf.Finish()

	contents, err := buf.Bytes(0, math.MaxInt64)
	assert.NoError(t, err)

	assert.NotContains(t, string(contents), "MASKED_KEY=MASKED_VALUE")
	assert.Contains(t, string(contents), "MASKED_KEY=[MASKED]")

	assert.NotContains(t, string(contents), "MASKED_KEY_OTHER=MASKED_VALUE_OTHER")
	assert.NotContains(t, string(contents), "MASKED_KEY_OTHER=[MASKED]_OTHER")
	assert.Contains(t, string(contents), "MASKED_KEY_OTHER=[MASKED]")

	assert.NotContains(t, string(contents), "CLEARTEXT_KEY=[MASKED]")
	assert.Contains(t, string(contents), "CLEARTEXT_KEY=CLEARTEXT_VALUE")

	assert.NotContains(t, string(contents), "x-amz-credential=foobar")
	assert.Contains(t, string(contents), "x-amz-credential=[MASKED]")

	assert.NotContains(t, string(contents), "glpat-abcdef mytoken:ghijklmno foobar-pqrstuvwxyz")
	assert.Contains(t, string(contents), "glpat-[MASKED] mytoken:[MASKED] foobar-[MASKED]")
}

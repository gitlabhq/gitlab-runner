package buildtest

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

func RunBuildWithMasking(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	testBuildWithMasking(t, config, setup, false)
}

func RunBuildWithMaskingProxyExec(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	testBuildWithMasking(t, config, setup, true)
}

func testBuildWithMasking(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn, proxy bool) {
	config.ProxyExec = &proxy

	resp, err := common.GetRemoteSuccessfulBuildPrintVars(
		config.Shell,
		"MASKED_KEY",
		"CLEARTEXT_KEY",
		"MASKED_KEY_OTHER",
		"URL_MASKED_PARAM",
		"TOKEN_REVEALS",
		"ADD_MASK_SECRET",
	)
	require.NoError(t, err)

	resp.Features.TokenMaskPrefixes = []string{"glpat-", "mytoken:", "foobar-"}

	if proxy {
		resp.Steps = append([]common.Step{
			{
				Name:   "before_script",
				Script: []string{`echo "::add-mask::ADD_MASK_SECRET_VALUE"`},
				When:   common.StepWhenAlways,
			},
		}, resp.Steps...)
	}

	build := &common.Build{
		JobResponse: resp,
		Runner:      config,
	}

	build.Variables = append(
		build.Variables,
		common.JobVariable{Key: "MASKED_KEY", Value: "MASKED_VALUE", Masked: true},
		common.JobVariable{Key: "CLEARTEXT_KEY", Value: "CLEARTEXT_VALUE", Masked: false},
		common.JobVariable{Key: "MASKED_KEY_OTHER", Value: "MASKED_VALUE_OTHER", Masked: true},
		common.JobVariable{Key: "URL_MASKED_PARAM", Value: "https://example.com/?x-amz-credential=foobar"},

		common.JobVariable{Key: "TOKEN_REVEALS", Value: "glpat-abcdef mytoken:ghijklmno foobar-pqrstuvwxyz"},

		// proxy exec masking
		common.JobVariable{Key: "ADD_MASK_SECRET", Value: "ADD_MASK_SECRET_VALUE"},
	)

	if setup != nil {
		setup(t, build)
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

	assert.NotContains(t, string(contents), "glpat-abcdef")
	assert.NotContains(t, string(contents), "mytoken:ghijklmno")
	assert.NotContains(t, string(contents), "foobar-pqrstuvwxyz")
	assert.Contains(t, string(contents), "glpat-[MASKED]")
	assert.Contains(t, string(contents), "mytoken:[MASKED]")
	assert.Contains(t, string(contents), "foobar-[MASKED]")

	if proxy {
		assert.Contains(t, string(contents), "ADD_MASK_SECRET=[MASKED]")
	} else {
		assert.Contains(t, string(contents), "ADD_MASK_SECRET=ADD_MASK_SECRET_VALUE")
	}
}

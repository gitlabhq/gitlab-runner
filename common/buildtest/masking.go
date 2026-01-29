package buildtest

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

func RunBuildWithMasking(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	t.Run("success job", func(t *testing.T) {
		testBuildWithMasking(t, config, setup, false)
	})

	t.Run("failed job (can mask error message)", func(t *testing.T) {
		resp, err := common.GetRemoteFailedBuild()
		require.NoError(t, err)

		// different platforms/executors report the error differently
		masks := []string{
			"Job failed: exit code 1",
			"Job failed: exit status 1",
			"Job failed: run exit (exit code: 1)",
			"Job failed: command terminated with exit code 1",
			"Job failed: step \"user_script\": exec: executing script: exit status 1",
		}

		build := &common.Build{
			Job:    resp,
			Runner: config,
		}

		for idx, mask := range masks {
			build.Variables = append(build.Variables, spec.Variable{Key: fmt.Sprintf("MASK_ERROR_MSG_%d", idx), Value: mask, Masked: true})
		}

		if setup != nil {
			setup(t, build)
		}

		buf, err := trace.New()
		require.NoError(t, err)
		defer buf.Close()

		err = build.Run(&common.Config{}, &common.Trace{Writer: buf})
		assert.Error(t, err)

		buf.Finish()

		contents, err := buf.Bytes(0, math.MaxInt64)
		assert.NoError(t, err)

		for _, mask := range masks {
			assert.NotContains(t, string(contents), mask)
		}
		assert.Contains(t, string(contents), "ERROR: [MASKED]")
	})
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
		resp.Steps = append([]spec.Step{
			{
				Name:   "before_script",
				Script: []string{`echo "::add-mask::ADD_MASK_SECRET_VALUE"`},
				When:   spec.StepWhenAlways,
			},
		}, resp.Steps...)
	}

	build := &common.Build{
		Job:    resp,
		Runner: config,
	}

	build.Variables = append(
		build.Variables,
		spec.Variable{Key: "MASKED_KEY", Value: "MASKED_VALUE", Masked: true},
		spec.Variable{Key: "CLEARTEXT_KEY", Value: "CLEARTEXT_VALUE", Masked: false},
		spec.Variable{Key: "MASKED_KEY_OTHER", Value: "MASKED_VALUE_OTHER", Masked: true},
		spec.Variable{Key: "URL_MASKED_PARAM", Value: "https://example.com/?x-amz-credential=foobar"},

		spec.Variable{Key: "TOKEN_REVEALS", Value: "glpat-abcdef mytoken:ghijklmno foobar-pqrstuvwxyz"},

		// proxy exec masking
		spec.Variable{Key: "ADD_MASK_SECRET", Value: "ADD_MASK_SECRET_VALUE"},
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

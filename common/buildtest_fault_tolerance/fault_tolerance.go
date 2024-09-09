package buildtest

import (
	"context"
	"os"
	"testing"
	"time"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

const shell = "fault-tolerance-shell"

func init() {
	s := common.MockShell{}
	s.On("GetName").Return(shell)
	s.On("IsDefault").Return(false)
	s.On("GenerateScript", mock.Anything, mock.Anything, mock.Anything).Return("script", nil)
	common.RegisterShell(&s)
}

type testFaultToleranceExecutor struct {
	*common.MockExecutor
	*common.MockStatefulExecutor
}

type statefulExecutorState struct{}

func buildMocks(t *testing.T, cfg *common.RunnerConfig) *setupMocksConfig {
	response, err := common.GetSuccessfulBuild()
	require.NoError(t, err)

	store := common.NewMockJobStore(t)

	store.On("Request").Return(nil, nil).Once()

	nw := common.NewMockNetwork(t)
	nw.On("RequestJob", mock.Anything, mock.Anything, mock.Anything).Return(&response, true).Once()
	nw.On("UpdateJob", mock.Anything, mock.Anything, mock.Anything).Return(common.UpdateJobResult{
		State: common.UpdateSucceeded,
	})

	var sentOffset int
	var call *mock.Call
	call = nw.On("PatchTrace", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			content, _ := args.Get(2).([]byte)
			sentOffset += len(content)

			call.ReturnArguments = []any{common.PatchTraceResult{
				SentOffset: sentOffset,
				State:      common.PatchSucceeded,
			}}
		})

	manager := common.NewStatefulJobManager(
		nw,
		store,
		network.ClientJobTraceProviderFunc,
		cfg,
	)

	job, _ := manager.RequestJob(context.Background(), nil)

	trace, err := manager.ProcessJob(&common.JobCredentials{ID: 1})
	require.NoError(t, err)

	store.On("Update", job).Return(nil)
	store.On("Remove", job).Return(nil).Once()

	e := common.NewMockExecutor(t)

	e.On("Shell").Return(&common.ShellScriptInfo{Shell: shell})
	e.On("Prepare", mock.Anything).Return(nil)
	e.On("Finish", mock.Anything)
	e.On("Cleanup")

	statefulExecutor := common.NewMockStatefulExecutor(t)

	state := &statefulExecutorState{}

	statefulExecutor.On("GetState").Return(state).Once()

	executor := &testFaultToleranceExecutor{
		MockExecutor:         e,
		MockStatefulExecutor: statefulExecutor,
	}

	provider := common.NewMockExecutorProvider(t)

	provider.On("CanCreate").Return(true)
	provider.On("Create").Return(executor)
	provider.On("GetDefaultShell").Return("script-shell")
	provider.On("GetFeatures", mock.Anything).Return(nil)

	common.RegisterExecutorProvider(cfg.Executor, provider)

	return &setupMocksConfig{
		cfg:      cfg,
		executor: executor,
		provider: provider,
		trace:    trace,
		store:    store,
		job:      job,
		state:    state,
		manager:  manager,
	}
}

func mockDefaultExecutorBuildStages(executor *common.MockExecutor, stages []common.BuildStage) {
	for _, stage := range stages {
		executor.On("Run", matchBuildStage(stage)).Return(nil).Once()
	}
}

type setupMocksConfig struct {
	cfg      *common.RunnerConfig
	executor *testFaultToleranceExecutor
	provider common.ExecutorProvider
	trace    common.JobTrace
	store    *common.MockJobStore
	job      *common.Job
	manager  *common.StatefulJobManager
	state    *statefulExecutorState
}

func matchBuildStage(buildStage common.BuildStage) interface{} {
	return mock.MatchedBy(func(cmd common.ExecutorCommand) bool {
		return cmd.Stage == buildStage
	})
}

func TestFaultTolerance(t *testing.T) {
	tests := map[string]struct {
		cfg func(t *testing.T) *common.RunnerConfig

		runBuild func(t *testing.T, build *common.Build, mocks *setupMocksConfig)
	}{
		"no restarts": {
			cfg: func(t *testing.T) *common.RunnerConfig {
				cfg := &common.RunnerConfig{}
				cfg.Executor = t.Name()
				cfg.Store = &common.StoreConfig{}

				return cfg
			},

			runBuild: func(t *testing.T, build *common.Build, mocks *setupMocksConfig) {
				executor := mocks.executor.MockExecutor

				mockDefaultExecutorBuildStages(executor, []common.BuildStage{
					common.BuildStagePrepare,
					common.BuildStageGetSources,
					common.BuildStageRestoreCache,
					common.BuildStageDownloadArtifacts,
					"step_script",
					common.BuildStageAfterScript,
					common.BuildStageArchiveOnSuccessCache,
					common.BuildStageUploadOnSuccessArtifacts,
					common.BuildStageCleanup,
				})

				err := build.Run(&common.Config{}, mocks.trace)
				require.NoError(t, err)
			},
		},
		"restart": {
			cfg: func(t *testing.T) *common.RunnerConfig {
				cfg := &common.RunnerConfig{}
				cfg.Executor = t.Name()
				cfg.Store = &common.StoreConfig{}

				return cfg
			},

			runBuild: func(t *testing.T, build *common.Build, mocks *setupMocksConfig) {
				executor := mocks.executor.MockExecutor

				// Will be called by manager.RequestJob after the job is restarted
				// this will mark the job as resumed
				mocks.store.On("Request").Return(mocks.job, nil).Once()

				// all the stages up until step_script will be called the first time the job is ran
				mockDefaultExecutorBuildStages(executor, []common.BuildStage{
					common.BuildStagePrepare,
					common.BuildStageGetSources,
					common.BuildStageRestoreCache,
					common.BuildStageDownloadArtifacts,
				})

				// kill the build in the step_script stage
				// some error logs will be printed, but we don't care about those
				executor.On("Run", matchBuildStage("step_script")).
					Run(func(args mock.Arguments) {
						go func() {
							build.SystemInterrupt <- os.Kill
						}()

						time.Sleep(100 * time.Millisecond)
					}).
					Return(nil).Once()

				err := build.Run(&common.Config{}, mocks.trace)
				require.Error(t, err)

				// the state will be set to failed we set it back
				mocks.job.State.SetBuildState(common.BuildRunRuntimeRunning)

				// these should be called when a job is resumed
				statefulExecutor := mocks.executor.MockStatefulExecutor
				statefulExecutor.On("SetState", mocks.state).Return(true).Once()
				statefulExecutor.On("Resume", mock.Anything).Return(nil).Once()

				_, _ = mocks.manager.RequestJob(context.Background(), nil)

				// when a job is resumed all the previous stages will be skipped
				// step_script will be replaced with a call to Resume
				// after that all the bellow stages should run as normal
				mockDefaultExecutorBuildStages(executor, []common.BuildStage{
					common.BuildStageAfterScript,
					common.BuildStageArchiveOnSuccessCache,
					common.BuildStageUploadOnSuccessArtifacts,
					common.BuildStageCleanup,
				})

				err = build.Run(&common.Config{}, mocks.trace)
				require.NoError(t, err)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := tt.cfg(t)

			mocks := buildMocks(t, cfg)

			build := common.NewBuild(mocks.job, cfg)
			build.JobStore = mocks.store
			build.SystemInterrupt = make(chan os.Signal, 1)

			tt.runBuild(t, build, mocks)
		})
	}
}

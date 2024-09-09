//go:build !integration

package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/testutil"
)

var newTestFileStoreConfig = func(t *testing.T, mutate func(c *common.RunnerConfig)) *common.RunnerConfig {
	c := &common.RunnerConfig{}
	c.Store = &common.StoreConfig{
		File: &common.FileStore{
			Path: testutil.Ptr(t.TempDir()),
		},
	}

	if mutate != nil {
		mutate(c)
	}

	return c
}

type testJobScanner struct {
	called atomic.Bool
	job    *common.Job
}

func (f *testJobScanner) Scan() bool {
	if f.called.Load() {
		return false
	}

	f.called.Store(true)
	return true
}

func (f *testJobScanner) Job() (*common.Job, error) {
	return f.job, nil
}

func TestConcurrentFileJobStore(t *testing.T) {
	job := common.NewJob(&common.JobResponse{ID: 0})

	config := newTestFileStoreConfig(t, nil)

	store, err := FileProvider().Get(config)
	require.NoError(t, err)

	s := store.(*jobFileStore)

	filter := func(j *common.Job) bool {
		assert.Equal(t, job, j)
		return true
	}

	s.canResumeFilter = filter
	s.canDeleteFilter = filter

	s.newJobScanner = func() (jobScanner, error) {
		return &testJobScanner{
			job: job,
		}, nil
	}

	invoke := func(t *testing.T, parentWg *sync.WaitGroup, count int, fn func()) {
		defer parentWg.Done()

		var wg sync.WaitGroup
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				require.NotPanics(t, fn)
			}()
		}

		wg.Wait()
	}

	fns := []func(){
		func() {
			_, _ = s.Request()
		},
		func() {
			_, _ = s.List()
		},
		func() {
			_ = s.Update(job)
		},
		func() {
			_ = s.Remove(job)
		},
	}

	var wg sync.WaitGroup
	for _, fn := range fns {
		wg.Add(1)
		go invoke(t, &wg, 500, fn)
	}

	wg.Wait()
}

func TestJobFileStore(t *testing.T) {
	testErr := errors.New("err")

	simpleTestFilter := func(t *testing.T, expectedJob *common.Job, condition bool) jobFilter {
		return func(job *common.Job) bool {
			assert.Equal(t, expectedJob, job)
			return condition
		}
	}

	tests := map[string]struct {
		mutateConfig   func(c *common.RunnerConfig)
		constructorErr error
		run            func(t *testing.T, s *jobFileStore)
	}{
		"nil path": {
			mutateConfig: func(c *common.RunnerConfig) {
				c.Store.File.Path = nil
			},
			constructorErr: errEmptyStorePath,
		},
		"empty path": {
			mutateConfig: func(c *common.RunnerConfig) {
				c.Store.File.Path = testutil.Ptr("")
			},
			constructorErr: errEmptyStorePath,
		},
		"nil config": {
			mutateConfig: func(c *common.RunnerConfig) {
				c.Store.File = nil
			},
			constructorErr: errEmptyStorePath,
		},
		"file path expanded": {
			mutateConfig: func(c *common.RunnerConfig) {
				c.Environment = []string{"value=test"}
				c.Store.File.Path = testutil.Ptr("test/$value")
			},
			run: func(t *testing.T, s *jobFileStore) {
				require.Equal(t, "test/test", s.storePath)
			},
		},
		"request job new scanner err": {
			run: func(t *testing.T, s *jobFileStore) {
				s.newJobScanner = func() (jobScanner, error) {
					return nil, testErr
				}

				job, err := s.Request()
				assert.ErrorIs(t, err, testErr)
				assert.Nil(t, job)
			},
		},
		"request job ignore err": {
			run: func(t *testing.T, s *jobFileStore) {
				s.newJobScanner = func() (jobScanner, error) {
					j := newMockJobScanner(t)
					j.On("Scan").Return(true).Once()
					j.On("Job").Return(nil, testErr).Once()
					j.On("Scan").Return(false).Once()

					return j, nil
				}

				job, err := s.Request()
				assert.NoError(t, err)
				assert.Nil(t, job)
			},
		},
		"request job": {
			run: func(t *testing.T, s *jobFileStore) {
				expectedJob := common.NewJob(nil)

				s.newJobScanner = func() (jobScanner, error) {
					j := newMockJobScanner(t)
					j.On("Scan").Return(true).Once()
					j.On("Job").Return(expectedJob, nil).Once()

					return j, nil
				}

				s.canResumeFilter = simpleTestFilter(t, expectedJob, true)

				job, err := s.Request()
				assert.NoError(t, err)
				assert.Equal(t, expectedJob, job)
			},
		},
		"request job doesn't pass filter": {
			run: func(t *testing.T, s *jobFileStore) {
				expectedJob := common.NewJob(nil)

				s.newJobScanner = func() (jobScanner, error) {
					j := newMockJobScanner(t)
					j.On("Scan").Return(true).Once()
					j.On("Scan").Return(false).Once()
					j.On("Job").Return(expectedJob, nil).Once()

					return j, nil
				}

				s.canResumeFilter = simpleTestFilter(t, expectedJob, false)

				job, err := s.Request()
				assert.NoError(t, err)
				assert.Nil(t, job)
			},
		},
		"list jobs new scanner err": {
			run: func(t *testing.T, s *jobFileStore) {
				s.newJobScanner = func() (jobScanner, error) {
					return nil, testErr
				}

				jobs, err := s.List()
				assert.ErrorIs(t, err, testErr)
				assert.Empty(t, jobs)
			},
		},
		"list jobs ignore err": {
			run: func(t *testing.T, s *jobFileStore) {
				s.newJobScanner = func() (jobScanner, error) {
					j := newMockJobScanner(t)
					j.On("Scan").Return(true).Once()
					j.On("Job").Return(nil, testErr).Once()
					j.On("Scan").Return(false).Once()

					return j, nil
				}

				jobs, err := s.List()
				assert.NoError(t, err)
				assert.Empty(t, jobs)
			},
		},
		"list jobs empty": {
			run: func(t *testing.T, s *jobFileStore) {
				s.newJobScanner = func() (jobScanner, error) {
					j := newMockJobScanner(t)
					j.On("Scan").Return(false).Once()

					return j, nil
				}

				jobs, err := s.List()
				assert.NoError(t, err)
				assert.Empty(t, jobs)
			},
		},
		"list jobs": {
			run: func(t *testing.T, s *jobFileStore) {
				expectedJob := common.NewJob(nil)

				s.newJobScanner = func() (jobScanner, error) {
					j := newMockJobScanner(t)
					j.On("Scan").Return(true).Once()
					j.On("Scan").Return(false).Once()
					j.On("Job").Return(expectedJob, nil).Once()

					return j, nil
				}

				jobs, err := s.List()
				assert.NoError(t, err)
				assert.Len(t, jobs, 1)
				assert.Equal(t, expectedJob, jobs[0])
			},
		},
		"update": {
			run: func(t *testing.T, s *jobFileStore) {
				job := common.NewJob(&common.JobResponse{ID: 0})

				c := newMockJobCodec(t)
				c.On(
					"Encode",
					mock.AnythingOfType("*os.File"),
					mock.AnythingOfType("*common.Job"),
				).Return(nil).Run(func(args mock.Arguments) {
					w := args.Get(0).(io.Writer)
					_, err := fmt.Fprint(w, "job")
					require.NoError(t, err)
				}).Once()

				c.On(
					"Encode",
					mock.AnythingOfType("*os.File"),
					mock.AnythingOfType("*common.Job"),
				).Return(nil).Run(func(args mock.Arguments) {
					w := args.Get(0).(io.Writer)
					_, err := fmt.Fprint(w, "job1")
					require.NoError(t, err)
				}).Once()

				s.codec = c

				err := s.Update(job)
				require.NoError(t, err)

				entries, err := os.ReadDir(s.storePath)
				require.NoError(t, err)
				require.Len(t, entries, 1)

				file, err := os.ReadFile(filepath.Join(s.storePath, entries[0].Name()))
				require.NoError(t, err)
				require.Equal(t, "job", string(file))

				err = s.Update(job)
				require.NoError(t, err)

				file, err = os.ReadFile(filepath.Join(s.storePath, entries[0].Name()))
				require.NoError(t, err)
				require.Equal(t, "job1", string(file))
			},
		},
		"remove": {
			run: func(t *testing.T, s *jobFileStore) {
				job := common.NewJob(&common.JobResponse{ID: 0})

				err := os.WriteFile(s.filePath(job), []byte("job"), 0600)
				require.NoError(t, err)

				entries, err := os.ReadDir(s.storePath)
				require.NoError(t, err)
				require.Len(t, entries, 1)

				err = s.Remove(job)
				require.NoError(t, err)

				entries, err = os.ReadDir(s.storePath)
				require.NoError(t, err)
				require.Empty(t, entries)
			},
		},
		"remove not existing": {
			run: func(t *testing.T, s *jobFileStore) {
				job := common.NewJob(&common.JobResponse{ID: 0})

				err := s.Remove(job)
				require.NoError(t, err)
			},
		},
		"cleanup job on Request": {
			run: func(t *testing.T, s *jobFileStore) {
				job := common.NewJob(&common.JobResponse{ID: 0})

				s.newJobScanner = func() (jobScanner, error) {
					j := newMockJobScanner(t)
					j.On("Scan").Return(true).Once()
					j.On("Scan").Return(false).Once()
					j.On("Job").Return(job, nil).Once()

					return j, nil
				}

				s.canDeleteFilter = simpleTestFilter(t, job, true)

				err := os.WriteFile(s.filePath(job), []byte("job"), 0600)
				require.NoError(t, err)

				requestedJob, err := s.Request()
				require.NoError(t, err)
				require.Nil(t, requestedJob)

				entries, err := os.ReadDir(s.storePath)
				require.NoError(t, err)
				require.Empty(t, entries)
			},
		},
		"skip invalid job on Request": {
			run: func(t *testing.T, s *jobFileStore) {
				job := common.NewJob(&common.JobResponse{ID: 0})

				s.newJobScanner = func() (jobScanner, error) {
					j := newMockJobScanner(t)
					j.On("Scan").Return(true).Once()
					j.On("Scan").Return(false).Once()
					j.On("Job").Return(nil, errors.New("err")).Once()

					return j, nil
				}

				err := os.WriteFile(s.filePath(job), []byte("job"), 0600)
				require.NoError(t, err)

				requestedJob, err := s.Request()
				require.Nil(t, err)
				require.Nil(t, requestedJob)

				entries, err := os.ReadDir(s.storePath)
				require.NoError(t, err)
				require.Len(t, entries, 1)
			},
		},
		"read files with scanner in order": {
			run: func(t *testing.T, s *jobFileStore) {
				s.newJobScanner = func() (jobScanner, error) {
					codec := newMockJobCodec(t)

					// on Windows t.TempDir() sometimes wouldn't cleanup and would fail the test
					tmpDir, err := os.MkdirTemp("", "")
					require.NoError(t, err)

					fileMatcher := func(expectedFile *os.File) func(f *os.File) bool {
						return func(f *os.File) bool {
							return f.Name() == expectedFile.Name()
						}
					}

					var previousCall *mock.Call
					for i := 0; i < 3; i++ {
						file, err := os.Create(filepath.Join(tmpDir, fmt.Sprintf("%d.state", i)))
						require.NoError(t, err)

						tmpFile, err := os.Create(filepath.Join(tmpDir, fmt.Sprintf("%d.state.tmp", i)))
						require.NoError(t, err)

						firstCall := codec.On(
							"Decode",
							mock.MatchedBy(fileMatcher(file)),
							mock.Anything,
						).Return(nil)
						if previousCall != nil {
							firstCall = firstCall.NotBefore(previousCall)
						}

						previousCall = codec.On(
							"Decode",
							mock.MatchedBy(fileMatcher(tmpFile)),
							mock.Anything,
						).Return(nil).NotBefore(firstCall)
					}

					scanner, err := newFileStoreJobScanner(tmpDir, codec)
					require.NoError(t, err)

					return scanner, nil
				}

				filter := func(job *common.Job) bool {
					return false
				}
				s.canDeleteFilter = filter
				s.canResumeFilter = filter

				requestedJob, err := s.Request()
				require.Nil(t, err)
				require.Nil(t, requestedJob)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			config := newTestFileStoreConfig(t, tt.mutateConfig)

			store, err := FileProvider().Get(config)
			if tt.constructorErr != nil {
				require.ErrorIs(t, err, tt.constructorErr)
				return
			}

			if tt.run != nil {
				tt.run(t, store.(*jobFileStore))
			}
		})
	}
}

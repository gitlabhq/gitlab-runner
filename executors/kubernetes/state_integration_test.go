//go:build integration && kubernetes

package kubernetes_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/store"
	"gitlab.com/gitlab-org/gitlab-runner/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func execInPod(t *testing.T, config *common.KubernetesConfig, command string) {
	client := getTestKubeClusterClient(t)

	pods, err := client.
		CoreV1().
		Pods(ciNamespace).
		List(context.Background(), metav1.ListOptions{
			LabelSelector: labels.Set(config.PodLabels).String(),
		})

	require.NoError(t, err)
	require.NotEmpty(t, pods.Items)

	pod := pods.Items[0].Name
	args := []string{
		"exec",
		"-n", ciNamespace,
		"-c", "build",
		"-it",
		pod,
		"--",
		"sh", "-c", command,
	}
	err = exec.Command("kubectl", args...).Run()
	require.NoError(t, err)
}

func getBuildWithNotifyFile(t *testing.T) (*common.Build, func(signal int)) {
	tmpfile := fmt.Sprintf("/tmp/notify-%d", time.Now().UnixNano())

	script := fmt.Sprintf(`
file_path="%s"
timeout=300
interval=1

touch "$file_path"

elapsed=0

while [[ $elapsed -lt $timeout ]]; do
    if [[ -s "$file_path" ]]; then
        content=$(cat "$file_path")
        if [[ "$content" == "1" ]]; then
			echo "Sleep signal received..."
			rm "$file_path"
			sleep 2
        elif [[ "$content" == "2" ]]; then
			echo "Exit signal, bye..."
			break
        fi
    fi
    sleep $interval
    elapsed=$((elapsed + interval))
    echo "File is empty after ${elapsed}s..."
done
`, tmpfile)

	build := getTestBuildWithImage(t, common.TestAlpineImage, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse(script)
	})

	build.Runner.Store = &common.StoreConfig{
		Name:           store.FileProvider().Name(),
		HealthInterval: testutil.Ptr(1),
		HealthTimeout:  testutil.Ptr(5),
		MaxRetries:     testutil.Ptr(2),
		File: &common.FileStore{
			Path: testutil.Ptr("/tmp/store"),
		},
	}

	fs, err := store.FileProvider().Get(build.Runner)
	require.NoError(t, err)
	build.JobStore = fs

	build.SystemInterrupt = make(chan os.Signal, 1)

	notify := func(signal int) {
		execInPod(t, build.Runner.Kubernetes, fmt.Sprintf("echo %d > %s", signal, tmpfile))
	}

	return build, notify
}

func waitForLogLine(r io.Reader, expr string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if matched, _ := regexp.MatchString(expr, scanner.Text()); matched {
			return
		}
	}
}

// we need to drain the buffer since io.Pipe isn't buffered
// if we don't the writer will block on writing
func drainBuffer(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
	}
}

func assertJobLogIntegrity(t *testing.T, log io.Reader) {
	var counter int
	var sleepSignal bool
	var exitSignal bool
	var succeeded bool

	scanner := bufio.NewScanner(log)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		if strings.Contains(line, "Job succeeded") && sleepSignal && exitSignal {
			succeeded = true
			break
		}

		if line == "File is empty after 1s..." {
			counter++
			continue
		}

		if counter > 0 && !exitSignal {
			if line == "Sleep signal received..." {
				sleepSignal = true
				continue
			}

			if line == "Exit signal, bye..." {
				exitSignal = true
				continue
			}

			if line != fmt.Sprintf("File is empty after %ds...", counter+1) {
				t.Fatalf("Unexpected log line: %q", line)
			}

			counter++
		}
	}

	assert.True(t, counter >= 15, "Expected at least 15 iterations, got %d", counter)
	assert.True(t, sleepSignal, "Expected to receive sleep signal")
	assert.True(t, exitSignal, "Expected to receive exit signal")
	assert.True(t, succeeded, "Expected job to succeed")
}

// TestFaultToleranceStopDuringScript tests the fault tolerance of the runner when the job is stopped during
// the script execution. The runner should be able to stop the job and resume it from the last checkpoint.
// This test simulates running a job. It gives a bit more control over the job execution and the runner
// but takes a little setting up as opposed to simply running a runner as a subprocess and mocking out the network.
func TestFaultToleranceStopDuringScript(t *testing.T) {
	var fullJobLog bytes.Buffer
	build, notify := getBuildWithNotifyFile(t)
	t.Cleanup(func() {
		build.SystemInterrupt <- os.Kill
	})

	r, w := io.Pipe()
	t.Cleanup(func() {
		r.Close()
		w.Close()
	})

	trace := &common.Trace{Writer: io.MultiWriter(w, os.Stdout, &fullJobLog)}

	buildSuspended := make(chan struct{})

	go func() {
		waitForLogLine(r, "File is empty after 3s...")
		notify(1)
		waitForLogLine(r, "Sleep signal received")

		// disable the trace so it doesn't mess with our tests output
		// the sleep in the job script will make sure this gets executed before
		// the job starts printing again
		trace.Disable()

		require.NoError(t, build.JobStore.Update(build.Job))

		close(buildSuspended)
	}()
	go func() {
		err := buildtest.RunBuildWithTrace(t, build, trace)
		require.NoError(t, err)
	}()

	<-buildSuspended

	// wait for the job health check to expire
	time.Sleep(time.Duration(*build.Runner.Store.HealthTimeout) + time.Second)
	job, err := build.JobStore.Request()
	require.NoError(t, err)
	require.NotNil(t, job)

	// usually the job manager is responsible for disabling the job upon resume
	// since we don't have a manager we do it manually just like requesting and resuming manually
	job.State.Resume()
	go func() {
		waitForLogLine(r, "File is empty after 15s...")
		notify(2)
		waitForLogLine(r, "Exit signal, bye...")

		go drainBuffer(r)
	}()

	trace = &common.Trace{Writer: io.MultiWriter(w, os.Stdout, &fullJobLog)}
	trace.Disable()
	newBuild := common.NewBuild(job, build.Runner)
	newBuild.JobStore = build.JobStore
	err = buildtest.RunBuildWithTrace(t, newBuild, trace)
	require.NoError(t, err)

	assertJobLogIntegrity(t, &fullJobLog)
}

type kubernetesRunnerTestPaths struct {
	tmpdir string
	config string
	store  string
	binary string
	log    string
}

func initializePaths(t *testing.T) kubernetesRunnerTestPaths {
	tmpdir := t.TempDir()
	fmt.Println("Test working dir:", tmpdir)

	paths := kubernetesRunnerTestPaths{
		tmpdir: tmpdir,
		config: filepath.Join(tmpdir, "config.toml"),
		store:  filepath.Join(tmpdir, "store"),
		binary: filepath.Join(tmpdir, "runner"),
		log:    filepath.Join(tmpdir, "runner.log"),
	}

	fmt.Printf("Paths: %+v\n", paths)

	return paths
}

func initializeConfig(t *testing.T, storePath, configPath, url string) *common.RunnerConfig {
	config := common.NewConfig()
	config.LogLevel = testutil.Ptr("debug")
	config.CheckInterval = 5

	runner := &common.RunnerConfig{
		Name:  "integration-kubernetes-runner",
		Limit: 1,
		RunnerCredentials: common.RunnerCredentials{
			URL:   url,
			Token: "test",
		},
		RunnerSettings: common.RunnerSettings{
			Executor: "kubernetes",
			Kubernetes: &common.KubernetesConfig{
				Image:     common.TestAlpineImage,
				Namespace: ciNamespace,
			},
			Store: &common.StoreConfig{
				Name:           store.FileProvider().Name(),
				HealthInterval: testutil.Ptr(1),
				HealthTimeout:  testutil.Ptr(5),
				MaxRetries:     testutil.Ptr(99),
				File: &common.FileStore{
					Path: testutil.Ptr(storePath),
				},
			},
		},
	}

	config.Runners = []*common.RunnerConfig{runner}

	fmt.Println("Saving config")
	require.NoError(t, config.SaveConfig(configPath))

	return runner
}

func initializeMockGitlabHTTPServer(t *testing.T, script string, logSink io.Writer) *httptest.Server {
	var responded atomic.Bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		switch r.URL.Path {
		case "/api/v4/jobs/request":
			w.Header().Set("Content-Type", "application/json")

			if !responded.CompareAndSwap(false, true) {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			w.WriteHeader(http.StatusCreated)

			response, err := common.GetRemoteBuildResponse(script)
			require.NoError(t, err)

			responseJSON, err := json.Marshal(response)
			require.NoError(t, err)

			_, err = io.Copy(w, bytes.NewReader(responseJSON))
			require.NoError(t, err)
		case "/api/v4/jobs/0/trace":
			_, err := io.Copy(logSink, r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusAccepted)
		default:

		}
	}))
	t.Cleanup(func() {
		ts.Close()
	})

	return ts
}

func getRunnerCommand(t *testing.T, binaryPath, configPath string, logFile io.Writer) *exec.Cmd {
	cmd := exec.Command(binaryPath, "run", "--config", configPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	return cmd
}

func setupFaultToleranceTest(t *testing.T, jobTicks int) (
	// mixing named and positional return values to make it easier to read
	paths kubernetesRunnerTestPaths,
	log *bytes.Buffer,
	logReader io.Reader,
	logFile *os.File,
) {
	paths = initializePaths(t)

	script := fmt.Sprintf(`for i in $(seq 1 %d); do echo "Time elapsed ${i}s"; sleep 1; done`, jobTicks)

	// jobLog is used to verify the integrity of the job log in the end of the job
	var jobLog bytes.Buffer
	// the pipe reader and writer are used to detect certain events in the job log
	// e.g. X seconds passed or a certain stage is executed, let's send a signal to the job
	r, w := io.Pipe()

	// the test server will respond with a job request and accept the job trace
	// it will also forward the logs so we can use them
	ts := initializeMockGitlabHTTPServer(t, script, io.MultiWriter(&jobLog, w, os.Stdout))

	// create the config.toml
	initializeConfig(t, paths.store, paths.config, ts.URL)

	buildtest.MustBuildBinary("../../", paths.binary)

	// the debugLogFile is used to debug the runner binary's output
	debugLogFile, err := os.OpenFile(paths.log, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	require.NoError(t, err)

	return paths, &jobLog, r, debugLogFile
}

func runCommandInBackground(t *testing.T, paths kubernetesRunnerTestPaths, debugLogFile *os.File, expectProcExitErr bool) *exec.Cmd {
	cmd := getRunnerCommand(t, paths.binary, paths.config, debugLogFile)
	go func() {
		fmt.Printf("Executing %v\n", cmd)

		err := cmd.Run()
		if expectProcExitErr {
			require.EqualError(t, err, "signal: killed")
		}
	}()

	return cmd
}

// TestFaultToleranceApplication tests the fault tolerance of the runner when the job is stopped during
// the script execution. The runner should be able to stop the job and resume it from the last checkpoint.
// This test is similar to TestFaultToleranceStopDuringScript but it runs the runner as a subprocess and
// mocks out the network.
// This is the most real world test for fault tolerance as we simply send SIGKILL to the subprocess at
// a completely random point in time and expect the runner to recover and resume the job.
func TestFaultToleranceApplication(t *testing.T) {
	t.Skip("Skip temporarily")
	assertTicksJobLog := func(t *testing.T, ticks int, log []byte) {
		scanner := bufio.NewScanner(bytes.NewReader(log))
		var foundStart bool
		var index int
		for scanner.Scan() {
			line := scanner.Text()
			if foundStart {
				assert.Contains(t, line, fmt.Sprintf("Time elapsed %ds", index))
				index++
				if index > ticks {
					break
				}
			}

			if strings.Contains(line, "Time elapsed 1s") {
				foundStart = true
				index = 2
			}
		}
	}

	_ = func(t *testing.T, log []byte, lines []string) {
		scanner := bufio.NewScanner(bytes.NewReader(log))

		var found int
		for scanner.Scan() {
			scanLine := scanner.Text()
			for _, line := range lines {
				if strings.Contains(scanLine, line) {
					found++
					break
				}
			}
		}

		assert.Equalf(t, found, len(lines), "Expected to find %d lines, found %d", len(lines), found)
	}

	for tn, tt := range map[string]struct {
		jobTicks       int
		stopConditions []string
		assertJobLogFn func(t *testing.T, log *bytes.Buffer)
		expectSuccess  bool
		// debug is used to run the test with a longer job to debug the runner process
		// it will print the command to attach to the runner process after it has been killed
		debug bool
	}{
		"stop once during script": {
			jobTicks:       10,
			stopConditions: []string{"Time elapsed 5s"},
			assertJobLogFn: func(t *testing.T, log *bytes.Buffer) {
				assertTicksJobLog(t, 10, log.Bytes())
			},
			expectSuccess: true,
		},
		// "stop three times during script": {
		// 	jobTicks:       30,
		// 	stopConditions: []string{"Time elapsed 5s", "Time elapsed 15s", "Time elapsed 25s"},
		// 	assertJobLogFn: func(t *testing.T, log *bytes.Buffer) {
		// 		assertTicksJobLog(t, 30, log.Bytes())
		// 	},
		// 	expectSuccess: true,
		// },
		// // TODO: fail jobs that are stopped before the prepare stage completes
		// //"stop during 'Preparing environment'": {
		// //	jobTicks:       10,
		// //	stopConditions: []string{"Preparing environment"},
		// //	expectSuccess:  false,
		// //	debug:          true,
		// //},
		// "stop during 'Getting source from Git repository'": {
		// 	jobTicks:       10,
		// 	stopConditions: []string{"Getting source from Git repository"},
		// 	assertJobLogFn: func(t *testing.T, log *bytes.Buffer) {
		// 		assertReaderContainsLines(t, log.Bytes(), []string{
		// 			"Fetching changes...",
		// 			"Skipping Git submodules setup",
		// 		})
		// 		assertTicksJobLog(t, 10, log.Bytes())
		// 	},
		// 	expectSuccess: true,
		// },
	} {
		t.Run(tn, func(t *testing.T) {
			if tt.debug {
				tt.jobTicks = 1000
			}

			paths, jobLog, r, debugLogFile := setupFaultToleranceTest(t, tt.jobTicks)
			if tt.debug {
				cmd := getRunnerCommand(t, paths.binary, paths.config, debugLogFile)
				fmt.Println("DEBUG: To attach to job after the runner process has been killed run:", cmd)
			}

			for _, stopCondition := range tt.stopConditions {
				cmd := runCommandInBackground(t, paths, debugLogFile, true)
				waitForLogLine(r, stopCondition)
				require.NoError(t, cmd.Process.Kill())
			}

			var cmd *exec.Cmd
			if !tt.debug {
				cmd = runCommandInBackground(t, paths, debugLogFile, false)
			}

			if tt.expectSuccess {
				waitForLogLine(r, "Job succeeded")
			} else {
				waitForLogLine(r, "Job failed")
			}

			if cmd != nil {
				require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
			}

			if tt.assertJobLogFn != nil {
				tt.assertJobLogFn(t, jobLog)
			}
		})
	}
}

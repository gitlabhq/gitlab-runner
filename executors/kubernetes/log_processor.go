package kubernetes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type logStreamProvider interface {
	LogStream(lineIndex uint64, output io.Writer) error
	String() string
}

type kubernetesLogStreamProvider struct {
	client       *kubernetes.Clientset
	clientConfig *restclient.Config
	namespace    string
	pod          string
	container    kubernetesLogProcessorContainer
}

func (s *kubernetesLogStreamProvider) LogStream(lineIndex uint64, output io.Writer) error {
	exec := ExecOptions{
		Namespace:     s.namespace,
		PodName:       s.pod,
		ContainerName: s.container.name,
		Stdin:         false,
		Command: []string{
			"gitlab-runner-helper",
			"read-logs",
			"--path",
			s.container.logPath,
			"--index",
			fmt.Sprint(lineIndex),
		},
		Out:      output,
		Err:      output,
		Executor: &DefaultRemoteExecutor{},
		Client:   s.client,
		Config:   s.clientConfig,
	}

	return exec.Run()
}

func (s *kubernetesLogStreamProvider) String() string {
	return fmt.Sprintf("%s/%s/%s", s.namespace, s.pod, s.container)
}

type logProcessor interface {
	// Listen listens for log lines
	// consumers should read from the channel until it's closed
	// otherwise, risk leaking goroutines
	Listen(ctx context.Context) <-chan string
}

// kubernetesLogProcessor processes log from multiple containers in a pod and sends them out through one channel.
// It also tries to reattach to the log constantly, stopping only when the passed context is cancelled.
type kubernetesLogProcessor struct {
	backoff      backoff.Backoff
	logger       *common.BuildLogger
	logProviders []logStreamProvider
}

type kubernetesLogProcessorContainer struct {
	name    string
	logPath string
}

type kubernetesLogProcessorPodConfig struct {
	namespace  string
	pod        string
	containers []kubernetesLogProcessorContainer
}

func newKubernetesLogProcessor(
	client *kubernetes.Clientset,
	clientConfig *restclient.Config,
	backoff backoff.Backoff,
	logger *common.BuildLogger,
	podCfg kubernetesLogProcessorPodConfig,
) logProcessor {
	logProviders := make([]logStreamProvider, len(podCfg.containers))
	for i, container := range podCfg.containers {
		logProviders[i] = &kubernetesLogStreamProvider{
			client:       client,
			clientConfig: clientConfig,
			namespace:    podCfg.namespace,
			pod:          podCfg.pod,
			container:    container,
		}
	}

	return &kubernetesLogProcessor{
		backoff:      backoff,
		logger:       logger,
		logProviders: logProviders,
	}
}

func (l *kubernetesLogProcessor) Listen(ctx context.Context) <-chan string {
	outCh := make(chan string)

	var wg sync.WaitGroup
	for _, logProvider := range l.logProviders {
		wg.Add(1)
		go func(logProvider logStreamProvider) {
			defer wg.Done()
			l.attach(ctx, logProvider, outCh)
		}(logProvider)
	}

	go func() {
		wg.Wait()
		close(outCh)
	}()

	return outCh
}

func (l *kubernetesLogProcessor) attach(ctx context.Context, logProvider logStreamProvider, outputCh chan string) {
	var attempt int32
	var lineIndex uint64

	for {
		select {
		// If we have to exit, check for that before trying to (re)attach
		case <-ctx.Done():
			return
		default:
		}

		if attempt > 0 {
			backoffDuration := l.backoff.ForAttempt(float64(attempt))
			l.logger.Debugln(fmt.Sprintf("Backing off reattaching log for %s for %s", logProvider, backoffDuration))
			time.Sleep(backoffDuration)
		}

		attempt++

		logs, writer := io.Pipe()
		errCh := make(chan error)
		go func() {
			err := logProvider.LogStream(lineIndex, writer)
			if err != nil {
				errCh <- fmt.Errorf("attaching to log %s: %w", logProvider, err)
			}
		}()

		doneCh := make(chan struct{})
		go func() {
			err := l.readLogs(ctx, logs, &lineIndex, outputCh)
			if err != nil {
				errCh <- fmt.Errorf("reading log for %s: %w", logProvider, err)
			}

			doneCh <- struct{}{}
		}()

		// If we succeed in connecting to the stream, set the attempts to 1, so that next time we try to reconnect
		// as soon as possible but also still have some delay, so we don't bombard kubernetes with requests in case
		// readLogs fails too frequently
		attempt = 1

		outputLines := make(chan string)
	a:
		for {
			select {
			case line := <-outputLines:
				outputCh <- line
			case err := <-errCh:
				l.logger.Warningln(fmt.Sprintf("Error %s. Retrying...", err))
				break a
			case <-doneCh:
				break a
			}
		}

		err := logs.Close()
		if err != nil {
			l.logger.Warningln(fmt.Sprintf("Error when closing Kubernetes log stream for %s. %v", logProvider, err))
		}
	}
}

func (l *kubernetesLogProcessor) readLogs(
	ctx context.Context, logs io.Reader, lineIndex *uint64, outputCh chan string,
) error {
	logsScanner, linesCh := l.scan(ctx, logs)

	for {
		select {
		case <-ctx.Done():
			return nil
		case line, more := <-linesCh:
			if !more {
				return logsScanner.Err()
			}

			newLineIndex, logLine := l.parseLogLine(line)
			if newLineIndex < *lineIndex {
				continue
			}

			*lineIndex = newLineIndex
			outputCh <- logLine
		}
	}
}

func (l *kubernetesLogProcessor) scan(ctx context.Context, logs io.Reader) (*bufio.Scanner, <-chan string) {
	logsScanner := bufio.NewScanner(logs)

	linesCh := make(chan string)
	go func() {
		defer close(linesCh)

		// This goroutine will exit when the calling method closes the logs stream or the context is cancelled
		for logsScanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case linesCh <- logsScanner.Text():
			}
		}
	}()

	return logsScanner, linesCh
}

// Each line starts with an RFC3339Nano formatted date. We need this date to resume the log from that point
// if we detach for some reason. The format is "2020-01-30T16:28:25.479904159Z log line continues as normal
// also the line doesn't include the "\n" at the end.
func (l *kubernetesLogProcessor) parseLogLine(line string) (uint64, string) {
	if len(line) == 0 {
		return 0, ""
	}

	// Get the index where the lineNumber ends and parse it
	lineNumberIndex := strings.Index(line, " ")

	// This should not happen but in case there's no space in the log try to parse them all as a lineNumber
	// this way we could at least get an error without going out of the bounds of the line
	var lineNumber string
	if lineNumberIndex > -1 {
		lineNumber = line[:lineNumberIndex]
	} else {
		lineNumber = line
	}

	parsedNumber, err := strconv.ParseUint(lineNumber, 10, 64)
	if err != nil {
		return 0, line
	}

	// We are sure this will never get out of bounds since we know that kubernetes always inserts a
	// lineNumber and space directly after. So if we get an empty log line, this slice will be simply empty
	logLine := line[lineNumberIndex+1:]

	return parsedNumber, logLine
}

package kubernetes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

type logStreamer interface {
	Stream(ctx context.Context, offset int64, output io.Writer) error
	fmt.Stringer
}

type kubernetesLogStreamer struct {
	kubernetesLogProcessorPodConfig

	client       *kubernetes.Clientset
	clientConfig *restclient.Config
	executor     RemoteExecutor
}

func (s *kubernetesLogStreamer) Stream(ctx context.Context, offset int64, output io.Writer) error {
	exec := ExecOptions{
		Namespace:     s.namespace,
		PodName:       s.pod,
		ContainerName: s.container,
		Stdin:         false,
		Command: []string{
			"gitlab-runner-helper",
			"read-logs",
			"--path",
			s.logPath,
			"--offset",
			strconv.FormatInt(offset, 10),
			"--wait-file-timeout",
			s.waitLogFileTimeout.String(),
		},
		Out:      output,
		Err:      output,
		Executor: s.executor,
		Client:   s.client,
		Config:   s.clientConfig,
	}

	return exec.executeRequest(ctx)
}

func (s *kubernetesLogStreamer) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", s.namespace, s.pod, s.container, s.logPath)
}

type logProcessor interface {
	// Process listens for log lines
	// consumers must read from the channel until it's closed
	Process(ctx context.Context, outCh chan string)
}

type logger interface {
	Debugln(args ...interface{})
	Warningln(args ...interface{})
}

type backoffCalculator interface {
	ForAttempt(attempt float64) time.Duration
}

// kubernetesLogProcessor processes the logs from a container and tries to reattach
// to the stream constantly, stopping only when the passed context is cancelled.
type kubernetesLogProcessor struct {
	backoff     backoffCalculator
	logger      logger
	logStreamer logStreamer

	logsOffset int64
}

type kubernetesLogProcessorPodConfig struct {
	namespace          string
	pod                string
	container          string
	logPath            string
	waitLogFileTimeout time.Duration
}

func newKubernetesLogProcessor(
	client *kubernetes.Clientset,
	clientConfig *restclient.Config,
	backoff backoffCalculator,
	logger logger,
	podCfg kubernetesLogProcessorPodConfig,
) *kubernetesLogProcessor {
	logStreamer := &kubernetesLogStreamer{
		kubernetesLogProcessorPodConfig: podCfg,
		client:                          client,
		clientConfig:                    clientConfig,
		executor:                        new(DefaultRemoteExecutor),
	}

	return &kubernetesLogProcessor{
		backoff:     backoff,
		logger:      logger,
		logStreamer: logStreamer,
	}
}

func (l *kubernetesLogProcessor) Process(ctx context.Context, outCh chan string) {
	defer close(outCh)
	l.attach(ctx, outCh)
}

func (l *kubernetesLogProcessor) attach(ctx context.Context, outputCh chan string) {
	var attempt float64 = -1

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		attempt++
		if attempt > 0 {
			backoffDuration := l.backoff.ForAttempt(attempt)
			l.logger.Debugln(fmt.Sprintf("Backing off reattaching log for %s for %s", l.logStreamer, backoffDuration))
			time.Sleep(backoffDuration)
		}

		err := l.processStream(ctx, outputCh)
		if err != nil {
			l.logger.Warningln(fmt.Sprintf("Error %s. Retrying...", err))
		}
	}
}

func (l *kubernetesLogProcessor) processStream(ctx context.Context, outputCh chan string) error {
	reader, writer := io.Pipe()
	defer func() {
		_ = reader.Close()
		_ = writer.Close()
	}()

	// Using errgroup.WithContext doesn't work here since if either one of the goroutines
	// exits with a nil error, we can't signal the other one to exit
	ctx, cancel := context.WithCancel(ctx)

	var gr errgroup.Group

	gr.Go(func() error {
		defer cancel()

		err := l.logStreamer.Stream(ctx, l.logsOffset, writer)
		if err != nil {
			err = fmt.Errorf("streaming logs %s: %w", l.logStreamer, err)
		}

		return err
	})

	gr.Go(func() error {
		defer cancel()

		err := l.readLogs(ctx, reader, outputCh)
		if err != nil {
			err = fmt.Errorf("reading logs %s: %w", l.logStreamer, err)
		}

		return err
	})

	return gr.Wait()
}

func (l *kubernetesLogProcessor) readLogs(ctx context.Context, logs io.Reader, outputCh chan string) error {
	logsScanner, linesCh := l.scan(ctx, logs)

	for {
		select {
		case <-ctx.Done():
			return nil
		case line, more := <-linesCh:
			if !more {
				return logsScanner.Err()
			}

			newLogsOffset, logLine := l.parseLogLine(line)
			if newLogsOffset != -1 {
				l.logsOffset = newLogsOffset
			}

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

// Each line starts with its bytes offset. We need this to resume the log from that point
// if we detach for some reason. The format is "10 log line continues as normal".
// The line doesn't include the new line character.
// Lines without offset are acceptable and return -1 for offset.
func (l *kubernetesLogProcessor) parseLogLine(line string) (int64, string) {
	if len(line) == 0 {
		return -1, ""
	}

	offsetIndex := strings.Index(line, " ")
	if offsetIndex == -1 {
		return -1, line
	}

	offset := line[:offsetIndex]
	parsedOffset, err := strconv.ParseInt(offset, 10, 64)
	if err != nil {
		return -1, line
	}

	logLine := line[offsetIndex+1:]
	return parsedOffset, logLine
}

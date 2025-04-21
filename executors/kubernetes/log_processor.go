package kubernetes

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

type logStreamer interface {
	Stream(ctx context.Context, offset int64, output io.Writer) error
	fmt.Stringer
}

type logScanner struct {
	reader *bufio.Reader
	err    error
}

// Err returns the first non-EOF error that was encountered by the Scanner.
func (ls *logScanner) Err() error {
	if ls.err == io.EOF {
		return nil
	}
	return ls.err
}

type kubernetesLogStreamer struct {
	kubernetesLogProcessorPodConfig

	client       kubernetes.Interface
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
		Out:        output,
		Err:        output,
		Executor:   s.executor,
		KubeClient: s.client,
		Config:     s.clientConfig,

		Context: ctx,
	}

	return exec.executeRequest()
}

func (s *kubernetesLogStreamer) String() string {
	return fmt.Sprintf("%s/%s/%s:%s", s.namespace, s.pod, s.container, s.logPath)
}

type logLineData struct {
	line   string
	offset int64
}

type logProcessor interface {
	// Process listens for log lines
	// consumers must read from the channel until it's closed
	// consumers are also notified in case of error through the error channel
	Process(ctx context.Context) (<-chan logLineData, <-chan error)
	// Finalize waits for all Goroutines called in Process() to finish.
	Finalize()
}

type backoffCalculator interface {
	ForAttempt(attempt float64) time.Duration
}

// kubernetesLogProcessor processes the logs from a container and tries to reattach
// to the stream constantly, stopping only when the passed context is cancelled.
type kubernetesLogProcessor struct {
	backoff     backoffCalculator
	logger      logrus.FieldLogger
	logStreamer logStreamer
	wg          sync.WaitGroup

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
	client kubernetes.Interface,
	clientConfig *restclient.Config,
	backoff backoffCalculator,
	logger logrus.FieldLogger,
	offset int64,
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
		logsOffset:  offset,
	}
}

func (l *kubernetesLogProcessor) Process(ctx context.Context) (<-chan logLineData, <-chan error) {
	outCh := make(chan logLineData)
	errCh := make(chan error)
	go func() {
		defer close(outCh)
		defer close(errCh)
		l.attach(ctx, outCh, errCh)
	}()

	return outCh, errCh
}

func (l *kubernetesLogProcessor) Finalize() {
	l.wg.Wait()
}

func (l *kubernetesLogProcessor) attach(ctx context.Context, outCh chan logLineData, errCh chan error) {
	var (
		attempt         float64 = -1
		backoffDuration time.Duration
	)

	for {
		// We do not exit because we need the processLogs goroutine still running.
		// Once the error message is sent, a new step cleanup variables is started.
		// As the pod is still running, the processLogs goroutine is not launched anymore.
		// This is why, even though the error is sent to fail the ongoing step,
		// we keep trying to reconnect to the output log, as a new one is created for variables cleanup.
		attempt++
		if attempt > 0 {
			backoffDuration = l.backoff.ForAttempt(attempt)
			l.logger.Debugln(fmt.Sprintf(
				"Backing off reattaching log for %s for %s (attempt %f)",
				l.logStreamer,
				backoffDuration,
				attempt,
			))
		}

		select {
		case <-ctx.Done():
			l.logger.Debugln(fmt.Sprintf("Detaching from log... %v", ctx.Err()))
			return
		case <-time.After(backoffDuration):
			err := l.processStream(ctx, outCh)
			exitCode := getExitCode(err)
			switch {
			case exitCode == outputLogFileNotExistsExitCode:
				// The cleanup variables step recreates a new output.log file
				// where the shells.TrapCommandExitStatus is written.
				// To not miss this line, we need to have the offset reset when we reconnect to the newly created log
				l.logsOffset = 0
				errCh <- fmt.Errorf("output log file deleted, cannot continue %w", err)
			case err != nil:
				l.logger.Warningln(fmt.Sprintf("Error %v. Retrying...", err))
			default:
				l.logger.Debug("processStream exited with no error")
			}
		}
	}
}

func (l *kubernetesLogProcessor) processStream(ctx context.Context, outCh chan logLineData) error {
	reader, writer := io.Pipe()
	defer func() {
		_ = reader.Close()
		_ = writer.Close()
	}()

	// Using errgroup.WithContext doesn't work here since if either one of the goroutines
	// exits with a nil error, we can't signal the other one to exit
	ctx, cancel := context.WithCancel(ctx)

	var gr errgroup.Group

	logsOffset := l.logsOffset
	gr.Go(func() error {
		defer cancel()

		err := l.logStreamer.Stream(ctx, logsOffset, writer)
		// prevent printing an error that the container exited
		// when the context is already cancelled
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil
		}

		if err != nil {
			err = fmt.Errorf("streaming logs %s: %w", l.logStreamer, err)
		}

		return err
	})

	gr.Go(func() error {
		defer cancel()

		err := l.readLogs(ctx, reader, outCh)
		if err != nil {
			err = fmt.Errorf("reading logs %s: %w", l.logStreamer, err)
		}

		return err
	})

	return gr.Wait()
}

func (l *kubernetesLogProcessor) readLogs(ctx context.Context, logs io.Reader, outCh chan logLineData) error {
	var previousLogsOffset int64 = -1
	logsScanner, linesCh := l.scan(ctx, logs)

	for {
		select {
		case <-ctx.Done():
			return nil
		case line, more := <-linesCh:
			if !more {
				l.logger.Debug("No more data in linesCh")
				return logsScanner.Err()
			}

			newLogsOffset, logLine := l.parseLogLine(line)
			if newLogsOffset != -1 {
				l.logsOffset = newLogsOffset
			}

			// Helper when reading the log add a new line when the buffer doesn't end with a new line
			// This makes the buffer size greater than the log offset shift
			// When the buffer size is greater than the log offset shift and the additional character is a \n
			// it can then be safely removed as it is likely the addition character added by helper
			if l := len(logLine); previousLogsOffset != -1 &&
				l > int(newLogsOffset-previousLogsOffset) && logLine[l-1] == '\n' {
				logLine = logLine[:l-1]
			}

			previousLogsOffset = newLogsOffset

			outCh <- logLineData{
				line:   logLine,
				offset: l.logsOffset,
			}
		}
	}
}

func (l *kubernetesLogProcessor) scan(ctx context.Context, logs io.Reader) (*logScanner, <-chan string) {
	logsScanner := &logScanner{
		reader: bufio.NewReaderSize(logs, bufio.MaxScanTokenSize),
		err:    nil,
	}

	linesCh := make(chan string)
	l.wg.Add(1)

	go func() {
		defer l.wg.Done()
		defer close(linesCh)

		// This goroutine will exit when the calling method closes the logs stream or the context is cancelled
		for {
			data, err := logsScanner.reader.ReadString('\n')
			if err != nil {
				logsScanner.err = err
				break
			}

			select {
			case <-ctx.Done():
				return
			case linesCh <- data:
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
	if line == "" {
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

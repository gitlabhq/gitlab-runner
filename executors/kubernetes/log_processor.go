package kubernetes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type logStreamProvider interface {
	LogStream(since *time.Time) (io.ReadCloser, error)
	String() string
}

type kubernetesLogStreamProvider struct {
	client    *kubernetes.Clientset
	namespace string
	pod       string
	container string
}

func (s *kubernetesLogStreamProvider) LogStream(since *time.Time) (io.ReadCloser, error) {
	var sinceTime metav1.Time
	if since != nil {
		sinceTime = metav1.NewTime(*since)
	}

	return s.client.
		CoreV1().
		Pods(s.namespace).
		GetLogs(s.pod, &api.PodLogOptions{
			Container:  s.container,
			SinceTime:  &sinceTime,
			Follow:     true,
			Timestamps: true,
		}).Stream()
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

type timestampsSet map[int64]struct{}

// kubernetesLogProcessor processes log from multiple containers in a pod and sends them out through one channel.
// It also tries to reattach to the log constantly, stopping only when the passed context is cancelled.
type kubernetesLogProcessor struct {
	backoff      backoff.Backoff
	logger       *common.BuildLogger
	logProviders []logStreamProvider
}

type kubernetesLogProcessorPodConfig struct {
	namespace  string
	pod        string
	containers []string
}

func newKubernetesLogProcessor(
	client *kubernetes.Clientset,
	backoff backoff.Backoff,
	logger *common.BuildLogger,
	podCfg kubernetesLogProcessorPodConfig,
) logProcessor {
	logProviders := make([]logStreamProvider, len(podCfg.containers))
	for i, container := range podCfg.containers {
		logProviders[i] = &kubernetesLogStreamProvider{
			client:    client,
			namespace: podCfg.namespace,
			pod:       podCfg.pod,
			container: container,
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
	var sinceTime time.Time
	var attempt int32

	processedTimestamps := timestampsSet{}

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

		logs, err := logProvider.LogStream(&sinceTime)
		if err != nil {
			l.logger.Warningln(fmt.Sprintf("Error attaching to log %s: %s. Retrying...", logProvider, err))
			continue
		}

		// If we succeed in connecting to the stream, set the attempts to 1, so that next time we try to reconnect
		// as soon as possible but also still have some delay, so we don't bombard kubernetes with requests in case
		// readLogs fails too frequently
		attempt = 1

		sinceTime, err = l.readLogs(ctx, logs, processedTimestamps, sinceTime, outputCh)
		if err != nil {
			l.logger.Warningln(fmt.Sprintf("Error reading log for %s: %s. Retrying...", logProvider, err))
		}

		err = logs.Close()
		if err != nil {
			l.logger.Warningln(fmt.Sprintf("Error when closing Kubernetes log stream for %s. %v", logProvider, err))
		}
	}
}

func (l *kubernetesLogProcessor) readLogs(
	ctx context.Context, logs io.Reader, timestamps timestampsSet,
	sinceTime time.Time, outputCh chan string,
) (time.Time, error) {
	logsScanner, linesCh := l.scan(ctx, logs)

	for {
		select {
		case <-ctx.Done():
			return sinceTime, nil
		case line, more := <-linesCh:
			if !more {
				return sinceTime, logsScanner.Err()
			}

			newSinceTime, logLine, parseErr := l.parseLogLine(line)
			if parseErr != nil {
				return sinceTime, parseErr
			}

			// Cache log lines based on their timestamp. Since the reattaching precision of kubernetes logs is seconds
			// we need to make sure that we won't process a line twice in case we reattach and get it again
			// The size of the int64 key is 8 bytes and the empty struct is 0. Even with a million logs we should be fine
			// using only 8 MB of memory.
			// Since there's a network delay before a log line is processed by kubernetes itself,
			// it's impossible to get two log lines with the same timestamp
			timeUnix := newSinceTime.UnixNano()
			_, alreadyProcessed := timestamps[timeUnix]
			if alreadyProcessed {
				continue
			}
			timestamps[timeUnix] = struct{}{}

			sinceTime = newSinceTime
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
// if we detach for some reason. The format is "2020-01-30T16:28:25.479904159Z log line continues as normal"
// also the line doesn't include the "\n" at the end.
func (l *kubernetesLogProcessor) parseLogLine(line string) (time.Time, string, error) {
	if len(line) == 0 {
		return time.Time{}, "", fmt.Errorf("empty line: %w", io.EOF)
	}

	// Get the index where the date ends and parse it
	dateEndIndex := strings.Index(line, " ")

	// This should not happen but in case there's no space in the log try to parse them all as a date
	// this way we could at least get an error without going out of the bounds of the line
	var date string
	if dateEndIndex > -1 {
		date = line[:dateEndIndex]
	} else {
		date = line
	}

	parsedDate, err := time.Parse(time.RFC3339Nano, date)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid log timestamp: %w", err)
	}

	// We are sure this will never get out of bounds since we know that kubernetes always inserts a
	// date and space directly after. So if we get an empty log line, this slice will be simply empty
	logLine := line[dateEndIndex+1:]

	return parsedDate, logLine, nil
}

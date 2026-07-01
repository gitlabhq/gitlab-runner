//go:build !integration

package snowplow

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/usage_log"
	labkitsnowplow "gitlab.com/gitlab-org/labkit/v2/events/snowplow"
)

func TestWriter_Store(t *testing.T) {
	bodyCh := make(chan string, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyCh <- string(bodyBytes)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	doneCh := make(chan struct{})
	w := newTestWriter(t, server.URL, labkitsnowplow.WithCallback(func(successes, failures []labkitsnowplow.SendResult) {
		if len(successes) > 0 {
			close(doneCh)
		}
	}))
	defer func() {
		assert.NoError(t, w.Close())
	}()

	record := testRecord()
	err := w.Store(record)
	require.NoError(t, err)

	// Wait for async send
	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for event send")
	}

	// Read and verify the body
	var body string
	select {
	case body = <-bodyCh:
	default:
		t.Fatal("Expected a request body")
	}

	var payload map[string]interface{}
	err = json.Unmarshal([]byte(body), &payload)
	require.NoError(t, err)

	// The payload should have a "data" array with our event
	data, ok := payload["data"].([]interface{})
	require.True(t, ok, "Expected 'data' array in payload")
	require.Greater(t, len(data), 0, "Expected at least one event in data array")

	event := data[0].(map[string]interface{})

	// Verify this is a structured event (billing events use "se" type)
	assert.Equal(t, "se", event["e"], "Expected structured event type")
	assert.NotEmpty(t, event["eid"], "Expected event ID")
	assert.NotEmpty(t, event["dtm"], "Expected device timestamp")
	assert.Equal(t, "gitlab-runner", event["aid"], "Expected app ID")

	// Verify structured event fields
	assert.Equal(t, "runner_compute_usage", event["se_ca"], "Expected category")
	assert.Equal(t, "runner_compute_usage", event["se_ac"], "Expected action/event type")
	assert.NotEmpty(t, event["se_la"], "Expected label (event ID)")
	assert.NotEmpty(t, event["se_va"], "Expected value (quantity)")

	// Verify billing context is present (base64 encoded)
	cx, ok := event["cx"].(string)
	require.True(t, ok, "Expected cx (contexts) field")

	cxBytes, err := base64.StdEncoding.DecodeString(cx)
	require.NoError(t, err)

	var contextsWrapper map[string]interface{}
	err = json.Unmarshal(cxBytes, &contextsWrapper)
	require.NoError(t, err)

	// Verify schema
	assert.Equal(t, "iglu:com.snowplowanalytics.snowplow/contexts/jsonschema/1-0-1", contextsWrapper["schema"])

	// Get billing context from array
	contextData, ok := contextsWrapper["data"].([]interface{})
	require.True(t, ok, "Expected contexts data array")
	require.Greater(t, len(contextData), 0)

	billingCtxWrapper, ok := contextData[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "iglu:com.gitlab/billable_usage/jsonschema/1-0-2", billingCtxWrapper["schema"])

	billingData, ok := billingCtxWrapper["data"].(map[string]interface{})
	require.True(t, ok, "Expected billing event data")

	// Verify billing fields
	assert.Equal(t, "runner_compute_usage", billingData["event_type"])
	assert.Equal(t, "seconds", billingData["unit_of_measure"])
	assert.Equal(t, "SM", billingData["realm"])
	assert.Equal(t, float64(123.45), billingData["quantity"])
	assert.Equal(t, "runner-uuid", billingData["subject"])
	assert.Equal(t, "dedicated-instance-abc123", billingData["instance_id"])
	assert.Equal(t, "unique-dedicated-instance-abc123", billingData["unique_instance_id"])
	assert.Equal(t, float64(42), billingData["project_id"])
	assert.Equal(t, float64(123), billingData["namespace_id"])
	assert.Equal(t, float64(100), billingData["root_namespace_id"])
	assert.Equal(t, float64(1), billingData["organization_id"])

	// Verify metadata
	metadata, ok := billingData["metadata"].(map[string]interface{})
	require.True(t, ok, "Expected metadata object")
	assert.Equal(t, "success", metadata["job_status"])
	assert.Equal(t, "", metadata["job_failure_reason"])
	assert.Equal(t, "docker", metadata["executor"])
	assert.NotEmpty(t, metadata["started_at"])
	assert.NotEmpty(t, metadata["finished_at"])
}

func TestWriter_Store_ClosedStorage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w := newTestWriter(t, server.URL)

	err := w.Close()
	require.NoError(t, err)

	record := testRecord()
	err = w.Store(record)
	assert.ErrorIs(t, err, labkitsnowplow.ErrEmitterStopped)
}

func TestWriter_Store_SkipsConfiguredFailureReason(t *testing.T) {
	requestCh := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestCh <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w, err := New(
		logrus.New(),
		server.URL,
		WithAppID("gitlab-runner"),
		WithSkipFailureReason("runner_system_failure"),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, w.Close())
	}()

	record := testRecord()
	record.Job.Status = "failed"
	record.Job.FailureReason = "runner_system_failure"

	err = w.Store(record)
	require.NoError(t, err)

	select {
	case <-requestCh:
		t.Fatal("Expected matching failure_reason record to be dropped, but a request was sent")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestWriter_Store_ForwardsByDefault(t *testing.T) {
	doneCh := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w, err := New(
		logrus.New(),
		server.URL,
		WithAppID("gitlab-runner"),
		WithCallback(func(successes, failures []labkitsnowplow.SendResult) {
			if len(successes) > 0 {
				select {
				case <-doneCh:
				default:
					close(doneCh)
				}
			}
		}),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, w.Close())
	}()

	record := testRecord()
	record.Job.Status = "failed"
	record.Job.FailureReason = "runner_system_failure"

	err = w.Store(record)
	require.NoError(t, err)

	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for event send")
	}
}

func TestWriter_Close_Idempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w := newTestWriter(t, server.URL)

	err := w.Close()
	assert.NoError(t, err)

	// Second close should be safe
	err = w.Close()
	assert.NoError(t, err)
}

func TestWriter_BuildBillingInputs(t *testing.T) {
	assertDefaults := func(t *testing.T, required labkitsnowplow.BillingEventRequiredInput, optional labkitsnowplow.BillingEventOptionalInput) {
		// Verify required fields
		assert.Equal(t, "runner_compute_usage", required.Category)
		assert.Equal(t, "runner_compute_usage", required.EventType)
		assert.Equal(t, "SM", required.Realm)
		assert.Equal(t, "seconds", required.UnitOfMeasure)
		assert.Equal(t, 123.45, required.Quantity)

		// Verify optional fields
		assert.Equal(t, "dedicated-instance-abc123", optional.InstanceID)
		assert.Equal(t, "unique-dedicated-instance-abc123", optional.UniqueInstanceID)
		assert.Equal(t, int64(42), optional.ProjectID)
		assert.Equal(t, int64(123), optional.NamespaceID)
		assert.Equal(t, int64(100), optional.RootNamespaceID)
		assert.Equal(t, int64(1), optional.OrganizationID)

		// Verify metadata built by snowplow writer from record fields
		require.NotNil(t, optional.Metadata)
		assert.Equal(t, "success", optional.Metadata["job_status"])
		assert.Equal(t, "", optional.Metadata["job_failure_reason"])
		assert.Equal(t, "docker", optional.Metadata["executor"])
		assert.NotEmpty(t, optional.Metadata["started_at"])
		assert.NotEmpty(t, optional.Metadata["finished_at"])
	}

	tests := map[string]struct {
		mockRecord func() usage_log.Record
		assert     func(t *testing.T, required labkitsnowplow.BillingEventRequiredInput, optional labkitsnowplow.BillingEventOptionalInput)
	}{
		"runner UUID is provided": {
			mockRecord: testRecord,
			assert: func(t *testing.T, required labkitsnowplow.BillingEventRequiredInput, optional labkitsnowplow.BillingEventOptionalInput) {
				assertDefaults(t, required, optional)
				assert.Equal(t, "runner-uuid", optional.Subject)
			},
		},
		"runner UUID is not provided": {
			mockRecord: func() usage_log.Record {
				r := testRecord()
				r.Runner.UUID = ""

				return r
			},
			assert: func(t *testing.T, required labkitsnowplow.BillingEventRequiredInput, optional labkitsnowplow.BillingEventOptionalInput) {
				assertDefaults(t, required, optional)
				assert.Equal(t, "0", optional.Subject)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			w := newTestWriter(t, server.URL)
			defer func() {
				assert.NoError(t, w.Close())
			}()

			require.NotNil(t, tt.mockRecord)
			record := tt.mockRecord()

			required, optional := w.buildBillingInputs(record)

			require.NotNil(t, tt.assert)
			tt.assert(t, required, optional)
		})
	}
}

func TestWriter_Store_Concurrent(t *testing.T) {
	requestCount := 0
	mu := sync.Mutex{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	doneCh := make(chan struct{})
	eventsSent := 0
	eventsMu := sync.Mutex{}

	w := newTestWriter(t, server.URL, labkitsnowplow.WithCallback(func(successes, failures []labkitsnowplow.SendResult) {
		eventsMu.Lock()
		for _, s := range successes {
			eventsSent += s.EventCount
		}
		total := eventsSent
		eventsMu.Unlock()

		if total >= 10 {
			select {
			case <-doneCh:
			default:
				close(doneCh)
			}
		}
	}))
	defer func() {
		assert.NoError(t, w.Close())
	}()

	const numRecords = 10
	var wg sync.WaitGroup
	wg.Add(numRecords)

	for i := 0; i < numRecords; i++ {
		go func(idx int) {
			defer wg.Done()
			record := testRecord()
			record.UUID = fmt.Sprintf("test-uuid-%d", idx)
			err := w.Store(record)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Wait for all events to be sent
	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for concurrent events to be sent")
	}

	mu.Lock()
	assert.Greater(t, requestCount, 0, "Expected at least one HTTP request")
	mu.Unlock()
}

func TestWriter_CustomRealm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w, err := New(logrus.New(), server.URL, WithRealm("SaaS"), WithAppID("test-app"))
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, w.Close())
	}()

	record := testRecord()
	required, _ := w.buildBillingInputs(record)
	assert.Equal(t, "SaaS", required.Realm)
}

func TestWriter_CustomCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w, err := New(logrus.New(), server.URL, WithCategory("custom_usage"), WithAppID("test-app"))
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, w.Close())
	}()

	record := testRecord()
	required, _ := w.buildBillingInputs(record)
	assert.Equal(t, "custom_usage", required.Category)
}

// newTestWriter creates a writer for testing, using the provided test server URL.
func newTestWriter(t *testing.T, serverURL string, emitterOpts ...labkitsnowplow.EmitterOption) *Writer {
	t.Helper()

	if len(emitterOpts) > 0 {
		w, err := newWithEmitterOpts(serverURL, emitterOpts...)
		require.NoError(t, err)
		return w
	}

	w, err := New(logrus.New(), serverURL, WithAppID("gitlab-runner"))
	require.NoError(t, err)
	return w
}

// newWithEmitterOpts creates a Writer with custom emitter options for testing.
func newWithEmitterOpts(collectorURI string, emitterOpts ...labkitsnowplow.EmitterOption) (*Writer, error) {
	opts := setupOptions(collectorURI, WithAppID("gitlab-runner"))

	emitter, err := labkitsnowplow.NewEmitter(collectorURI, emitterOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating snowplow emitter: %w", err)
	}

	tracker, err := labkitsnowplow.NewTracker(opts.appID, emitter)
	if err != nil {
		return nil, fmt.Errorf("creating snowplow tracker: %w", err)
	}

	return &Writer{
		options: opts,
		tracker: tracker,
		log:     logrus.New(),
	}, nil
}

func testRecord() usage_log.Record {
	startedAt := time.Date(2024, 9, 23, 14, 27, 45, 123000000, time.UTC)
	finishedAt := time.Date(2024, 9, 23, 14, 30, 45, 123000000, time.UTC)

	return usage_log.Record{
		UUID:      "test-uuid",
		Timestamp: time.Date(2024, 9, 23, 14, 30, 45, 123000000, time.UTC),
		Runner: usage_log.Runner{
			UUID:     "runner-uuid",
			ID:       "runner-id",
			Name:     "runner-name",
			SystemID: "system-id",
			Executor: "docker",
		},
		Job: usage_log.Job{
			ID:              7890123,
			PipelineID:      456789,
			Ref:             "main",
			URL:             "https://example.com/job/1",
			DurationSeconds: 123.45,
			Status:          "success",
			FailureReason:   "",
			StartedAt:       startedAt,
			FinishedAt:      finishedAt,
			Project: usage_log.Project{
				ID:       42,
				Name:     "test-project",
				FullPath: "group/test-project",
			},
			Namespace: usage_log.Namespace{
				ID: 123,
			},
			RootNamespace: usage_log.Namespace{
				ID: 100,
			},
			Organization: usage_log.Organization{
				ID: 1,
			},
			Instance: usage_log.Instance{
				ID:       "dedicated-instance-abc123",
				UniqueID: "unique-dedicated-instance-abc123",
			},
		},
		Labels: map[string]string{
			"test-label": "test-value",
		},
	}
}

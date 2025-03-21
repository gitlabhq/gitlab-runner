//go:build !integration

package usage_log

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStorage_Store(t *testing.T) {
	tests := map[string]struct {
		closeBeforeWrite bool
		labels           map[string]string
		writeError       error
		expectedErr      error
	}{
		"storage writer error": {
			writeError:  assert.AnError,
			expectedErr: ErrStoringLog,
		},
		"storage closed before write": {
			closeBeforeWrite: true,
			expectedErr:      ErrStorageIsClosed,
		},
		"successful write": {},
		"successful write with storage level labels": {
			labels: map[string]string{
				"test-const-label": "test-const-value",
			},
		},
		"successful write with storage level label overwrite": {
			labels: map[string]string{
				"test-const-label": "test-const-value",
				"test-label":       "test-enforced-value",
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			buf := new(bytes.Buffer)

			testTime := time.Date(2024, 12, 5, 22, 11, 00, 00, time.UTC)

			var o []Option
			if tc.labels != nil {
				o = append(o, WithLabels(tc.labels))
			}

			w := newMockDummyWriteCloser(t)
			s := NewStorage(w, o...)
			s.timer = func() time.Time { return testTime }

			if tc.closeBeforeWrite {
				w.EXPECT().Close().Return(nil)
				assert.NoError(t, s.Close())
			} else {
				w.EXPECT().Write(mock.Anything).Return(0, tc.writeError).Run(func(p []byte) {
					buf.Write(p)
				})
			}

			err := s.Store(Record{
				Runner: Runner{
					ID: "short_token",
				},
				Job: Job{
					URL: "job-url",
				},
				Labels: map[string]string{
					"test-label": "test-value",
				},
			})
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
				return
			}
			assert.NoError(t, err)

			var r Record

			decoder := json.NewDecoder(buf)
			require.NoError(t, decoder.Decode(&r))

			assert.Equal(t, testTime, r.Timestamp)
			assert.Equal(t, "short_token", r.Runner.ID)
			assert.Equal(t, "job-url", r.Job.URL)

			require.Contains(t, r.Labels, "test-label")

			expectedTestLabelValue := "test-value"
			if v, ok := tc.labels["test-label"]; ok {
				expectedTestLabelValue = v
			}

			assert.Equal(t, expectedTestLabelValue, r.Labels["test-label"])

			if tc.labels != nil {
				assert.Contains(t, r.Labels, "test-const-label")
				assert.Equal(t, "test-const-value", r.Labels["test-const-label"])
			}
		})
	}
}

func TestStorage_Close(t *testing.T) {
	tests := map[string]struct {
		returnedError error
	}{
		"no error to return": {
			returnedError: nil,
		},
		"error to return": {
			returnedError: assert.AnError,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			w := newMockDummyWriteCloser(t)
			w.EXPECT().Close().Return(tc.returnedError)

			s := NewStorage(w)

			done := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				defer wg.Done()

				select {
				case <-s.close:
				case <-done:
					assert.Fail(t, "expected 'close' channel to get closed")
				}
			}()

			assert.ErrorIs(t, s.Close(), tc.returnedError)

			time.Sleep(10 * time.Millisecond)
			close(done)

			wg.Wait()
		})
	}
}

func TestStorage_StoreTimeChanges(t *testing.T) {
	testRecord := Record{
		Runner: Runner{
			ID: "short_token",
		},
		Job: Job{
			URL: "job-url",
		},
		Labels: map[string]string{
			"test-label": "test-value",
		},
	}

	var receivedRecords []Record

	w := newMockDummyWriteCloser(t)
	w.EXPECT().Write(mock.Anything).Return(0, nil).Run(func(p []byte) {
		var r Record
		err := json.Unmarshal(p, &r)
		require.NoError(t, err)

		receivedRecords = append(receivedRecords, r)
	})

	s := NewStorage(w)
	err := s.Store(testRecord)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	err = s.Store(testRecord)
	require.NoError(t, err)

	assert.Len(t, receivedRecords, 2)
	r1 := receivedRecords[0]
	r2 := receivedRecords[1]

	assert.NotEqual(t, r1, r2)
}

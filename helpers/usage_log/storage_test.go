//go:build !integration

package usage_log

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStorage_Write(t *testing.T) {
	tests := map[string]struct {
		closeBeforeWrite bool
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
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			buf := new(bytes.Buffer)

			testTime := time.Date(2024, 12, 5, 22, 11, 00, 00, time.UTC)

			w := newMockDummyWriteCloser(t)
			s := NewStorage(w)
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
			})
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
				return
			}
			assert.NoError(t, err)

			line := buf.String()
			assert.Contains(t, line, `"timestamp":"2024-12-05T22:11:00Z"`)
			assert.Contains(t, line, `"id":"short_token"`)
			assert.Contains(t, line, `"url":"job-url"`)
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

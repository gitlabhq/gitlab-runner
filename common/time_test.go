// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in https://github.com/golang/go/blob/master/LICENSE.
// The original code can be found in https://github.com/golang/go/blob/master/src/time/time_test.go
// TODO: remove file after upgrading to Go 1.9+

package common

import (
	"testing"
	"time"
)

// Original tests can be found in https://github.com/golang/go/blob/master/src/time/time_test.go
var durationRoundTests = []struct {
	d    time.Duration
	m    time.Duration
	want time.Duration
}{
	{0, time.Second, 0},
	{time.Minute, -11 * time.Second, time.Minute},
	{time.Minute, 0, time.Minute},
	{time.Minute, 1, time.Minute},
	{2 * time.Minute, time.Minute, 2 * time.Minute},
	{2*time.Minute + 10*time.Second, time.Minute, 2 * time.Minute},
	{2*time.Minute + 30*time.Second, time.Minute, 3 * time.Minute},
	{2*time.Minute + 50*time.Second, time.Minute, 3 * time.Minute},
	{-time.Minute, 1, -time.Minute},
	{-2 * time.Minute, time.Minute, -2 * time.Minute},
	{-2*time.Minute - 10*time.Second, time.Minute, -2 * time.Minute},
	{-2*time.Minute - 30*time.Second, time.Minute, -3 * time.Minute},
	{-2*time.Minute - 50*time.Second, time.Minute, -3 * time.Minute},
	{8e18, 3e18, 9e18},
	{9e18, 5e18, 1<<63 - 1},
	{-8e18, 3e18, -9e18},
	{-9e18, 5e18, -1 << 63},
	{3<<61 - 1, 3 << 61, 3 << 61},
}

func TestDurationRound(t *testing.T) {
	for _, tt := range durationRoundTests {
		if got := roundDuration(tt.d, tt.m); got != tt.want {
			t.Errorf("Duration(%s).Round(%s) = %s; want: %s", tt.d, tt.m, got, tt.want)
		}
	}
}

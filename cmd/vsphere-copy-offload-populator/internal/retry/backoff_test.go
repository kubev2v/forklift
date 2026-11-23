package retry

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	processingTime := time.Second
	spread := 20900 * time.Millisecond
	for i := 0; i < 1000; i++ {
		delay, err := Backoff(processingTime, spread)
		if err != nil {
			t.Fatal(err)
		}
		if delay < processingTime || delay > processingTime+2*spread {
			t.Fatalf("delay %v is outside the requested interval", delay)
		}
	}
}

func TestBackoffInvalidDurations(t *testing.T) {
	for _, tc := range []struct{ processingTime, spread time.Duration }{
		{0, time.Second},
		{time.Second, 0},
		{-time.Second, time.Second},
		{time.Second, time.Duration(1 << 62)},
	} {
		if _, err := Backoff(tc.processingTime, tc.spread); err == nil {
			t.Fatalf("expected invalid durations to fail: %+v", tc)
		}
	}
}

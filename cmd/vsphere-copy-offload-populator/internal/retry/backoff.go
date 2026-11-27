package retry

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// Backoff samples a uniform delay between processingTime and processingTime + 2*spread.
func Backoff(processingTime, spread time.Duration) (time.Duration, error) {
	if processingTime <= 0 || spread <= 0 || spread > (time.Duration(1<<63-1)-processingTime)/2 {
		return 0, fmt.Errorf("invalid backoff durations: processing time %v, spread %v", processingTime, spread)
	}
	jitter, err := rand.Int(rand.Reader, big.NewInt(int64(2*spread)+1))
	if err != nil {
		return 0, fmt.Errorf("generate backoff: %w", err)
	}
	return processingTime + time.Duration(jitter.Int64()), nil
}

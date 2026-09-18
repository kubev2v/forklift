package conversion

import (
	"bytes"
	"sync"
	"testing"
)

func TestCaptureWriterForwardsAndRetainsTail(t *testing.T) {
	var forwarded bytes.Buffer
	writer := newCaptureWriter(&forwarded, 5)

	if _, err := writer.Write([]byte("1234567")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if got := forwarded.String(); got != "1234567" {
		t.Fatalf("expected all bytes to be forwarded, got %q", got)
	}
	if got := string(writer.Bytes()); got != "34567" {
		t.Fatalf("expected newest bytes in bounded tail, got %q", got)
	}
}

func TestCaptureWriterSupportsConcurrentWrites(t *testing.T) {
	writer := newCaptureWriter(&bytes.Buffer{}, 1024)
	const writers = 8
	const writesPerWriter = 100

	var group sync.WaitGroup
	group.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer group.Done()
			for j := 0; j < writesPerWriter; j++ {
				if _, err := writer.Write([]byte("x")); err != nil {
					t.Errorf("concurrent write failed: %v", err)
					return
				}
			}
		}()
	}
	group.Wait()

	if got := len(writer.Bytes()); got != writers*writesPerWriter {
		t.Fatalf("expected %d retained bytes, got %d", writers*writesPerWriter, got)
	}
}

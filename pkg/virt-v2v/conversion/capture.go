package conversion

import (
	"fmt"
	"io"
	"sync"

	"github.com/kubev2v/forklift/pkg/virt-v2v/errorreporting"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

const virtV2vCaptureLimit = 128 * 1024

type captureWriter struct {
	mu       sync.Mutex
	forward  io.Writer
	limit    int
	contents []byte
}

func newCaptureWriter(forward io.Writer, limit int) *captureWriter {
	return &captureWriter{
		forward: forward,
		limit:   limit,
	}
}

func (w *captureWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.forward.Write(p)
	if n > 0 {
		w.contents = append(w.contents, p[:n]...)
		if len(w.contents) > w.limit {
			w.contents = w.contents[len(w.contents)-w.limit:]
		}
	}
	return n, err
}

func (w *captureWriter) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.contents...)
}

func classifyCapturedOutput(stdout, stderr []byte) errorreporting.Failure {
	return errorreporting.Classify(string(stdout) + "\n" + string(stderr))
}

func (c *Conversion) writeTerminationFailure(stdout, stderr []byte) {
	failure := classifyCapturedOutput(stdout, stderr)
	if failure.Code == errorreporting.UnknownVirtV2vError {
		return
	}
	payload, err := errorreporting.Encode(failure)
	if err != nil {
		fmt.Printf("Failed to encode virt-v2v failure: %v\n", err)
		return
	}
	fileSystem := c.fileSystem
	if fileSystem == nil {
		fileSystem = utils.FileSystemImpl{}
	}
	if err := fileSystem.WriteFile("/dev/termination-log", payload, 0644); err != nil {
		fmt.Printf("Failed to write virt-v2v termination message: %v\n", err)
	}
}

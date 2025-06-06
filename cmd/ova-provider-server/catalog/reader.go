package catalog

import "io"

// ProgressReader reports status on a read.
type ProgressReader struct {
	io.Reader
	Source        string
	ContentLength int64
	BytesRead     int64
	LastUpdate    int64
	ProgressFunc  func(string, int64, int64)
}

func (r *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err != nil {
		return
	}
	if r.ProgressFunc == nil {
		return
	}
	r.BytesRead += int64(n)
	// only bother updating if we read at least 1% of the file
	if (r.BytesRead-r.LastUpdate) >= (r.ContentLength/100) || r.BytesRead >= r.ContentLength {
		r.ProgressFunc(r.Source, r.ContentLength, r.BytesRead)
		r.LastUpdate = r.BytesRead
	}
	return
}

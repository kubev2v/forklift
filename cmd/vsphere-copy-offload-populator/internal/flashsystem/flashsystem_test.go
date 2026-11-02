package flashsystem

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMultiHostMappingRetriesSCSIConflict(t *testing.T) {
	var ids []string
	queries := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, body := 200, `[{"SCSI_id":"10"}]`
		if strings.Contains(r.URL.Path, "lshostvdiskmap") {
			queries++
		} else {
			b, _ := io.ReadAll(r.Body)
			ids = append(ids, string(b))
			if len(ids) == 1 {
				status, body = 409, "CMMVC5879E"
			} else {
				body = "{}"
			}
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	log := logger.New("test")
	c := FlashSystemClonner{api: &FlashSystemAPIClient{ManagementIP: "test", httpClient: &http.Client{Transport: transport}, log: log}, log: log}
	if err := c.mapMultiHostVolume("/mkvdiskhostmap/1", "host", "1"); err != nil {
		t.Fatal(err)
	}
	if queries != 2 || len(ids) != 2 || !strings.Contains(ids[0], `"scsi":11`) || !strings.Contains(ids[1], `"scsi":12`) {
		t.Fatalf("queries=%d requests=%v", queries, ids)
	}
}

func TestMultiHostMappingDoesNotRetryOtherConflicts(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		status, body := 200, `[]`
		if strings.Contains(r.URL.Path, "mkvdiskhostmap") {
			status, body = 409, "another error"
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	log := logger.New("test")
	c := FlashSystemClonner{api: &FlashSystemAPIClient{ManagementIP: "test", httpClient: &http.Client{Transport: transport}, log: log}, log: log}
	if err := c.mapMultiHostVolume("/mkvdiskhostmap/1", "host", "1"); err == nil || !strings.Contains(err.Error(), "another error") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("got %d requests, want 2", requests)
	}
}

func TestMultiHostMappingUsesRefreshedSCSIIDs(t *testing.T) {
	queries := 0
	var ids []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, body := 200, `[{"SCSI_id":"10"}]`
		if strings.Contains(r.URL.Path, "lshostvdiskmap") {
			queries++
			if queries > 1 {
				body = `[{"SCSI_id":"10"},{"SCSI_id":"12"}]`
			}
		} else {
			b, _ := io.ReadAll(r.Body)
			ids = append(ids, string(b))
			if len(ids) == 1 {
				status, body = 409, "CMMVC5879E"
			} else {
				body = "{}"
			}
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	log := logger.New("test")
	c := FlashSystemClonner{api: &FlashSystemAPIClient{ManagementIP: "test", httpClient: &http.Client{Transport: transport}, log: log}, log: log}
	if err := c.mapMultiHostVolume("/mkvdiskhostmap/1", "host", "1"); err != nil {
		t.Fatal(err)
	}
	if queries != 2 || len(ids) != 2 || !strings.Contains(ids[0], `"scsi":11`) || !strings.Contains(ids[1], `"scsi":13`) {
		t.Fatalf("queries=%d requests=%v", queries, ids)
	}
}

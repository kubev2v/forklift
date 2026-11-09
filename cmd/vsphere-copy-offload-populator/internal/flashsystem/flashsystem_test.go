package flashsystem

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMultiHostMappingRetriesSCSIConflict(t *testing.T) {
	type mappingRequest struct {
		path    string
		payload map[string]interface{}
	}
	var mappings []mappingRequest
	queries := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, body := 200, `[{"SCSI_id":"10"}]`
		if strings.Contains(r.URL.Path, "lshostvdiskmap") {
			queries++
		} else {
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				return nil, err
			}
			mappings = append(mappings, mappingRequest{path: r.URL.Path, payload: payload})
			if len(mappings) == 1 {
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
	if queries != 2 || len(mappings) != 2 {
		t.Fatalf("queries=%d mappings=%v", queries, mappings)
	}
	for _, mapping := range mappings {
		if mapping.path != "/mkvdiskhostmap/1" || mapping.payload["host"] != "host" {
			t.Fatalf("unexpected mapping request: %+v", mapping)
		}
	}
	if mappings[0].payload["scsi"] != float64(11) || mappings[1].payload["scsi"] != float64(12) {
		t.Fatalf("unexpected SCSI IDs in requests: %+v", mappings)
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

func TestMultiHostMappingStopsAfterPersistentSCSIConflicts(t *testing.T) {
	var delays []time.Duration
	mappings, queries := 0, 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, body := http.StatusOK, `[{"SCSI_id":"10"}]`
		if strings.Contains(r.URL.Path, "lshostvdiskmap") {
			queries++
		} else {
			mappings++
			status, body = http.StatusConflict, flashSystemSCSIIDInUse
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	log := logger.New("test")
	c := FlashSystemClonner{api: &FlashSystemAPIClient{ManagementIP: "test", httpClient: &http.Client{Transport: transport}, log: log}, log: log}
	err := c.mapMultiHostVolumeWithSleep("/mkvdiskhostmap/1", "host", "1", func(delay time.Duration) {
		delays = append(delays, delay)
	})
	if err == nil || !strings.Contains(err.Error(), flashSystemSCSIIDInUse) || !strings.Contains(err.Error(), "after 6 attempts") {
		t.Fatalf("expected exhausted CMMVC5879E error, got: %v", err)
	}
	if mappings != 6 || queries != 6 {
		t.Fatalf("mappings=%d queries=%d, want 6 and 6", mappings, queries)
	}
	if len(delays) != 5 {
		t.Fatalf("got %d backoff delays, want 5", len(delays))
	}
}

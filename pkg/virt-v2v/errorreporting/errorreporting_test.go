package errorreporting

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	core "k8s.io/api/core/v1"
)

func TestClassifyDirtyFilesystem(t *testing.T) {
	output := `virt-v2v: error: filesystem was mounted read-only, even though we asked for it to be mounted read-write.
This usually means that the filesystem was not cleanly unmounted.
Possible causes include trying to convert a guest which is running, or using Windows Hibernation or Fast Restart.`

	failure := Classify(output)

	if failure.Code != DirtyFilesystem {
		t.Fatalf("expected code %q, got %q", DirtyFilesystem, failure.Code)
	}
	if failure.Summary == "" || failure.Message == "" || failure.Remediation == "" {
		t.Fatalf("expected complete failure details, got %#v", failure)
	}
	if failure.Summary != "The guest filesystem was not cleanly unmounted." {
		t.Errorf("unexpected summary: %q", failure.Summary)
	}
}

func TestClassifyDirtyFilesystemWithTimestampedFragmentedOutput(t *testing.T) {
	output := "2026-09-06T12:04:14.530350333Z Falling back to read-only mount because the NTFS partition is in an unsafe state.\n" +
		"2026-09-06T12:04:14.530350333Z virt-v2v: error: filesystem was mounted read-only, even though we asked for it to be mounted read-write.\n" +
		"2026-09-06T12:04:14.530397233Z This usually means that the filesystem was not cleanly unmounted."

	failure := Classify(output)

	if failure.Code != DirtyFilesystem {
		t.Fatalf("expected timestamped output to classify as %q, got %q", DirtyFilesystem, failure.Code)
	}
}

func TestClassifyDoesNotMatchStandaloneReadOnlyOrIOError(t *testing.T) {
	for _, output := range []string{
		"filesystem was mounted read-only for inspection",
		"I/O error, dev sda, sector 267344772",
		"virt-v2v: error: unable to read the source disk",
	} {
		failure := Classify(output)
		if failure.Code != UnknownVirtV2vError {
			t.Errorf("expected %q for %q, got %q", UnknownVirtV2vError, output, failure.Code)
		}
	}
}

func TestClassifyNormalizesControlCharactersAndWhitespace(t *testing.T) {
	output := "\x1b[31mfilesystem was mounted read-only, even though we asked for it to be mounted read-write.\x1b[0m\r\n" +
		"This usually means that the filesystem was not cleanly\tunmounted."

	failure := Classify(output)

	if failure.Code != DirtyFilesystem {
		t.Fatalf("expected normalized output to classify as %q, got %q", DirtyFilesystem, failure.Code)
	}
}

func TestEncodeDecode(t *testing.T) {
	failure := Classify("virt-v2v: error: filesystem was mounted read-only, even though we asked for it to be mounted read-write. This usually means that the filesystem was not cleanly unmounted.")

	payload, err := Encode(failure)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(payload) > MaxTerminationMessageBytes {
		t.Fatalf("encoded payload is %d bytes, want at most %d", len(payload), MaxTerminationMessageBytes)
	}
	decoded, err := Decode(payload)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded != failure {
		t.Fatalf("Decode() = %#v, want %#v", decoded, failure)
	}
}

func TestDecodeRejectsNonCanonicalPayloads(t *testing.T) {
	canonical := dirtyFilesystemFailure
	canonicalJSON := mustJSON(t, canonical)
	altered := canonical
	altered.Message = "different"
	alteredJSON := mustJSON(t, altered)
	trailingJSON := append(append([]byte(nil), canonicalJSON...), []byte(` {}`)...)
	unknownFieldJSON := append(append([]byte(nil), canonicalJSON[:len(canonicalJSON)-1]...), []byte(`,"extra":"x"}`)...)

	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "unknown code", payload: []byte(`{"code":"SomeOtherFailure","summary":"x","message":"y","remediation":"z"}`)},
		{name: "altered known failure", payload: alteredJSON},
		{name: "trailing json", payload: trailingJSON},
		{name: "unknown field", payload: unknownFieldJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decode(tt.payload); err == nil {
				t.Fatalf("Decode() unexpectedly accepted %s", tt.name)
			}
		})
	}

	tooLarge := bytes.Repeat([]byte("x"), MaxTerminationMessageBytes+1)
	if _, err := Decode(tooLarge); err == nil {
		t.Fatal("Decode() unexpectedly accepted oversized payload")
	}
}

func TestFormat(t *testing.T) {
	formatted := Format(dirtyFilesystemFailure)
	for _, fragment := range []string{
		dirtyFilesystemFailure.Summary,
		dirtyFilesystemFailure.Message,
		dirtyFilesystemFailure.Remediation,
	} {
		if !strings.Contains(formatted, fragment) {
			t.Fatalf("Format() = %q, missing %q", formatted, fragment)
		}
	}
}

func TestExtractFromPod(t *testing.T) {
	failure := Failure{
		Code:        DirtyFilesystem,
		Summary:     "The guest filesystem was not cleanly unmounted.",
		Message:     "virt-v2v could not mount the filesystem read-write.",
		Remediation: "Start the VM in the source environment, allow it to boot, and let MTV power it off cleanly before retrying. Check for Windows hibernation or Fast Startup.",
	}
	payload, err := Encode(failure)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	pod := &core.Pod{Status: core.PodStatus{ContainerStatuses: []core.ContainerStatus{
		{Name: "sidecar", State: core.ContainerState{Terminated: &core.ContainerStateTerminated{ExitCode: 1, Message: "irrelevant"}}},
		{Name: "virt-v2v", State: core.ContainerState{Terminated: &core.ContainerStateTerminated{ExitCode: 1, Message: string(payload)}}},
	}}}

	got, ok := ExtractFromPod(pod)
	if !ok {
		t.Fatal("ExtractFromPod() did not find the failure")
	}
	if got != failure {
		t.Fatalf("ExtractFromPod() = %#v, want %#v", got, failure)
	}
}

func TestExtractFromPodFallsBackToLastTerminationState(t *testing.T) {
	failure := Failure{
		Code:        DirtyFilesystem,
		Summary:     "The guest filesystem was not cleanly unmounted.",
		Message:     "virt-v2v could not mount the filesystem read-write.",
		Remediation: "Start the VM in the source environment, allow it to boot, and let MTV power it off cleanly before retrying. Check for Windows hibernation or Fast Startup.",
	}
	payload, err := Encode(failure)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	pod := &core.Pod{Status: core.PodStatus{ContainerStatuses: []core.ContainerStatus{
		{Name: "virt-v2v", LastTerminationState: core.ContainerState{Terminated: &core.ContainerStateTerminated{ExitCode: 1, Message: string(payload)}}},
	}}}

	got, ok := ExtractFromPod(pod)
	if !ok {
		t.Fatal("ExtractFromPod() did not find the failure via LastTerminationState")
	}
	if got != failure {
		t.Fatalf("ExtractFromPod() = %#v, want %#v", got, failure)
	}
}

func TestExtractFromPodRejectsMalformedOrMissingPayload(t *testing.T) {
	for name, pod := range map[string]*core.Pod{
		"missing container": {Status: core.PodStatus{ContainerStatuses: []core.ContainerStatus{{Name: "other"}}}},
		"running container": {Status: core.PodStatus{ContainerStatuses: []core.ContainerStatus{{Name: "virt-v2v", State: core.ContainerState{Running: &core.ContainerStateRunning{}}}}}},
		"malformed payload": {Status: core.PodStatus{ContainerStatuses: []core.ContainerStatus{{Name: "virt-v2v", State: core.ContainerState{Terminated: &core.ContainerStateTerminated{ExitCode: 1, Message: "not json"}}}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := ExtractFromPod(pod); ok {
				t.Fatal("ExtractFromPod() unexpectedly accepted payload")
			}
		})
	}
}

func mustJSON(t *testing.T, value Failure) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return payload
}

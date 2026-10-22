package errorreporting

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	core "k8s.io/api/core/v1"
)

const (
	DirtyFilesystem     = "DirtyFilesystem"
	UnknownVirtV2vError = "UnknownVirtV2vError"

	MaxTerminationMessageBytes = 4096
)

type Failure struct {
	Code        string `json:"code"`
	Summary     string `json:"summary"`
	Message     string `json:"message"`
	Remediation string `json:"remediation"`
}

var (
	ansiPattern      = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	timestampPattern = regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\s*`)
)

var dirtyFilesystemFailure = Failure{
	Code:    DirtyFilesystem,
	Summary: "The guest filesystem was not cleanly unmounted.",
	Message: "virt-v2v could not mount the filesystem read-write.",
	Remediation: "Start the VM in the source environment, allow it to boot, and let MTV power it off cleanly " +
		"before retrying. Check for Windows hibernation or Fast Startup.",
}

// Classify identifies a known virt-v2v failure from the complete output blob.
// Matching is deliberately based on stable fragments rather than physical log lines.
func Classify(output string) Failure {
	normalized := normalize(output)
	if strings.Contains(normalized, "filesystem was mounted read-only") &&
		(strings.Contains(normalized, "not cleanly unmounted") || strings.Contains(normalized, "unsafe state")) {
		return dirtyFilesystemFailure
	}
	return Failure{Code: UnknownVirtV2vError}
}

// KnownFailure returns the repository-owned presentation for a known code.
func KnownFailure(code string) (Failure, bool) {
	switch code {
	case DirtyFilesystem:
		return dirtyFilesystemFailure, true
	default:
		return Failure{}, false
	}
}

// Encode validates and serializes a known failure for a Kubernetes termination message.
func Encode(failure Failure) ([]byte, error) {
	canonical, ok := KnownFailure(failure.Code)
	if !ok || failure != canonical {
		return nil, errors.New("unknown or non-canonical virt-v2v failure")
	}
	payload, err := json.Marshal(failure)
	if err != nil {
		return nil, fmt.Errorf("marshal virt-v2v failure: %w", err)
	}
	if len(payload) > MaxTerminationMessageBytes {
		return nil, fmt.Errorf("virt-v2v failure payload exceeds %d bytes", MaxTerminationMessageBytes)
	}
	return payload, nil
}

// Decode validates a termination message and reconstructs the repository-owned text.
func Decode(payload []byte) (Failure, error) {
	if len(payload) == 0 || len(payload) > MaxTerminationMessageBytes {
		return Failure{}, fmt.Errorf("virt-v2v failure payload size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var received Failure
	if err := decoder.Decode(&received); err != nil {
		return Failure{}, fmt.Errorf("decode virt-v2v failure: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Failure{}, errors.New("virt-v2v failure payload contains multiple values")
	}
	canonical, ok := KnownFailure(received.Code)
	if !ok || received != canonical {
		return Failure{}, errors.New("virt-v2v failure payload is not canonical")
	}
	return canonical, nil
}

// ExtractFromPod reads the virt-v2v container's termination message from a pod.
// The fallback to LastTerminationState covers pods whose container was restarted
// before the pod became terminal.
func ExtractFromPod(pod *core.Pod) (Failure, bool) {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != "virt-v2v" {
			continue
		}
		terminated := status.State.Terminated
		if terminated == nil {
			terminated = status.LastTerminationState.Terminated
		}
		if terminated == nil {
			return Failure{}, false
		}
		failure, err := Decode([]byte(terminated.Message))
		if err != nil {
			return Failure{}, false
		}
		return failure, true
	}
	return Failure{}, false
}

// Format returns the human-readable error propagated to Conversion and Plan status.
func Format(failure Failure) string {
	if failure.Code == UnknownVirtV2vError {
		return ""
	}
	return failure.Summary + " " + failure.Message + " Suggested action: " + failure.Remediation
}

func normalize(output string) string {
	output = ansiPattern.ReplaceAllString(output, " ")
	output = timestampPattern.ReplaceAllString(output, " ")
	return strings.ToLower(strings.Join(strings.Fields(output), " "))
}

package diagnostics

import "regexp"

var (
	// High-priority error patterns: these are the root-cause lines users care about
	rootCausePatterns = []*regexp.Regexp{
		regexp.MustCompile(`virt-v2v: error:`),
		regexp.MustCompile(`guestfsd: error:`),
		regexp.MustCompile(`(?i)FAILED.*Result:`),
		regexp.MustCompile(`(?i)I/O error, dev`),
		regexp.MustCompile(`(?i)conversion failed`),
		regexp.MustCompile(`(?i)unable to mount`),
		regexp.MustCompile(`(?i)unknown filesystem type`),
		regexp.MustCompile(`(?i)xfs_repair`),
		regexp.MustCompile(`(?i)superblock.*corrupt`),
	}

	errorPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\berror\b`),
		regexp.MustCompile(`(?i)\bfailed\b`),
		regexp.MustCompile(`(?i)\bfatal\b`),
		regexp.MustCompile(`(?i)\bpanic\b`),
	}

	warningPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bwarn(ing)?\b`),
	}

	ignorePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)error.*nil`),
		regexp.MustCompile(`(?i)no error`),
		regexp.MustCompile(`(?i)error count\D+\b0\b`),
		regexp.MustCompile(`(?i)nbdkit:.*debug`),
		regexp.MustCompile(`(?i)Cannot open file`),
		regexp.MustCompile(`(?i)get_backend_setting.*NULL.*error`),
		regexp.MustCompile(`(?i)Failed to resolve (group|user)`),
		regexp.MustCompile(`(?i)Failed to determine unit`),
		regexp.MustCompile(`(?i)Failed to connect to bus`),
		regexp.MustCompile(`(?i)Failed to parse ACL`),
		regexp.MustCompile(`(?i)you can ignore this message`),
	}
)

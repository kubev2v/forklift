package cmdbuilder

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const maskedPlaceholder = "***"

// entry is one argument group: a bare token or a flag+value pair.
// When masked, the value is hidden in MaskedArgs (display overrides for single tokens).
type entry struct {
	tokens  []string
	masked  bool
	display string // shown instead of "***" for single masked tokens
}

type envOpKind int

const (
	envOpSet    envOpKind = iota // add or override a variable
	envOpUnset                   // remove a variable from the inherited env
	envOpFilter                  // transform a variable's value; returning "" removes it
)

type envOp struct {
	kind  envOpKind
	key   string
	value string              // used by envOpSet
	fn    func(string) string // used by envOpFilter
}

// CmdBuilder accumulates command-line arguments and environment modifications.
type CmdBuilder struct {
	entries []entry
	envOps  []envOp
	logger  *logrus.Logger
}

func New() *CmdBuilder {
	return &CmdBuilder{}
}

// WithLogger attaches a logger; the command is logged at Debug level before execution.
func (b *CmdBuilder) WithLogger(logger *logrus.Logger) *CmdBuilder {
	b.logger = logger
	return b
}

// Add appends one or more bare tokens (flags, separators, positional args).
func (b *CmdBuilder) Add(tokens ...string) *CmdBuilder {
	for _, t := range tokens {
		b.entries = append(b.entries, entry{tokens: []string{t}})
	}
	return b
}

// Flag appends a flag+value pair.
func (b *CmdBuilder) Flag(flag, value string) *CmdBuilder {
	b.entries = append(b.entries, entry{tokens: []string{flag, value}})
	return b
}

// SensitiveFlag appends a flag+value pair whose value is hidden in MaskedArgs.
func (b *CmdBuilder) SensitiveFlag(flag, value string) *CmdBuilder {
	b.entries = append(b.entries, entry{tokens: []string{flag, value}, masked: true})
	return b
}

// SensitiveArg appends a single token that is passed as actual to the process
// but shown as display in MaskedArgs (e.g. "password=+***" for inline secrets).
func (b *CmdBuilder) SensitiveArg(actual, display string) *CmdBuilder {
	b.entries = append(b.entries, entry{tokens: []string{actual}, masked: true, display: display})
	return b
}

// AddIf appends tokens only when cond is true.
func (b *CmdBuilder) AddIf(cond bool, tokens ...string) *CmdBuilder {
	if cond {
		return b.Add(tokens...)
	}
	return b
}

// FlagIf appends a flag+value pair only when cond is true.
func (b *CmdBuilder) FlagIf(cond bool, flag, value string) *CmdBuilder {
	if cond {
		return b.Flag(flag, value)
	}
	return b
}

// Args returns the full argument list for exec.Command.
func (b *CmdBuilder) Args() []string {
	out := make([]string, 0, len(b.entries)*2)
	for _, e := range b.entries {
		out = append(out, e.tokens...)
	}
	return out
}

// MaskedArgs returns the argument list safe for logging (sensitive values replaced).
func (b *CmdBuilder) MaskedArgs() []string {
	out := make([]string, 0, len(b.entries)*2)
	for _, e := range b.entries {
		if !e.masked {
			out = append(out, e.tokens...)
			continue
		}
		switch len(e.tokens) {
		case 2:
			out = append(out, e.tokens[0], maskedPlaceholder)
		case 1:
			if e.display != "" {
				out = append(out, e.display)
			} else {
				out = append(out, maskedPlaceholder)
			}
		default:
			out = append(out, e.tokens...)
		}
	}
	return out
}

// SetEnv adds or overrides an environment variable in the child process.
func (b *CmdBuilder) SetEnv(key, value string) *CmdBuilder {
	b.envOps = append(b.envOps, envOp{kind: envOpSet, key: key, value: value})
	return b
}

// UnsetEnv removes a variable from the child's environment.
func (b *CmdBuilder) UnsetEnv(key string) *CmdBuilder {
	b.envOps = append(b.envOps, envOp{kind: envOpUnset, key: key})
	return b
}

// FilterEnv transforms a variable's value through fn; returning "" removes it.
func (b *CmdBuilder) FilterEnv(key string, fn func(string) string) *CmdBuilder {
	b.envOps = append(b.envOps, envOp{kind: envOpFilter, key: key, fn: fn})
	return b
}

func (b *CmdBuilder) buildEnv() []string {
	base := os.Environ()
	envMap := make(map[string]string, len(base))
	for _, e := range base {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
	}
	for _, op := range b.envOps {
		switch op.kind {
		case envOpSet:
			envMap[op.key] = op.value
		case envOpUnset:
			delete(envMap, op.key)
		case envOpFilter:
			if v, ok := envMap[op.key]; ok {
				if newV := op.fn(v); newV != "" {
					envMap[op.key] = newV
				} else {
					delete(envMap, op.key)
				}
			}
		}
	}
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}

// Command returns a configured *exec.Cmd ready to run.
func (b *CmdBuilder) Command(ctx context.Context, name string) *exec.Cmd {
	if b.logger != nil {
		parts := append([]string{name}, b.MaskedArgs()...)
		b.logger.WithField("command", strings.Join(parts, " ")).Info("Executing command")
	}
	cmd := exec.CommandContext(ctx, name, b.Args()...)
	if len(b.envOps) > 0 {
		cmd.Env = b.buildEnv()
	}
	return cmd
}

// RunCombined runs the command and returns merged stdout+stderr. It invokes
// onLine (if non-nil) with each line as it is produced. Returns ctx.Err()
// directly on cancellation or timeout.
func (b *CmdBuilder) RunCombined(ctx context.Context, name string, onLine func(line string)) ([]byte, error) {
	cmd := b.Command(ctx, name)
	cmd.WaitDelay = time.Second

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var buf bytes.Buffer
	var mu sync.Mutex
	var scanErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = pr.Close() }()
		scanner := bufio.NewScanner(pr)
		scanner.Split(splitCommandOutputLines)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			buf.WriteString(line)
			buf.WriteByte('\n')
			mu.Unlock()
			if onLine != nil {
				onLine(line)
			}
		}
		scanErr = scanner.Err()
	}()

	err := cmd.Start()
	if err != nil {
		_ = pw.Close()
		<-done
		return nil, err
	}
	if b.logger != nil {
		b.logger.WithField("pid", cmd.Process.Pid).Info("Process started")
	}

	err = cmd.Wait()
	_ = pw.Close()
	<-done

	if err != nil && ctx.Err() != nil {
		return buf.Bytes(), ctx.Err()
	}
	if err == nil && scanErr != nil {
		return buf.Bytes(), scanErr
	}
	return buf.Bytes(), err
}

// RunSeparate runs the command and returns stdout and stderr independently,
// while streaming stderr lines through onLine for live progress. Stdout is
// captured into a buffer (suitable for structured output like XML). Returns
// ctx.Err() directly on cancellation or timeout.
func (b *CmdBuilder) RunSeparate(ctx context.Context, name string, onLine func(line string)) (stdout, stderr []byte, err error) {
	cmd := b.Command(ctx, name)
	cmd.WaitDelay = 5 * time.Second

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	stderrPr, stderrPw := io.Pipe()
	cmd.Stderr = stderrPw

	var stderrBuf bytes.Buffer
	var scanErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = stderrPr.Close() }()
		scanner := bufio.NewScanner(stderrPr)
		scanner.Split(splitCommandOutputLines)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line)
			stderrBuf.WriteByte('\n')
			if onLine != nil {
				onLine(line)
			}
		}
		scanErr = scanner.Err()
	}()

	err = cmd.Start()
	if err != nil {
		_ = stderrPw.Close()
		<-done
		return nil, nil, err
	}
	if b.logger != nil {
		b.logger.WithField("pid", cmd.Process.Pid).Info("Process started")
	}

	err = cmd.Wait()
	_ = stderrPw.Close()
	<-done

	if err != nil && ctx.Err() != nil {
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), ctx.Err()
	}
	if err == nil && scanErr != nil {
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), scanErr
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
}

func splitCommandOutputLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b != '\r' && b != '\n' {
			continue
		}
		if b == '\r' && i == len(data)-1 && !atEOF {
			return 0, nil, nil
		}
		advance = i + 1
		if b == '\r' && advance < len(data) && data[advance] == '\n' {
			advance++
		}
		return advance, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// ExitCode extracts the exit code from a run error; returns -1 if not available.
func ExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// Package stagelog provides a logr.LogSink that formats each line with the
// logger name (stage) at the beginning as [stage], so WithName("cloning")
// produces "[cloning] msg ..." in the output.
package stagelog

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
)

const loggerKey = "logger"

// stageSink is a logr.LogSink that writes lines as:
//   I0211 14:30:00.123456       1 file.go:123] [stage] msg key=value ...
// The "stage" is taken from the "logger" key (set by WithName), using the
// last segment after the final dot so "copy-offload.cloning" -> "[cloning]".
type stageSink struct {
	name      string
	values    []interface{}
	callDepth int
	out       io.Writer
}

// NewStageSink returns a logr.LogSink that formats output with [stage] at the
// start of each message. out is the writer for log lines (e.g. os.Stderr).
func NewStageSink(out io.Writer) logr.LogSink {
	if out == nil {
		out = os.Stderr
	}
	return &stageSink{out: out}
}

func (s *stageSink) Init(info logr.RuntimeInfo) {
	s.callDepth = info.CallDepth
}

func (s *stageSink) Enabled(level int) bool {
	// Delegate to klog's V so -v and -vmodule still work.
	return klog.VDepth(s.callDepth+2, klog.Level(level)).Enabled()
}

func (s *stageSink) Info(level int, msg string, kvList ...interface{}) {
	merged := mergeKV(s.values, kvList)
	stage := s.stageFromKV(merged)
	rest := s.kvWithoutLogger(merged)
	s.writeLine('I', stage, msg, nil, rest)
}

func (s *stageSink) Error(err error, msg string, kvList ...interface{}) {
	merged := mergeKV(s.values, kvList)
	stage := s.stageFromKV(merged)
	rest := s.kvWithoutLogger(merged)
	s.writeLine('E', stage, msg, err, rest)
}

func (s *stageSink) WithName(name string) logr.LogSink {
	clone := *s
	if clone.name == "" {
		clone.name = name
	} else {
		clone.name = clone.name + "." + name
	}
	clone.values = append([]interface{}(nil), clone.values...)
	return &clone
}

func (s *stageSink) WithValues(kvList ...interface{}) logr.LogSink {
	clone := *s
	clone.values = mergeKV(s.values, kvList)
	return &clone
}

func (s *stageSink) WithCallDepth(depth int) logr.LogSink {
	clone := *s
	clone.callDepth += depth
	return &clone
}

// stageFromKV returns the stage tag from the sink's name (set by WithName).
// Uses the last segment after the final dot, e.g. "copy-offload.cloning" -> "cloning".
func (s *stageSink) stageFromKV(kv []interface{}) string {
	name := s.name
	if name == "" {
		// Fallback: check for "logger" in kv (some backends inject it)
		for i := 0; i+1 < len(kv); i += 2 {
			if k, ok := kv[i].(string); ok && k == loggerKey {
				if v, ok := kv[i+1].(string); ok {
					name = v
				}
				break
			}
		}
	}
	if name == "" {
		return ""
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 && idx+1 < len(name) {
		name = name[idx+1:]
	}
	return name
}

// kvWithoutLogger returns kv pairs excluding the "logger" key, for appending after the message.
func (s *stageSink) kvWithoutLogger(kv []interface{}) []interface{} {
	out := make([]interface{}, 0, len(kv))
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok && k == loggerKey {
			continue
		}
		out = append(out, kv[i], kv[i+1])
	}
	return out
}

func (s *stageSink) writeLine(level byte, stage, msg string, err error, kv []interface{}) {
	var b bytes.Buffer
	// Header: I0211 14:30:00.123456       1 file.go:123]
	now := time.Now()
	b.WriteByte(level)
	b.WriteString(now.Format("0102 15:04:05.000000"))
	b.WriteString("       1 ")
	if _, file, line, ok := runtime.Caller(s.callDepth + 2); ok {
		b.WriteString(file)
		fmt.Fprintf(&b, ":%d", line)
	}
	b.WriteString("] ")
	// [stage] prefix
	if stage != "" {
		fmt.Fprintf(&b, "[%s] ", stage)
	}
	b.WriteString(msg)
	if err != nil {
		fmt.Fprintf(&b, " err=%q", err.Error())
	}
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(&b, " %v=%v", kv[i], kv[i+1])
	}
	b.WriteByte('\n')
	s.out.Write(b.Bytes())
}

func mergeKV(first, second []interface{}) []interface{} {
	if len(second) == 0 {
		return first
	}
	out := make([]interface{}, 0, len(first)+len(second))
	out = append(out, first...)
	out = append(out, second...)
	if len(out)%2 != 0 {
		out = append(out, "(MISSING)")
	}
	return out
}

// Ensure we implement optional CallDepthLogSink so klog can pass call depth for correct file:line.
var _ logr.CallDepthLogSink = (*stageSink)(nil)

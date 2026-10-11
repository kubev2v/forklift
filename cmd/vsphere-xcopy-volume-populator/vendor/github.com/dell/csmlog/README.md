# csmlog README

## Overview

The csmlog package provides a structured logging library backed by zap, adding features such as automatic request ID extraction and custom formatting.

## Usage

To use the csmlog package in its simplest form, simply import it. The global logger is automatically initialized on package import using environment variables.

### Example 1: Package-Level Convenience Functions

```go
import (
    "github.com/dell/csmlog"
)

func main() {
    csmlog.SetLevel(csmlog.TraceLevel)
    csmlog.SetFormat("text")

    csmlog.Info("Package-level Info message")
    csmlog.Warn("Package-level Warn message")
    csmlog.Error("Package-level Error message")
    csmlog.Debug("Package-level Debug message")
    csmlog.Trace("Package-level Trace message")

    csmlog.Infof("Package-level formatted %s", "message")
    csmlog.Warnf("Package-level formatted %s", "warning")
}
```

**Output:**
```
2026-05-20T20:56:46Z  [INFO] Package-level Info message
2026-05-20T20:56:46Z  [WARN] Package-level Warn message
2026-05-20T20:56:46Z [ERROR] Package-level Error message
2026-05-20T20:56:46Z [DEBUG] Package-level Debug message
2026-05-20T20:56:46Z [DEBUG] Package-level Trace message
2026-05-20T20:56:46Z  [INFO] Package-level formatted message
2026-05-20T20:56:46Z  [WARN] Package-level formatted warning
```

### Example 2: Basic Logging with Global Logger

```go
import (
    "github.com/dell/csmlog"
)

func main() {
    csmlog.SetLevel(csmlog.TraceLevel)
    csmlog.SetFormat("text")

    log := csmlog.GetLogger()
    log.Info("This is an info message")
    log.Warn("This is a warning message")
    log.Error("This is an error message")
    log.Debug("This is a debug message")
    log.Trace("This is a trace message")
}
```

**Output:**
```
2026-05-20T20:56:46Z  [INFO] This is an info message
2026-05-20T20:56:46Z  [WARN] This is a warning message
2026-05-20T20:56:46Z [ERROR] This is an error message
2026-05-20T20:56:46Z [DEBUG] This is a debug message
2026-05-20T20:56:46Z [DEBUG] This is a trace message
```

### Example 3: JSON Format Output

```go
import (
    "github.com/dell/csmlog"
)

func main() {
    csmlog.SetLevel(csmlog.InfoLevel)
    csmlog.SetFormat("json")

    log := csmlog.GetLogger()
    log.WithFields(csmlog.Fields{
        "volume_id": "vol-123",
        "array_id":  "PS001",
        "operation": "create_volume",
        "component": "csi-powerstore",
    }).Info("Volume created successfully")
}
```

**Output:**
```json
{"level":"info","time":"2026-05-20T15:56:46-05:00","msg":"Volume created successfully","volume_id":"vol-123","array_id":"PS001","operation":"create_volume","component":"csi-powerstore"}
```

### Example 4: Helper Methods

```go
import (
    "time"
    "github.com/dell/csmlog"
)

func main() {
    csmlog.SetLevel(csmlog.InfoLevel)
    csmlog.SetFormat("json")

    log := csmlog.GetLogger()
    log = log.WithOperation("DeleteVolume").WithComponent("csi-powerstore")
    log.Info("Deleting volume")

    start := time.Now()
    time.Sleep(50 * time.Millisecond)
    log.TrackDuration(start).Info("Operation completed")
}
```

**Output:**
```json
{"level":"info","time":"2026-05-20T15:56:46-05:00","msg":"Deleting volume","operation":"DeleteVolume","component":"csi-powerstore"}
{"level":"info","time":"2026-05-20T15:56:46-05:00","msg":"Operation completed","operation":"DeleteVolume","component":"csi-powerstore","duration_ms":50}
```

### Example 5: Context with Request ID

```go
import (
    "context"
    "google.golang.org/grpc/metadata"
    "github.com/dell/csmlog"
)

func main() {
    csmlog.SetLevel(csmlog.InfoLevel)
    csmlog.SetFormat("text")

    md := metadata.Pairs(csmlog.RequestIDKey, "req-abc-123")
    ctx := metadata.NewIncomingContext(context.Background(), md)

    log := csmlog.GetLogger()
    log.WithContext(ctx).Info("Request with ID")
}
```

**Output:**
```
2026-05-20T20:56:46Z  [INFO] [ReqID=req-abc-123] Request with ID
```

### Example 6: Instanced Logger (Not Global)

```go
import (
    "bytes"
    "github.com/dell/csmlog"
)

func main() {
    buf := &bytes.Buffer{}

    // Create a custom logger instance with specific output using New()
    customLog := csmlog.New("json", csmlog.InfoLevel, buf)

    customLog = customLog.WithFields(csmlog.Fields{
        "custom_field": "custom_value",
        "instance":     "custom-logger",
    })
    customLog.Info("Using custom logger instance")

    fmt.Printf("Custom logger output:\n%s\n", buf.String())
}
```

**Output:**
```
Custom logger output:
{"level":"info","time":"2026-05-20T15:56:46-05:00","msg":"Using custom logger instance","custom_field":"custom_value","instance":"custom-logger"}
```

### Example 7: Text Format with Fields

```go
import (
    "github.com/dell/csmlog"
)

func main() {
    csmlog.SetLevel(csmlog.InfoLevel)
    csmlog.SetFormat("text")

    log := csmlog.GetLogger()
    log.WithFields(csmlog.Fields{
        "node_id":      "node-001",
        "volume_name":  "my-volume",
        "staging_path": "/tmp/staging",
    }).Info("Volume attached to node")
}
```

**Output:**
```
2026-05-20T20:56:46Z  [INFO] [node_id=node-001] [volume_name=my-volume] [staging_path=/tmp/staging]  Volume attached to node
```

### Example 8: Formatted Messages

```go
import (
    "github.com/dell/csmlog"
)

func main() {
    csmlog.SetLevel(csmlog.InfoLevel)
    csmlog.SetFormat("text")

    log := csmlog.GetLogger()
    log.Infof("Processing %d volumes in array %s", 5, "PS001")
    log.Warnf("Volume %s is in degraded state", "vol-456")
    log.Errorf("Failed to connect to array %s: %v", "PS002", "connection timeout")
}
```

**Output:**
```
2026-05-20T20:56:46Z  [INFO] Processing 5 volumes in array PS001
2026-05-20T20:56:46Z  [WARN] Volume vol-456 is in degraded state
2026-05-20T20:56:46Z [ERROR] Failed to connect to array PS002: connection timeout
```

## Local Logger Instances

While the global logger is convenient for most use cases, creating local instances of `CsmLog` via `csmlog.New()` can be beneficial in specific scenarios:

### When to Use Local Instances

**Isolated Output Destinations**
- Write logs to specific files, buffers, or custom writers without affecting the global logger
- Useful for testing, debugging, or when you need to capture logs from a specific component separately

**Independent Configuration**
- Set different log levels or formats for specific components or operations
- For example, use JSON format for production logs but text format for debug logs in a specific module

**Test Isolation**
- Create isolated loggers in tests to verify log output without polluting test results or global state
- Write to buffers to assert on log content in unit tests

**Concurrent Safety with Different Contexts**
- Each instance maintains its own field set, allowing different components to have different pre-set fields
- Avoids race conditions when multiple goroutines need different field configurations

### Example Use Cases

```go
// Testing: Capture logs to a buffer for assertion
buf := &bytes.Buffer{}
testLogger := csmlog.New("json", csmlog.InfoLevel, buf)
testLogger.Info("Test message")
assert.Contains(t, buf.String(), "Test message")

// Component-specific logging
componentLogger := csmlog.New("json", csmlog.DebugLevel, os.Stderr)
componentLogger = componentLogger.WithFields(csmlog.Fields{
    "component": "storage-manager",
    "version":   "1.2.3",
})

// Separate log file for audit trail
auditFile, _ := os.OpenFile("audit.log", os.O_APPEND|os.O_CREATE, 0644)
auditLogger := csmlog.New("json", csmlog.WarnLevel, auditFile)
auditLogger.WithFields(csmlog.Fields{"audit": true}).Info("User action")
```

## Features

- Automatic request ID extraction: csmlog can automatically extract the request ID from the context and include it in the log message, if the logger is created with a context.
- Custom formatting: csmlog provides a custom formatter that includes the timestamp, log level, and request ID in the log message. This is done automatically to improve log consistency across the product.
- Environment variable configuration: The global logger is automatically initialized on package import using `CSI_LOG_LEVEL` and `CSI_LOG_FORMAT` environment variables.
- Panic and Fatal handling: `Panic`/`Panicf` induce a panic after logging, `Fatal`/`Fatalf` terminate the program after logging.

## Configuration

The global logger is automatically initialized on package import with the following environment variables:

- `CSI_LOG_FORMAT`: Output format ("json" or "text"). Defaults to "json" if not set.
- `CSI_LOG_LEVEL`: Log level ("panic", "fatal", "error", "warn", "info", "debug", "trace"). Defaults to "info" if not set.

Log levels can be changed at runtime using `csmlog.SetLevel()`. The format can be changed using `csmlog.SetFormat()`.

## Replacing logrus with csmlog

1. Any package that previously contained logrus usage can simply import csmlog - the global logger is auto-initialized.
2. Logging calls that previously depended on `logrus` should be switched to `csmlog` calls instead (or, calls on the specific local logging instance, in the cases where one is used).
3. Prepends (such as `log.WithError(err).Error(str)`) do not follow the formatting specified, and such lines should be reworked (typically by including the `err.Error()` as a part of the string). Alternatively, additional work can be done to make the field-extraction logic of the calls more robust, but the original intent was for uniformity in the fields displayed per log line.

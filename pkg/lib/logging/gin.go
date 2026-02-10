package logging

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// ginLogEntry is the JSON structure for Gin request log lines.
// Fields are chosen to match the zap JSON encoder convention
// used by the rest of the forklift controller/inventory pods.
type ginLogEntry struct {
	Level    string `json:"level"`
	TS       string `json:"ts"`
	Logger   string `json:"logger"`
	Msg      string `json:"msg"`
	Status   int    `json:"status"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Latency  string `json:"latency"`
	ClientIP string `json:"clientIP"`
	BodySize int    `json:"bodySize"`
	Error    string `json:"error,omitempty"`
}

// ginRecoveryEntry is the JSON structure for Gin panic recovery log lines.
type ginRecoveryEntry struct {
	Level      string `json:"level"`
	TS         string `json:"ts"`
	Logger     string `json:"logger"`
	Msg        string `json:"msg"`
	Error      string `json:"error"`
	Stacktrace string `json:"stacktrace,omitempty"`
}

const ginTimeFormat = "2006-01-02 15:04:05.000"

// jsonMarshal is the JSON marshaller used by the Gin log helpers.
// It is a package-level variable so tests can swap it to simulate
// marshal failures and exercise fallback paths.
var jsonMarshal = json.Marshal

// jsonLogFormatter returns a gin.LogFormatter that outputs each request
// as a single JSON line to match the zap JSON convention.
func jsonLogFormatter(params gin.LogFormatterParams) string {
	entry := ginLogEntry{
		Level:    "info",
		TS:       params.TimeStamp.Format(ginTimeFormat),
		Logger:   "gin",
		Msg:      "request",
		Status:   params.StatusCode,
		Method:   params.Method,
		Path:     params.Path,
		Latency:  params.Latency.String(),
		ClientIP: params.ClientIP,
		BodySize: params.BodySize,
	}

	if params.ErrorMessage != "" {
		entry.Error = params.ErrorMessage
		if params.StatusCode >= http.StatusInternalServerError {
			entry.Level = "error"
		}
	}

	b, err := jsonMarshal(entry)
	if err != nil {
		// Fallback: should never happen with these simple types.
		return fmt.Sprintf(`{"level":"error","ts":"%s","logger":"gin","msg":"failed to marshal log entry","error":"%s","status":%d,"method":"%s","path":"%s","latency":"%s","clientIP":"%s","bodySize":%d}`+"\n",
			time.Now().Format(ginTimeFormat), err.Error(),
			params.StatusCode, params.Method, params.Path,
			params.Latency.String(), params.ClientIP, params.BodySize)
	}
	return string(b) + "\n"
}

// jsonRecoveryHandler logs panics as JSON error lines and aborts with 500.
func jsonRecoveryHandler(c *gin.Context, err any) {
	entry := ginRecoveryEntry{
		Level:      "error",
		TS:         time.Now().Format(ginTimeFormat),
		Logger:     "gin",
		Msg:        "panic recovered",
		Error:      fmt.Sprintf("%v", err),
		Stacktrace: string(debug.Stack()),
	}

	b, marshalErr := jsonMarshal(entry)
	if marshalErr == nil {
		fmt.Fprintln(os.Stderr, string(b))
	} else {
		fmt.Fprintf(os.Stderr, `{"level":"error","ts":"%s","logger":"gin","msg":"panic recovered","error":"%v","marshalError":"%v"}`+"\n",
			time.Now().Format(ginTimeFormat), err, marshalErr)
	}

	c.AbortWithStatus(http.StatusInternalServerError)
}

// GinEngine returns a *gin.Engine configured with JSON-formatted Logger
// and Recovery middleware. It is a drop-in replacement for gin.Default().
//
// In development mode (LOG_DEVELOPMENT=true) the standard Gin text
// formatters are used instead, preserving colorized console output.
func GinEngine() *gin.Engine {
	if !Settings.Development {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	if Settings.Development {
		// Development: use default Gin text logger and recovery.
		engine.Use(gin.Logger(), gin.Recovery())
	} else {
		// Production: JSON logger writing to stderr, JSON recovery.
		engine.Use(gin.LoggerWithConfig(gin.LoggerConfig{
			Formatter: jsonLogFormatter,
			Output:    os.Stderr,
		}))
		engine.Use(gin.CustomRecovery(jsonRecoveryHandler))
	}

	return engine
}

// Co-authored-by: Claude <noreply@anthropic.com>

package logging

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onsi/gomega"
)

// ---------- jsonLogFormatter ----------

func TestJsonLogFormatter(t *testing.T) {
	t.Run("successful 200 request", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		ts := time.Date(2025, 6, 15, 10, 30, 45, 123000000, time.UTC)
		params := gin.LogFormatterParams{
			TimeStamp:  ts,
			StatusCode: http.StatusOK,
			Method:     "GET",
			Path:       "/api/v1/resources",
			Latency:    42 * time.Millisecond,
			ClientIP:   "192.168.1.100",
			BodySize:   256,
		}

		result := jsonLogFormatter(params)

		var entry ginLogEntry
		err := json.Unmarshal([]byte(result), &entry)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(entry.Level).To(gomega.Equal("info"))
		g.Expect(entry.Logger).To(gomega.Equal("gin"))
		g.Expect(entry.Msg).To(gomega.Equal("request"))
		g.Expect(entry.Status).To(gomega.Equal(200))
		g.Expect(entry.Method).To(gomega.Equal("GET"))
		g.Expect(entry.Path).To(gomega.Equal("/api/v1/resources"))
		g.Expect(entry.Latency).To(gomega.Equal("42ms"))
		g.Expect(entry.ClientIP).To(gomega.Equal("192.168.1.100"))
		g.Expect(entry.BodySize).To(gomega.Equal(256))
		g.Expect(entry.Error).To(gomega.BeEmpty())
	})

	t.Run("request with error status < 500", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		params := gin.LogFormatterParams{
			TimeStamp:    time.Now(),
			StatusCode:   http.StatusBadRequest,
			Method:       "POST",
			Path:         "/api/v1/items",
			Latency:      5 * time.Millisecond,
			ClientIP:     "10.0.0.1",
			BodySize:     0,
			ErrorMessage: "invalid request body",
		}

		result := jsonLogFormatter(params)

		var entry ginLogEntry
		err := json.Unmarshal([]byte(result), &entry)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(entry.Level).To(gomega.Equal("info"))
		g.Expect(entry.Error).To(gomega.Equal("invalid request body"))
		g.Expect(entry.Status).To(gomega.Equal(400))
	})

	t.Run("request with error status >= 500", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		params := gin.LogFormatterParams{
			TimeStamp:    time.Now(),
			StatusCode:   http.StatusBadGateway,
			Method:       "GET",
			Path:         "/api/v1/proxy",
			Latency:      30 * time.Second,
			ClientIP:     "10.0.0.2",
			BodySize:     0,
			ErrorMessage: "upstream timeout",
		}

		result := jsonLogFormatter(params)

		var entry ginLogEntry
		err := json.Unmarshal([]byte(result), &entry)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(entry.Level).To(gomega.Equal("error"))
		g.Expect(entry.Error).To(gomega.Equal("upstream timeout"))
		g.Expect(entry.Status).To(gomega.Equal(502))
	})

	t.Run("timestamp formatting", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		ts := time.Date(2025, 1, 2, 3, 4, 5, 678000000, time.UTC)
		params := gin.LogFormatterParams{
			TimeStamp:  ts,
			StatusCode: http.StatusOK,
			Method:     "GET",
			Path:       "/",
		}

		result := jsonLogFormatter(params)

		var entry ginLogEntry
		err := json.Unmarshal([]byte(result), &entry)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(entry.TS).To(gomega.Equal("2025-01-02 03:04:05.678"))
	})

	t.Run("zero-value params", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		params := gin.LogFormatterParams{}
		result := jsonLogFormatter(params)

		var entry ginLogEntry
		err := json.Unmarshal([]byte(result), &entry)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(entry.Level).To(gomega.Equal("info"))
		g.Expect(entry.Logger).To(gomega.Equal("gin"))
		g.Expect(entry.Msg).To(gomega.Equal("request"))
		g.Expect(entry.Status).To(gomega.Equal(0))
		g.Expect(entry.Error).To(gomega.BeEmpty())
	})

	t.Run("output ends with newline", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		params := gin.LogFormatterParams{
			TimeStamp:  time.Now(),
			StatusCode: http.StatusOK,
			Method:     "GET",
			Path:       "/health",
		}

		result := jsonLogFormatter(params)
		g.Expect(result).To(gomega.HaveSuffix("\n"))
	})

	t.Run("marshal failure fallback includes request data", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		// Swap the marshaller to force an error.
		origMarshal := jsonMarshal
		t.Cleanup(func() { jsonMarshal = origMarshal })
		jsonMarshal = func(v any) ([]byte, error) {
			return nil, errors.New("synthetic marshal error")
		}

		params := gin.LogFormatterParams{
			TimeStamp:  time.Now(),
			StatusCode: http.StatusBadGateway,
			Method:     "DELETE",
			Path:       "/api/v1/resources/42",
			Latency:    100 * time.Millisecond,
			ClientIP:   "10.20.30.40",
			BodySize:   512,
		}

		result := jsonLogFormatter(params)

		// The fallback is a hand-crafted JSON line; parse it loosely.
		var fallback map[string]any
		err := json.Unmarshal([]byte(result), &fallback)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(fallback["level"]).To(gomega.Equal("error"))
		g.Expect(fallback["logger"]).To(gomega.Equal("gin"))
		g.Expect(fallback["msg"]).To(gomega.Equal("failed to marshal log entry"))
		g.Expect(fallback["error"]).To(gomega.ContainSubstring("synthetic marshal error"))

		// Verify the original request data is present.
		g.Expect(fallback["status"]).To(gomega.BeNumerically("==", 502))
		g.Expect(fallback["method"]).To(gomega.Equal("DELETE"))
		g.Expect(fallback["path"]).To(gomega.Equal("/api/v1/resources/42"))
		g.Expect(fallback["latency"]).To(gomega.Equal("100ms"))
		g.Expect(fallback["clientIP"]).To(gomega.Equal("10.20.30.40"))
		g.Expect(fallback["bodySize"]).To(gomega.BeNumerically("==", 512))

		g.Expect(result).To(gomega.HaveSuffix("\n"))
	})
}

// ---------- jsonRecoveryHandler ----------

// captureStderr redirects os.Stderr to a pipe, runs fn, then returns
// whatever was written to stderr. Must be called from a single goroutine.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = origStderr

	scanner := bufio.NewScanner(r)
	var output string
	for scanner.Scan() {
		output += scanner.Text() + "\n"
	}
	r.Close()
	return output
}

func TestJsonRecoveryHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("panic with string error", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		stderr := captureStderr(t, func() {
			jsonRecoveryHandler(c, "something broke")
		})

		// Verify HTTP 500 response.
		g.Expect(w.Code).To(gomega.Equal(http.StatusInternalServerError))

		// Verify JSON on stderr.
		var entry ginRecoveryEntry
		err := json.Unmarshal([]byte(stderr), &entry)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(entry.Level).To(gomega.Equal("error"))
		g.Expect(entry.Logger).To(gomega.Equal("gin"))
		g.Expect(entry.Msg).To(gomega.Equal("panic recovered"))
		g.Expect(entry.Error).To(gomega.Equal("something broke"))
		g.Expect(entry.Stacktrace).NotTo(gomega.BeEmpty())
	})

	t.Run("panic with integer error", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		stderr := captureStderr(t, func() {
			jsonRecoveryHandler(c, 42)
		})

		g.Expect(w.Code).To(gomega.Equal(http.StatusInternalServerError))

		var entry ginRecoveryEntry
		err := json.Unmarshal([]byte(stderr), &entry)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(entry.Error).To(gomega.Equal("42"))
	})

	t.Run("panic with error value", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		stderr := captureStderr(t, func() {
			jsonRecoveryHandler(c, http.ErrBodyNotAllowed)
		})

		g.Expect(w.Code).To(gomega.Equal(http.StatusInternalServerError))

		var entry ginRecoveryEntry
		err := json.Unmarshal([]byte(stderr), &entry)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(entry.Error).To(gomega.Equal(http.ErrBodyNotAllowed.Error()))
	})

	t.Run("marshal failure fallback includes error and marshalError", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		// Swap the marshaller to force an error.
		origMarshal := jsonMarshal
		t.Cleanup(func() { jsonMarshal = origMarshal })
		jsonMarshal = func(v any) ([]byte, error) {
			return nil, errors.New("synthetic marshal error")
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		stderr := captureStderr(t, func() {
			jsonRecoveryHandler(c, "db connection lost")
		})

		g.Expect(w.Code).To(gomega.Equal(http.StatusInternalServerError))

		// The fallback is a hand-crafted JSON line; parse it loosely.
		var fallback map[string]any
		err := json.Unmarshal([]byte(stderr), &fallback)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(fallback["level"]).To(gomega.Equal("error"))
		g.Expect(fallback["logger"]).To(gomega.Equal("gin"))
		g.Expect(fallback["msg"]).To(gomega.Equal("panic recovered"))
		g.Expect(fallback["error"]).To(gomega.Equal("db connection lost"))
		g.Expect(fallback["marshalError"]).To(gomega.ContainSubstring("synthetic marshal error"))
	})
}

// ---------- GinEngine ----------

func TestGinEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("production mode", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		origDev := Settings.Development
		t.Cleanup(func() {
			Settings.Development = origDev
			gin.SetMode(gin.TestMode)
		})

		Settings.Development = false
		engine := GinEngine()

		g.Expect(gin.Mode()).To(gomega.Equal(gin.ReleaseMode))

		// Register a route and verify the engine works with middleware.
		handler := func(c *gin.Context) {
			c.String(http.StatusOK, "pong")
		}
		engine.GET("/ping", handler)

		// Verify the engine serves requests correctly.
		req := httptest.NewRequest("GET", "/ping", nil)
		w := httptest.NewRecorder()
		captureStderr(t, func() {
			engine.ServeHTTP(w, req)
		})
		g.Expect(w.Code).To(gomega.Equal(http.StatusOK))
		g.Expect(w.Body.String()).To(gomega.Equal("pong"))
	})

	t.Run("development mode", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		origDev := Settings.Development
		t.Cleanup(func() {
			Settings.Development = origDev
			gin.SetMode(gin.TestMode)
		})

		Settings.Development = true
		engine := GinEngine()

		// GinEngine skips gin.SetMode(ReleaseMode) in development,
		// so the mode must NOT be release.
		g.Expect(gin.Mode()).NotTo(gomega.Equal(gin.ReleaseMode))

		// Register a route and verify the engine works with middleware.
		engine.GET("/ping", func(c *gin.Context) {
			c.String(http.StatusOK, "pong")
		})

		req := httptest.NewRequest("GET", "/ping", nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		g.Expect(w.Code).To(gomega.Equal(http.StatusOK))
		g.Expect(w.Body.String()).To(gomega.Equal("pong"))
	})

	t.Run("production mode produces JSON logs", func(t *testing.T) {
		g := gomega.NewGomegaWithT(t)

		origDev := Settings.Development
		t.Cleanup(func() {
			Settings.Development = origDev
			gin.SetMode(gin.TestMode)
		})

		Settings.Development = false

		// The engine must be created inside captureStderr because
		// gin.LoggerConfig.Output captures os.Stderr at creation time.
		var w *httptest.ResponseRecorder
		stderr := captureStderr(t, func() {
			engine := GinEngine()
			engine.GET("/health", func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest("GET", "/health", nil)
			w = httptest.NewRecorder()
			engine.ServeHTTP(w, req)
		})

		g.Expect(w.Code).To(gomega.Equal(http.StatusOK))
		g.Expect(w.Body.String()).To(gomega.Equal("ok"))

		// The JSON logger writes to stderr; verify the output is valid JSON.
		var entry ginLogEntry
		err := json.Unmarshal([]byte(stderr), &entry)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		g.Expect(entry.Level).To(gomega.Equal("info"))
		g.Expect(entry.Logger).To(gomega.Equal("gin"))
		g.Expect(entry.Msg).To(gomega.Equal("request"))
		g.Expect(entry.Status).To(gomega.Equal(200))
		g.Expect(entry.Method).To(gomega.Equal("GET"))
		g.Expect(entry.Path).To(gomega.Equal("/health"))
	})
}

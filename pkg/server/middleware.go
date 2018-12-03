package server

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// WithCORS enables CORS
func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Link")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")
		next.ServeHTTP(w, r)
	})
}

// WithPrometheus returns a http.Handler that captures telemetry requestMetrics for every request.
func WithPrometheus(handlerName string, requestMetrics *HttpMetrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		nw := &ResponseWriter{ResponseWriter: w}
		next.ServeHTTP(nw, r)

		rspCode := strconv.Itoa(nw.status)
		requestMetrics.handledTotalCount.WithLabelValues(handlerName, rspCode, r.Method, r.URL.Path).Inc()
		requestMetrics.handledHistogram.WithLabelValues(handlerName, rspCode, r.Method, r.URL.Path).Observe(time.Since(startTime).Seconds())
	})
}

// WithDebugger prints request data and response code to debug log
func WithDebugger(logger log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := httputil.DumpRequest(r, true)
		if err != nil {
			b = []byte(fmt.Sprintf("failed to dump request: %s", err))
		}
		nw := &ResponseWriter{ResponseWriter: w}
		next.ServeHTTP(nw, r)

		level.Debug(logger).Log("message", "request", "content", string(b), "responseCode", nw.status)
	})
}

type leaderable interface {
	IsLeader() bool
}

func WithLeaderOnly(leaderPathPrefix string, state interface{}, next http.Handler) http.Handler {
	subj, ok := state.(leaderable)
	if !ok {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, leaderPathPrefix) && !subj.IsLeader() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// OKHandler returns status 200 code only
func OKHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type ResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	if !w.Written() {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *ResponseWriter) WriteHeader(s int) {
	w.status = s
	w.ResponseWriter.WriteHeader(s)
}

func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("the ResponseWriter doesn't support the Hijacker interface")
	}
	return hijacker.Hijack()
}

func (w *ResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		if !w.Written() {
			w.WriteHeader(http.StatusOK)
		}
		flusher.Flush()
	}
}

func (w *ResponseWriter) Written() bool {
	return w.status != 0
}

func (w *ResponseWriter) CloseNotify() <-chan bool {
	if n, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return n.CloseNotify()
	}
	return nil
}

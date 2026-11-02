package logger

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type (
	loggerKey      struct{}
	requestMetaKey struct{}
)

type requestMeta struct {
	UserID string
}

// Logger returns the per-request *slog.Logger with request_id pre-populated.
// Falls back to slog.Default() when ctx has no logger (e.g. background jobs).
func Logger(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// Call after a successful token parse so the request summary line includes the user.
func AnnotateUser(ctx context.Context, userID string) {
	if m, ok := ctx.Value(requestMetaKey{}).(*requestMeta); ok {
		m.UserID = userID
	}
}

// RequestLogger is a net/http middleware that logs every request.
// Logged fields: method, path, status, latency_ms, request_id, user_id (when authenticated).
// Not logged: cookies, Authorization header, request body, query string.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := uuid.New().String()

		meta := &requestMeta{}
		l := slog.Default().With("request_id", requestID)

		ctx := r.Context()
		ctx = context.WithValue(ctx, requestMetaKey{}, meta)
		ctx = context.WithValue(ctx, loggerKey{}, l)
		r = r.WithContext(ctx)

		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		level := slog.LevelInfo
		if rw.status >= 500 {
			level = slog.LevelError
		}

		args := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"latency_ms", time.Since(start).Milliseconds(),
		}
		if meta.UserID != "" {
			args = append(args, "user_id", meta.UserID)
		}

		l.Log(r.Context(), level, "request", args...)
	})
}

type statusRecorder struct {
	http.ResponseWriter

	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

// VisitRecorder persists one page view / API request for the admin metrics
// dashboard. Record is best-effort: middleware must not fail the request when
// the write fails.
type VisitRecorder interface {
	Record(ctx context.Context, userID *int, ip, method, path string, status int) error
}

// visitResponseWriter captures the status code written by the handler chain.
type visitResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *visitResponseWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *visitResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *visitResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// VisitMiddleware records requests served by the API so admins can see visit
// volume and active users. Mount it AFTER AuthTokenMiddleware so the
// authenticated user is already in context (this app gates every meaningful
// page behind login; anonymous /auth, /media and /stream routes sit outside
// the protected tree and are not counted). Only GETs are recorded — they are
// the page-view proxy. /admin/* is excluded to avoid the dashboard's own 5s
// poll polluting the table.
func VisitMiddleware(recorder VisitRecorder, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || shouldSkipVisit(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			wrapped := &visitResponseWriter{ResponseWriter: w}
			next.ServeHTTP(wrapped, r)

			var userID *int
			if user, err := GetAuthenticatedUserFromContext(r); err == nil && user != nil {
				id := user.ID
				userID = &id
			}
			if err := recorder.Record(r.Context(), userID, clientIP(r), r.Method, r.URL.Path, wrapped.status); err != nil {
				logger.Warn("failed to record page view", "error", err, "path", r.URL.Path)
			}
		})
	}
}

// shouldSkipVisit excludes the admin area (feedback loop) and anything that is
// not a real page view.
func shouldSkipVisit(path string) bool {
	return strings.HasPrefix(path, "/api/v1/admin")
}

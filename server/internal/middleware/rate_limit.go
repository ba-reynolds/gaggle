package middleware

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ba-reynolds/gophersocial/internal/apperrors"
	"github.com/ba-reynolds/gophersocial/internal/cache"
	"github.com/ba-reynolds/gophersocial/internal/util"
)

// RateLimitMiddleware limits requests per client IP for a given route prefix.
// Used on auth endpoints (register/login/refresh) to slow down brute force.
func RateLimitMiddleware(rdb *cache.Client, maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rdb == nil || maxRequests <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := clientIP(r)
			key := "rl:" + r.URL.Path + ":" + clientIP
			if !rdb.Allow(r.Context(), key, maxRequests, window) {
				util.RespondWithAppError(w, apperrors.TooManyRequestsError())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	// Respect common proxies, falling back to RemoteAddr.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

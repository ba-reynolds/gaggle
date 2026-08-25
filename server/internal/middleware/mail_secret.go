package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// OrchidSecretMiddleware gates the mail-intake routes on the shared
// x-orchid-secret header (the INTAKE_SECRET also configured in the Cloudflare
// Email Worker and orchid's ORCHID_MAIL_SINK_TOKEN). The comparison is
// constant-time and the secret value is never logged or echoed back.
//
// An empty configured secret FAILS CLOSED (every request 401s): the intake
// stores verification codes, so it must never serve unauthenticated traffic,
// even by accident in a dev environment.
func OrchidSecretMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("x-orchid-secret")
			configured := secret != ""
			match := subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) == 1
			if !configured || !match {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

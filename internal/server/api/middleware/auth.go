package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const cookieName = "norn_secret"

// Auth returns middleware that requires a valid secret in cookie.
func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value != secret {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GenerateSecret creates a random 32-byte hex secret.
func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

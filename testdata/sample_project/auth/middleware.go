package auth

import (
	"net/http"
	"strings"
)

// AuthMiddleware validates Bearer tokens before passing requests to the next handler.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !validateToken(token) {
			http.Error(w, "invalid token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func validateToken(token string) bool {
	// In production, this would verify JWT signature and expiration.
	return len(token) > 0
}

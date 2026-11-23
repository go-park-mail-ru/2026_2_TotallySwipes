package middleware

import (
	"crypto/subtle"
	"net/http"
)

func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("csrf_token")
		token := r.Header.Get("X-CSRF-Token")

		if err != nil || cookie.Value == "" || token == "" {
			writeCSRFForbidden(w)
			return
		}

		if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(token)) != 1 {
			writeCSRFForbidden(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeCSRFForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":{"code":"CSRF_INVALID","message":"Отсутствует или неверный CSRF-токен"}}`))
}

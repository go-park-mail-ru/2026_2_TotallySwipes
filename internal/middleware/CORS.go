package middleware

import (
	"net/http"
	"strings"
)

func CORS(allowOrigins []string) func(http.Handler) http.Handler {
	origins := make(map[string]bool, len(allowOrigins))

	for _, origin := range allowOrigins {
		origins[origin] = true
	}

	allowedMethods := map[string]bool{
		http.MethodGet:    true,
		http.MethodPost:   true,
		http.MethodPut:    true,
		http.MethodPatch:  true,
		http.MethodDelete: true,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")

			origin := r.Header.Get("Origin")

			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			isPreflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""

			if isPreflight {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
			}

			if !origins[origin] {
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if !isPreflight {
				next.ServeHTTP(w, r)
				return
			}

			method := r.Header.Get("Access-Control-Request-Method")
			if !allowedMethods[method] {
				http.Error(w, "Method not allowed", http.StatusForbidden)
				return
			}

			headers := r.Header.Get("Access-Control-Request-Headers")
			if headers != "" {
				for _, header := range strings.Split(headers, ",") {
					if !strings.EqualFold(strings.TrimSpace(header), "Content-Type") {
						http.Error(w, "Header not allowed", http.StatusForbidden)
						return
					}
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)

		})
	}
}

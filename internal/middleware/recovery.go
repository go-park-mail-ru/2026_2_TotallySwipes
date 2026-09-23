package middleware

import (
	"log/slog"
	"net/http"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err, "path", r.URL.Path, "method", r.Method)

				// Если ответ уже частично отправлен (заголовки или часть тела),
				// net/http всё равно проигнорирует повторный WriteHeader, а наш
				// JSON просто приклеится к уже ушедшим байтам — писать бессмысленно.
				if rec, ok := w.(*statusRecorder); ok && rec.written {
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

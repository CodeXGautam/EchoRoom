package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func readyHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := map[string]string{"postgres": "ok", "redis": "ok"}
		code := http.StatusOK
		if err := deps.PG.Ping(ctx); err != nil {
			status["postgres"] = "down"
			code = http.StatusServiceUnavailable
		}
		if err := deps.Redis.Ping(ctx).Err(); err != nil {
			status["redis"] = "down"
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(status)
	}
}

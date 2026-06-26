package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type HealthResponse struct {
	Status string `json:"status"`
}

func HealthzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, HealthResponse{
			Status: "ok",
		})
	})
}

func ReadyzHandler(redisClient *redis.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if redisClient != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
			defer cancel()

			if err := redisClient.Ping(ctx).Err(); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, HealthResponse{
					Status: "redis_unavailable",
				})
				return
			}
		}

		writeJSON(w, http.StatusOK, HealthResponse{
			Status: "ready",
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}
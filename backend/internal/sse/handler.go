package sse

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/lynk/backend/internal/auth"
)

const heartbeatInterval = 15 * time.Second

// StreamHandler returns an http.HandlerFunc that streams Server-Sent Events to
// the authenticated client. Authentication is performed via the SuperTokens
// session claims on the request context.
func StreamHandler(broker *Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Require authenticated user
		claims, err := auth.GetUserContext(r.Context())
		if err != nil || claims == nil || claims.UserID == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Verify that the ResponseWriter supports flushing
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, `{"error":"streaming unsupported"}`, http.StatusInternalServerError)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if behind proxy

		// Subscribe to broker
		ch := broker.Subscribe(claims.UserID)
		defer broker.Unsubscribe(claims.UserID, ch)

		// Send an initial connected event to confirm stream is live
		if _, err := fmt.Fprintf(w, "event: connected\ndata: {}\n\n"); err != nil {
			return
		}
		flusher.Flush()

		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return

			case <-ticker.C:
				// Heartbeat ping to prevent proxy timeouts
				if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
					return
				}
				flusher.Flush()

			case msg, open := <-ch:
				if !open {
					return
				}

				data, err := json.Marshal(msg.Data)
				if err != nil {
					slog.Warn("sse: failed to marshal event data", "error", err)
					continue
				}

				if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.Event, data); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

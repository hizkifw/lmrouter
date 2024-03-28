package hub

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hizkifw/lmrouter/message"
)

func RunServer(addr string) error {
	// Create the hub
	var hub = Hub{
		workers: make(map[uuid.UUID]*Worker),
	}

	// Begin background processes
	go hub.PingLoop()

	// Handle the completions endpoint
	http.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		// Parse the completions request
		req := message.CompletionsRequest{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Failed to parse request", http.StatusBadRequest)
			return
		}

		// Request completions from the workers
		hub.RequestCompletions(req, w, r.Context())
	})

	// Handle the worker websocket endpoint
	http.HandleFunc("/internal/v1/worker/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWorkerWS(&hub, w, r)
	})

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Listening on %s", addr)
	return server.ListenAndServe()
}

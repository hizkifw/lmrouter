package hub

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hizkifw/lmrouter/message"
)

//go:embed index.html
var indexHTML []byte

type ServerOpts struct {
	// Addr is the address to listen on
	Addr string `arg:"--listen" help:"address to listen on" default:":9090"`
}

func RunServer(opts *ServerOpts) error {
	// Create the hub
	var hub = Hub{
		workers: make(map[uuid.UUID]*Worker),
	}

	// Begin background processes
	go hub.PingLoop()

	// Index page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(indexHTML)
	})

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

	http.HandleFunc("/internal/v1/workers", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(hub.GetWorkers())
	})

	server := &http.Server{
		Addr:              opts.Addr,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Listening on %s", opts.Addr)
	return server.ListenAndServe()
}

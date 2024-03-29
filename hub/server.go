package hub

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
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

func RunServer(opts *ServerOpts, ctx context.Context) error {
	// Create the hub
	var hub = Hub{
		workers: make(map[uuid.UUID]*Worker),
	}

	// Begin background processes
	go hub.PingLoop()

	mux := http.NewServeMux()

	// Index page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(indexHTML)
	})

	// Handle the completions endpoint
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		// Parse the completions request
		req := message.CompletionsRequest{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Failed to parse request", http.StatusBadRequest)
			return
		}

		// Request completions from the workers
		hub.RequestCompletions(req, w, r.Context())
	})

	// Handle the list models endpoint
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		resp := message.ListModelsResponse{
			Object: "list",
			Data:   hub.GetAllModels(),
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Handle the worker websocket endpoint
	mux.HandleFunc("/internal/v1/worker/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWorkerWS(&hub, w, r)
	})

	mux.HandleFunc("/internal/v1/workers", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(hub.GetWorkers())
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler:           mux,
	}
	listener, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", opts.Addr, err)
	}
	log.Printf("Listening on %s", listener.Addr().String())
	go func() {
		server.Serve(listener)
		cancel()
	}()

	for {
		select {
		case <-interrupt:
			cancel()

		case <-ctx.Done():
			log.Println("interrupt")

			// Close the server
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				log.Println("shutdown:", err)
				return err
			}

			return nil
		}
	}
}

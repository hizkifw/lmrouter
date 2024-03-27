package hub

import (
	"log"
	"net/http"
	"strings"

	"github.com/hizkifw/lmrouter/message"
)

type Hub struct {
	workers []*Worker
}

var hub = Hub{
	workers: make([]*Worker, 0),
}

func (h *Hub) RegisterWorker(worker *Worker) {
	h.workers = append(h.workers, worker)
}

func (h *Hub) RequestCompletions(req message.CompletionsRequest, w http.ResponseWriter) {
	// TODO: worker selection, for now just use the first worker
	if len(h.workers) == 0 {
		http.Error(w, "No workers available", http.StatusServiceUnavailable)
		return
	}

	worker := h.workers[0]

	// Request completions from the worker
	if err := worker.RequestCompletions(req, w); err != nil {
		if strings.Contains(err.Error(), "websocket: close") || strings.Contains(err.Error(), "write: broken pipe") {
			log.Printf("Worker connection closed, retrying request: %v", err)

			// Worker connection closed, remove it from the hub
			for i, w := range h.workers {
				if w == worker {
					h.workers = append(h.workers[:i], h.workers[i+1:]...)
					break
				}
			}

			// Retry the request
			h.RequestCompletions(req, w)
		} else {
			log.Printf("Failed to request completions: %v", err)
		}
	}
}

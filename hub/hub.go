package hub

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hizkifw/lmrouter/message"
)

type Hub struct {
	workers     map[uuid.UUID]*Worker
	workersLock sync.Mutex
}

func (h *Hub) PingLoop() {
	for {
		h.workersLock.Lock()
		workersList := make([]*Worker, 0, len(h.workers))
		for _, worker := range h.workers {
			workersList = append(workersList, worker)
		}
		h.workersLock.Unlock()

		for _, worker := range workersList {
			id, err := message.Send[string](worker.Mbuf, &message.TypedMessage[string]{
				Type:    message.MTPing,
				Message: "ping",
			})
			if err != nil {
				log.Printf("Failed to send ping to worker %v: %v", worker.Id, err)
				worker.Conn.Close()
				h.UnregisterWorker(worker.Id)
				continue
			}

			reply, err := message.ReceiveId[string](worker.Mbuf, id, context.Background())
			if err != nil {
				log.Printf("Failed to receive ping reply from worker %v: %v", worker.Id, err)
				worker.Conn.Close()
				h.UnregisterWorker(worker.Id)
				continue
			}

			if reply.Type != message.MTAck {
				log.Printf("Invalid ping reply from worker %v: %v", worker.Id, reply)
				worker.Conn.Close()
				h.UnregisterWorker(worker.Id)
				continue
			}
		}

		time.Sleep(5 * time.Second)
	}
}

func (h *Hub) RegisterWorker(worker *Worker) {
	h.workersLock.Lock()
	defer h.workersLock.Unlock()
	h.workers[worker.Id] = worker
	worker.Conn.SetCloseHandler(func(code int, text string) error {
		h.UnregisterWorker(worker.Id)
		return nil
	})
	log.Printf("Registered worker %v", worker.Id)
}

func (h *Hub) UnregisterWorker(id uuid.UUID) {
	h.workersLock.Lock()
	defer h.workersLock.Unlock()

	if _, ok := h.workers[id]; !ok {
		return
	}

	delete(h.workers, id)
	log.Printf("Unregistered worker %v", id)
}

func (h *Hub) RequestCompletions(req message.CompletionsRequest, w http.ResponseWriter, ctx context.Context) {
	if len(h.workers) == 0 {
		http.Error(w, "No workers available", http.StatusServiceUnavailable)
		return
	}

	// Find the worker with the least active tasks
	var worker *Worker = nil
	for _, w := range h.workers {
		if worker == nil || w.GetActiveTasks() < worker.GetActiveTasks() {
			worker = w
		}
	}

	// Request completions from the worker
	if err := worker.RequestCompletions(req, w, ctx); err != nil {
		if strings.Contains(err.Error(), "websocket: close") || strings.Contains(err.Error(), "write: broken pipe") {
			log.Printf("Worker connection closed, retrying request: %v", err)

			// Worker connection closed, remove it from the hub
			h.UnregisterWorker(worker.Id)

			// Retry the request
			if ctx.Err() == nil {
				h.RequestCompletions(req, w, ctx)
			}
		} else {
			log.Printf("Failed to request completions: %v", err)
		}
	}
}

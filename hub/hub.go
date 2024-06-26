package hub

import (
	"context"
	"fmt"
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

func (h *Hub) GetWorkers() []*Worker {
	h.workersLock.Lock()
	defer h.workersLock.Unlock()

	workerList := make([]*Worker, 0, len(h.workers))
	for _, worker := range h.workers {
		workerList = append(workerList, worker)
	}

	return workerList
}

func (h *Hub) GetAllModels() []message.Model {
	workersList := h.GetWorkers()
	models := make([]message.Model, 0)
	inserted := make(map[string]bool)
	for _, worker := range workersList {
		for _, model := range worker.Info.AvailableModels {
			key := fmt.Sprintf("%s/%s", model.OwnedBy, model.Id)
			if _, ok := inserted[key]; !ok {
				models = append(models, model)
				inserted[key] = true
			}
		}
	}
	return models

}

func (h *Hub) PingLoop() {
	for {
		workersList := h.GetWorkers()
		for _, worker := range workersList {
			id, err := message.Send[string](worker.mbuf, &message.TypedMessage[string]{
				Type:    message.MTPing,
				Message: "ping",
			})
			if err != nil {
				log.Printf("Failed to send ping to worker %v: %v", worker.Id, err)
				worker.conn.Close()
				h.UnregisterWorker(worker.Id)
				continue
			}

			reply, err := message.ReceiveId[string](worker.mbuf, id, context.Background())
			if err != nil {
				log.Printf("Failed to receive ping reply from worker %v: %v", worker.Id, err)
				worker.conn.Close()
				h.UnregisterWorker(worker.Id)
				continue
			}

			if reply.Type != message.MTAck {
				log.Printf("Invalid ping reply from worker %v: %v", worker.Id, reply)
				worker.conn.Close()
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
	worker.conn.SetCloseHandler(func(code int, text string) error {
		h.UnregisterWorker(worker.Id)
		return nil
	})
	log.Printf("Registered worker %v", worker.Id)
}

func (h *Hub) UnregisterWorker(id uuid.UUID) {
	h.workersLock.Lock()
	defer h.workersLock.Unlock()

	if worker, ok := h.workers[id]; ok {
		delete(h.workers, id)

		worker.mbuf.Close()
		log.Printf("Unregistered worker %v", id)
	}
}

func (h *Hub) RequestCompletions(req message.CompletionsRequest, w http.ResponseWriter, ctx context.Context) {
	if len(h.workers) == 0 {
		http.Error(w, "No workers available", http.StatusServiceUnavailable)
		return
	}

	// Find the worker with the least active tasks
	var worker *Worker = nil
	for _, w := range h.workers {
		if !w.HasModel(req.Model) {
			continue
		}

		if worker == nil || w.GetActiveTasks() < worker.GetActiveTasks() {
			worker = w
		}
	}

	if worker == nil {
		http.Error(w, "No workers available for model", http.StatusServiceUnavailable)
		return
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
			http.Error(w, "Failed to request completions", http.StatusInternalServerError)
		}
	}
}

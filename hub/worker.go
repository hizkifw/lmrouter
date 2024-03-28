package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hizkifw/lmrouter/message"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Worker struct {
	Id   uuid.UUID          `json:"id"`
	Info message.WorkerInfo `json:"info"`

	conn            *websocket.Conn
	mbuf            *message.MessageBuffer
	activeTasks     int
	activeTasksLock sync.Mutex
}

func (w *Worker) HasModel(modelId string) bool {
	for _, model := range w.Info.AvailableModels {
		if model.Id == modelId {
			return true
		}
	}
	return false
}

func (w *Worker) GetActiveTasks() int {
	w.activeTasksLock.Lock()
	defer w.activeTasksLock.Unlock()
	return w.activeTasks
}

func (w *Worker) RequestCompletions(cr message.CompletionsRequest, wr http.ResponseWriter, ctx context.Context) error {
	w.activeTasksLock.Lock()
	w.activeTasks++
	w.activeTasksLock.Unlock()
	defer func() {
		w.activeTasksLock.Lock()
		w.activeTasks--
		w.activeTasksLock.Unlock()
	}()

	// Request completions from the worker
	id, err := message.Send[message.CompletionsRequest](w.mbuf, &message.TypedMessage[message.CompletionsRequest]{
		Type:    message.MTCompletionsRequest,
		Message: cr,
	})
	if err != nil {
		return fmt.Errorf("failed to send completions request to worker: %w", err)
	}
	log.Printf("Sending completions request %s to worker %s", id, w.Id)

	// Write headers
	wr.Header().Set("Cache-Control", "no-cache")
	if cr.Stream {
		wr.Header().Set("Content-Type", "text/event-stream")
		wr.Header().Set("Connection", "keep-alive")
	} else {
		wr.Header().Set("Content-Type", "application/json")
	}

	// Wait for the response
	processing := true
	headersSent := false
	for processing {
		resp, err := message.ReceiveId[message.CompletionsResponse](w.mbuf, id, ctx)
		if err != nil {
			return fmt.Errorf("failed to read response from worker: %w", err)
		}
		if resp.Type != message.MTCompletionsResponse {
			return fmt.Errorf("expected completions_response message, got %v", resp.Type)
		}

		// Write the response
		if !headersSent {
			wr.WriteHeader(http.StatusOK)
			headersSent = true
		}

		if cr.Stream {
			wr.Write([]byte("data: "))
		}

		respBytes, err := json.Marshal(resp.Message)
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}

		wr.Write(respBytes)

		if cr.Stream {
			wr.Write([]byte("\n\n"))
			if f, ok := wr.(http.Flusher); ok {
				f.Flush()
			}
		} else {
			processing = false
		}

		if len(resp.Message.Choices) > 0 && resp.Message.Choices[0].FinishReason != nil {
			processing = false
		}
	}

	return nil
}

func handleWorkerWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// Upgrade the connection to a websocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection to websocket: %v", err)
		return
	}

	// Send the welcome message
	mb := message.NewMessageBuffer(conn)
	go mb.RecvLoop()

	_, err = message.Send[message.ServerInfo](mb, &message.TypedMessage[message.ServerInfo]{
		Type: message.MTServerInfo,
		Message: message.ServerInfo{
			ServerName:    "blegh",
			ServerVersion: "0.1.0",
			Message:       "Welcome to blegh",
		},
	})
	if err != nil {
		log.Printf("Failed to send welcome message: %v", err)
		return
	}

	// Read the message from the worker
	info, err := message.ReceiveType[message.WorkerInfo](mb, message.MTWorkerInfo, r.Context())
	if err != nil {
		log.Printf("Failed to read registration message: %v", err)
		return
	}
	if info.Type != message.MTWorkerInfo {
		log.Printf("Expected worker_info message, got %v", info.Type)
		return
	}

	// Register the worker
	worker := &Worker{
		Id:   uuid.New(),
		Info: info.Message,
		conn: conn,
		mbuf: mb,
	}
	hub.RegisterWorker(worker)

	// Send the registration response
	_, err = message.Send[message.Ack](mb, &message.TypedMessage[message.Ack]{
		Type:    message.MTAck,
		Id:      info.Id,
		Message: message.Ack{Ok: true, Message: worker.Id.String()},
	})
	if err != nil {
		http.Error(w, "Failed to send registration response", http.StatusInternalServerError)
		return
	}
}

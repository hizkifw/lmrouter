package agent

import (
	"context"
	"log"
	"net/http"

	"github.com/hizkifw/lmrouter/message"
)

func initWebsocket(done chan struct{}, mb *message.MessageBuffer, ctx context.Context) {
	defer close(done)

	// Wait for server identification
	serverInfo, err := message.ReceiveType[message.ServerInfo](mb, message.MTServerInfo, ctx)
	if err != nil {
		log.Printf("failed to read server info: %v", err)
		return
	}

	log.Printf("Connecting to server: %#v", serverInfo.Message)

	// Send worker info
	id, err := message.Send[message.WorkerInfo](mb, &message.TypedMessage[message.WorkerInfo]{
		Type:    message.MTWorkerInfo,
		Message: message.WorkerInfo{WorkerName: "worker1"},
	})
	if err != nil {
		log.Printf("failed to write worker info: %v", err)
		return
	}

	// Wait for ack
	ackMsg, err := message.ReceiveId[message.Ack](mb, id, ctx)
	if err != nil {
		log.Printf("failed to read ack: %v", err)
		return
	}
	if !ackMsg.Message.Ok {
		log.Printf("registration failed: %v", ackMsg.Message.Message)
		return
	}
	log.Printf("Registered worker: %v", ackMsg.Message.Message)

	// Ping message handler
	go func() {
		for {
			ping, err := message.ReceiveType[string](mb, message.MTPing, ctx)
			if err != nil {
				log.Printf("failed to read ping: %v", err)
				return
			}

			message.Send[string](mb, &message.TypedMessage[string]{
				Type:    message.MTAck,
				Id:      ping.Id,
				Message: "pong",
			})
		}
	}()

	// Wait for completions request
	client := &http.Client{}
	for {
		req, err := message.ReceiveType[message.CompletionsRequest](mb, message.MTCompletionsRequest, ctx)
		if err != nil {
			log.Printf("failed to read completions request: %v", err)
			return
		}
		log.Printf("Received completions request %s", req.Id)

		go func(req message.TypedMessage[message.CompletionsRequest]) {
			handleCompletions(&req, client, mb)
			log.Printf("Completed request %s", req.Id)
		}(*req)
	}

}

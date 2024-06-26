package agent

import (
	"context"
	"log"
	"net/http"

	"github.com/hizkifw/lmrouter/message"
)

func initWebsocket(opts *AgentOpts, done chan struct{}, mb *message.MessageBuffer, ctx context.Context) {
	defer close(done)

	// Query available models
	client := &http.Client{}
	models, err := queryModels(opts, client)
	if err != nil {
		log.Printf("failed to query models: %v", err)
		return
	}
	log.Printf("Available models: %v", models)

	// Wait for server identification
	serverInfo, err := message.ReceiveType[message.ServerInfo](mb, message.MTServerInfo, ctx)
	if err != nil {
		log.Printf("failed to read server info: %v", err)
		return
	}

	log.Printf("Registering to server: %#v", serverInfo.Message)

	// Send worker info
	id, err := message.Send[message.WorkerInfo](mb, &message.TypedMessage[message.WorkerInfo]{
		Type: message.MTWorkerInfo,
		Message: message.WorkerInfo{
			WorkerName:      opts.WorkerName,
			AvailableModels: models,
		},
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
	for {
		req, err := message.ReceiveType[message.CompletionsRequest](mb, message.MTCompletionsRequest, ctx)
		if err != nil {
			log.Printf("failed to read completions request: %v", err)
			return
		}
		log.Printf("Received completions request %s", req.Id)

		go func(req message.TypedMessage[message.CompletionsRequest]) {
			handleCompletions(opts, &req, client, mb)
			log.Printf("Completed request %s", req.Id)
		}(*req)
	}

}

package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hizkifw/lmrouter/message"
)

func RunAgent(hubAddr string) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: hubAddr, Path: "/internal/v1/worker/ws"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)

		// Wait for server identification
		serverInfo := message.TypedMessage[message.ServerInfo]{}
		if err := c.ReadJSON(&serverInfo); err != nil {
			log.Printf("failed to read server info: %v", err)
			return
		}
		if serverInfo.Type != message.MTServerInfo {
			log.Printf("expected server_info message, got %v", serverInfo.Type)
			return
		}

		log.Printf("Connecting to server: %#v", serverInfo.Message)

		// Send worker info
		workerInfo := message.TypedMessage[message.WorkerInfo]{
			Type:    message.MTWorkerInfo,
			Message: message.WorkerInfo{WorkerName: "worker1"},
		}
		if err := c.WriteJSON(workerInfo); err != nil {
			log.Printf("failed to write worker info: %v", err)
			return
		}

		// Wait for ack
		ack := message.TypedMessage[message.Ack]{}
		if err := c.ReadJSON(&ack); err != nil {
			log.Printf("failed to read ack: %v", err)
			return
		}
		if ack.Type != message.MTAck {
			log.Printf("expected ack message, got %v", ack.Type)
			return
		}
		if !ack.Message.Ok {
			log.Printf("registration failed: %v", ack.Message.Message)
			return
		}

		log.Printf("Registered worker: %v", ack.Message.Message)

		// Wait for completions request
		for {
			req := message.TypedMessage[message.CompletionsRequest]{}
			if err := c.ReadJSON(&req); err != nil {
				log.Printf("failed to read completions request: %v", err)
				return
			}
			if req.Type != message.MTCompletionsRequest {
				log.Printf("expected completions_request message, got %v", req.Type)
				return
			}
			log.Printf("Received completions request: %#v", req.Message)

			// Perform completion
			// POST to http://localhost:5000/v1/completions
			// with the completions request

			// Create a new HTTP client
			client := &http.Client{}

			// Marshal the request into JSON
			reqBody, err := json.Marshal(req.Message)
			if err != nil {
				log.Printf("failed to marshal request: %v", err)
				return
			}

			// Create a new HTTP request
			httpReq, err := http.NewRequest("POST", "http://127.0.0.1:5000/v1/completions", bytes.NewBuffer(reqBody))
			if err != nil {
				log.Printf("failed to create request: %v", err)
				return
			}

			// Set the Content-Type header
			httpReq.Header.Set("Content-Type", "application/json")

			// Send the HTTP request
			resp, err := client.Do(httpReq)
			if err != nil {
				log.Printf("failed to send request: %v", err)
				return
			}
			defer resp.Body.Close()

			// Check the HTTP response status
			if resp.StatusCode != http.StatusOK {
				log.Printf("unexpected response status: %v", resp.Status)
				return
			}

			// Scan the response body
			if req.Message.Stream {
				reader := bufio.NewReader(resp.Body)
				for {
					line, err := reader.ReadBytes('\n')
					if err != nil {
						if err != io.EOF {
							log.Printf("failed to read response body: %v", err)
						}
						break
					}

					if bytes.HasPrefix(line, []byte("data: ")) {
						line = line[6:]
					}

					// Unmarshal the response body into a CompletionsResponse
					compResp := message.TypedMessage[message.CompletionsResponse]{
						Type: message.MTCompletionsResponse,
					}
					json.Unmarshal([]byte(line), &compResp.Message)

					// Send the completions response back to the server
					c.WriteJSON(compResp)
				}
			} else {
				// Unmarshal the response body into a CompletionsResponse
				compResp := message.TypedMessage[message.CompletionsResponse]{
					Type: message.MTCompletionsResponse,
				}
				json.NewDecoder(resp.Body).Decode(&compResp.Message)

				// Send the completions response back to the server
				c.WriteJSON(compResp)
			}

			log.Printf("Completed request: %#v", req.Message)
		}
	}()

	for {
		select {
		case <-done:
			return nil
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return err
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return nil
		}
	}
}

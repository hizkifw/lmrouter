package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/hizkifw/lmrouter/message"
)

func handleCompletions(req *message.TypedMessage[message.CompletionsRequest], client *http.Client, mb *message.MessageBuffer) {
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

	// Handle non-streaming response
	if !req.Message.Stream {
		// Unmarshal the response body into a CompletionsResponse
		compResp := message.TypedMessage[message.CompletionsResponse]{
			Type: message.MTCompletionsResponse,
			Id:   req.Id,
		}
		json.NewDecoder(resp.Body).Decode(&compResp.Message)

		// Send the completions response back to the server
		if _, err := message.Send[message.CompletionsResponse](mb, &compResp); err != nil {
			log.Printf("failed to send completions response: %v", err)
		}
		return
	}

	// Scan the response body
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
		} else {
			continue
		}

		// Unmarshal the response body into a CompletionsResponse
		compResp := message.TypedMessage[message.CompletionsResponse]{
			Type: message.MTCompletionsResponse,
			Id:   req.Id,
		}
		json.Unmarshal([]byte(line), &compResp.Message)

		// Send the completions response back to the server
		if _, err := message.Send[message.CompletionsResponse](mb, &compResp); err != nil {
			log.Printf("failed to send completions response: %v", err)
		}
	}
}
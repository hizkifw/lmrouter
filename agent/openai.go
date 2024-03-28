package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/hizkifw/lmrouter/message"
)

func queryModels(opts *AgentOpts, client *http.Client) ([]message.Model, error) {
	endpoint := opts.InferenceAddr.JoinPath("/v1/models").String()
	httpReq, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if opts.InferenceAuthorization != "" {
		httpReq.Header.Set("Authorization", opts.InferenceAuthorization)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status: %s", resp.Status)
	}

	var models message.ListModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return models.Data, nil
}

func handleCompletions(
	opts *AgentOpts, req *message.TypedMessage[message.CompletionsRequest],
	client *http.Client, mb *message.MessageBuffer,
) {
	// Marshal the request into JSON
	reqBody, err := json.Marshal(req.Message)
	if err != nil {
		log.Printf("failed to marshal request: %v", err)
		return
	}

	// Create a new HTTP request
	endpoint := opts.InferenceAddr.JoinPath("/v1/completions").String()
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("failed to create request: %v", err)
		return
	}

	if opts.InferenceAuthorization != "" {
		httpReq.Header.Set("Authorization", opts.InferenceAuthorization)
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
		if bytes.HasSuffix(line, []byte("\n")) {
			line = line[:len(line)-1]
		}

		// Skip [DONE] message from OpenAI
		if bytes.Equal(line, []byte("[DONE]")) {
			break
		}

		// Construct a CompletionsResponse
		compResp := message.TypedMessage[json.RawMessage]{
			Type:    message.MTCompletionsResponse,
			Id:      req.Id,
			Message: line,
		}

		// Send the completions response back to the server
		if _, err := message.Send[json.RawMessage](mb, &compResp); err != nil {
			log.Printf("failed to send completions response: %v", err)
		}
	}

	if _, err := message.Send[string](mb, &message.TypedMessage[string]{
		Type:    message.MTCompletionsDone,
		Id:      req.Id,
		Message: "done",
	}); err != nil {
		log.Printf("failed to send completions done message: %v", err)
	}
}

package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hizkifw/lmrouter/agent"
	"github.com/hizkifw/lmrouter/hub"
	"github.com/hizkifw/lmrouter/message"
	"github.com/stretchr/testify/assert"
)

func dummyInferenceServer(addr string, ctx context.Context) {
	mux := http.NewServeMux()

	// Handle the list models endpoint
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		resp := message.ListModelsResponse{
			Object: "list",
			Data: []message.Model{
				{
					Id:      "gpt-2",
					Object:  "model",
					Created: int(time.Now().UnixMilli()),
					OwnedBy: "openai",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Handle the completions endpoint
	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		req := message.CompletionsRequest{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Failed to parse request", http.StatusBadRequest)
			return
		}

		finishReason := "length"

		if !req.Stream {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(
				message.CompletionsResponse{
					ID:      "cmpl-0000",
					Object:  "text_completion",
					Created: (time.Now().UnixMilli()),
					Choices: []message.CompletionsChoice{
						{
							Text:         "Hello, world!",
							Index:        0,
							FinishReason: &finishReason,
						},
					},
					Model: "gpt-2",
				})
			return
		}

		// Streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		tokens := strings.Split("lmrouter is a language model router", " ")
		encoder := json.NewEncoder(w)
		for _, token := range tokens {
			w.Write([]byte("data: "))
			encoder.Encode(message.CompletionsResponse{
				ID:      "cmpl-0000",
				Object:  "text_completion",
				Created: (time.Now().UnixMilli()),
				Choices: []message.CompletionsChoice{
					{
						Text:         token,
						Index:        0,
						FinishReason: nil,
					},
				},
			})
			w.Write([]byte("\n")) // json encoder already adds a newline
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	go server.ListenAndServe()

	<-ctx.Done()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server.Shutdown(ctxTimeout)
}

func TestE2E(t *testing.T) {
	assert := assert.New(t)
	wg := &sync.WaitGroup{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hubListen := "127.22.33.45:9090"
	inferenceListen := "127.22.33.45:5000"
	hubUrl := url.URL{Scheme: "http", Host: hubListen}

	// Start the inference server
	wg.Add(1)
	go func() {
		defer wg.Done()
		dummyInferenceServer(inferenceListen, ctx)
	}()

	// Start the server
	wg.Add(1)
	go func() {
		defer wg.Done()
		hub.RunServer(&hub.ServerOpts{Addr: hubListen}, ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// Make sure there are no agents connected yet
	resp, err := http.Get(hubUrl.JoinPath("/internal/v1/workers").String())
	assert.NoError(err)
	var workers []hub.Worker
	assert.NoError(json.NewDecoder(resp.Body).Decode(&workers))
	assert.Len(workers, 0)

	// Start an agent
	ctxAgent, cancelAgent := context.WithCancel(ctx)
	defer cancelAgent()
	wgAgent := &sync.WaitGroup{}
	wgAgent.Add(1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer wgAgent.Done()
		agent.RunAgent(&agent.AgentOpts{
			HubAddr:       url.URL{Scheme: "ws", Host: hubListen},
			InferenceAddr: url.URL{Scheme: "http", Host: inferenceListen},
			WorkerName:    "test-worker",
		}, ctxAgent)
	}()

	// Wait for the agent to connect
	time.Sleep(100 * time.Millisecond)

	// Test that the agent connected
	resp, err = http.Get(hubUrl.JoinPath("/internal/v1/workers").String())
	assert.NoError(err)
	assert.NoError(json.NewDecoder(resp.Body).Decode(&workers))
	assert.Len(workers, 1)
	assert.Equal("test-worker", workers[0].Info.WorkerName)
	assert.Len(workers[0].Info.AvailableModels, 1)

	// Test the models endpoint
	resp, err = http.Get(hubUrl.JoinPath("/v1/models").String())
	assert.NoError(err)
	var models message.ListModelsResponse
	assert.NoError(json.NewDecoder(resp.Body).Decode(&models))
	assert.Len(models.Data, 1)
	assert.Equal("gpt-2", models.Data[0].Id)

	// Test the completions endpoint
	req := message.CompletionsRequest{Model: "gpt-2", Prompt: "Hello,"}
	enc, err := json.Marshal(req)
	assert.NoError(err)
	resp, err = http.Post(hubUrl.JoinPath("/v1/completions").String(), "application/json", bytes.NewReader(enc))
	assert.NoError(err)
	var compResp message.CompletionsResponse
	assert.NoError(json.NewDecoder(resp.Body).Decode(&compResp))
	assert.Equal("Hello, world!", compResp.Choices[0].Text)

	// Endpoint should fail if model is not available
	req = message.CompletionsRequest{Model: "unknown-model", Prompt: "Hello,"}
	enc, err = json.Marshal(req)
	assert.NoError(err)
	resp, err = http.Post(hubUrl.JoinPath("/v1/completions").String(), "application/json", bytes.NewReader(enc))
	assert.NoError(err)
	assert.Equal(http.StatusServiceUnavailable, resp.StatusCode)

	// Streaming response should work
	req = message.CompletionsRequest{Model: "gpt-2", Prompt: "lmrouter is", Stream: true}
	enc, err = json.Marshal(req)
	assert.NoError(err)
	resp, err = http.Post(hubUrl.JoinPath("/v1/completions").String(), "application/json", bytes.NewReader(enc))
	assert.NoError(err)
	assert.Equal(http.StatusOK, resp.StatusCode)
	reader := bufio.NewReader(resp.Body)
	parts := 0
	lastTime := time.Now()
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		if len(line) < 6 {
			continue
		}
		parts++
		assert.True(strings.HasPrefix(line, "data: "))
		resp := message.CompletionsResponse{}
		assert.NoError(json.Unmarshal([]byte(line[6:]), &resp))
		createdTime := time.Unix(0, int64(resp.Created)*int64(time.Millisecond))
		assert.WithinDuration(time.Now(), createdTime, 10*time.Millisecond)
		if parts > 1 {
			assert.WithinDuration(createdTime, lastTime.Add(10*time.Millisecond), 5*time.Millisecond)
		}
		lastTime = createdTime
	}
	assert.Equal(6, parts)

	// Close the agent and test that it disconnected
	cancelAgent()
	wgAgent.Wait()
	resp, err = http.Get(hubUrl.JoinPath("/internal/v1/workers").String())
	assert.NoError(err)
	assert.NoError(json.NewDecoder(resp.Body).Decode(&workers))
	assert.Len(workers, 0)

	// List of models should be empty as well
	resp, err = http.Get(hubUrl.JoinPath("/v1/models").String())
	assert.NoError(err)
	assert.NoError(json.NewDecoder(resp.Body).Decode(&models))
	assert.Len(models.Data, 0)

	// Cancel the context and wait for everything to shut down
	cancel()
	wg.Wait()
}

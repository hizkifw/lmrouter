package agent

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hizkifw/lmrouter/message"
)

type AgentOpts struct {
	// HubAddr is the address of the hub server
	HubAddr url.URL `arg:"--hub,required" help:"address of the hub server (e.g. ws://localhost:9090)"`

	// InferenceAddr is the address of the inference server
	InferenceAddr url.URL `arg:"--inference" help:"address of the OpenAI-compatible inference server" default:"http://localhost:5000"`

	// InferenceAuthorization is the value used for the Authorization header
	// when querying the inference server
	InferenceAuthorization string `arg:"--inference-authorization,env:INFERENCE_AUTHORIZATION" help:"value for the Authorization header when querying the inference server (e.g. Bearer abc)"`

	// WorkerName is the name of the worker
	WorkerName string `arg:"--name" help:"name of the worker" default:"worker"`
}

func RunAgent(opts *AgentOpts) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	log.Printf("Connecting to %s", opts.HubAddr.String())

	fullAddr := opts.HubAddr.JoinPath("/internal/v1/worker/ws")
	conn, _, err := websocket.DefaultDialer.Dial(fullAddr.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()

	done := make(chan struct{})
	conn.SetCloseHandler(func(code int, text string) error {
		close(done)
		return nil
	})

	mb := message.NewMessageBuffer(conn)
	go mb.RecvLoop()
	go initWebsocket(opts, done, mb, context.Background())

	for {
		select {
		case <-done:
			return nil
		case <-interrupt:
			log.Println("interrupt")

			// Close the connection
			err := mb.Close()
			if err != nil {
				log.Println("write close:", err)
				return err
			}

			// Wait for the connection to close
			select {
			case <-done:
			case <-time.After(time.Second):
			}

			return nil
		}
	}
}

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

func RunAgent(hubAddr string) error {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: hubAddr, Path: "/internal/v1/worker/ws"}
	log.Printf("connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
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
	go initWebsocket(done, mb, context.Background())

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

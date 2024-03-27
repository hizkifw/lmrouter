package main

import (
	"os"

	"github.com/hizkifw/lmrouter/agent"
	"github.com/hizkifw/lmrouter/hub"
)

func main() {
	if os.Args[1] == "server" {
		if err := hub.RunServer(":9090"); err != nil {
			panic(err)
		}
	} else if os.Args[1] == "agent" {
		if err := agent.RunAgent("localhost:9090"); err != nil {
			panic(err)
		}
	}
}

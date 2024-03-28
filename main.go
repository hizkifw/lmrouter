package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alexflint/go-arg"
	"github.com/hizkifw/lmrouter/agent"
	"github.com/hizkifw/lmrouter/hub"
)

func mustParseArgs(dest ...interface{}) {
	parser, err := arg.NewParser(arg.Config{}, dest...)
	if err != nil {
		log.Fatalf("failed to create parser: %v", err)
	}

	err = parser.Parse(os.Args[2:])
	switch {
	case err == nil:
		return
	case err == arg.ErrHelp:
		parser.WriteHelp(os.Stderr)
		os.Exit(0)
	case err == arg.ErrVersion:
		fmt.Println("1.0.0")
		os.Exit(0)
	default:
		parser.WriteHelp(os.Stderr)
		fmt.Println("")
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <server|agent> [args]\n", os.Args[0])
		os.Exit(1)
	}

	subcommand := os.Args[1]
	if subcommand == "server" {
		opts := hub.ServerOpts{}
		mustParseArgs(&opts)

		if err := hub.RunServer(&opts); err != nil {
			panic(err)
		}
	} else if subcommand == "agent" {
		opts := agent.AgentOpts{}
		mustParseArgs(&opts)

		if err := agent.RunAgent(&opts); err != nil {
			panic(err)
		}
	}
}

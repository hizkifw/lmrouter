# lmrouter

Just like [AI Horde](https://stablehorde.net/) but specifically for low-latency
streaming text generation.

Currently work in progress.

## Usage

```sh
# Build the project
go build .

# Run the server
./lmrouter server --listen :9090

# Run the agent
./lmrouter agent --hub ws://localhost:9090 --inference http://localhost:5000
```

package message

type MessageType string

const (
	MTServerInfo          MessageType = "server_info"
	MTWorkerInfo          MessageType = "worker_info"
	MTAck                 MessageType = "ack"
	MTCompletionsRequest  MessageType = "completions_request"
	MTCompletionsResponse MessageType = "completions_response"
)

type TypedMessage[T any] struct {
	Type    MessageType `json:"type"`
	Message T           `json:"message"`
}

type Ack struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

type ServerInfo struct {
	ServerName    string `json:"server_name"`
	ServerVersion string `json:"server_version"`
	Message       string `json:"message"`
}

type WorkerInfo struct {
	WorkerName string `json:"worker_name"`
}

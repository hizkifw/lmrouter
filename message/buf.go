package message

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func NewId() string {
	return uuid.New().String()
}

type MessageBuffer struct {
	conn       *websocket.Conn
	recvBuffer map[string]*TypedMessage[json.RawMessage]
	connLock   sync.Mutex
	bufferLock sync.Mutex
}

func NewMessageBuffer(conn *websocket.Conn) *MessageBuffer {
	return &MessageBuffer{
		conn:       conn,
		recvBuffer: make(map[string]*TypedMessage[json.RawMessage]),
	}
}

func Send[T any](mb *MessageBuffer, msg *TypedMessage[T]) (string, error) {
	if msg.Id == "" {
		msg.Id = NewId()
	}

	mb.connLock.Lock()
	defer mb.connLock.Unlock()

	if err := mb.conn.WriteJSON(msg); err != nil {
		return "", err
	}
	return msg.Id, nil
}

func castMessage[T any](msg *TypedMessage[json.RawMessage]) *TypedMessage[T] {
	m := &TypedMessage[T]{
		Type: msg.Type,
		Id:   msg.Id,
	}
	json.Unmarshal(msg.Message, &m.Message)
	return m
}

func Receive[T any](mb *MessageBuffer) (*TypedMessage[T], error) {
	var msg TypedMessage[T]

	mb.connLock.Lock()
	defer mb.connLock.Unlock()

	if err := mb.conn.ReadJSON(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func ReceiveId[T any](mb *MessageBuffer, id string) (*TypedMessage[T], error) {
	for {
		mb.bufferLock.Lock()
		if msg, ok := mb.recvBuffer[id]; ok {
			delete(mb.recvBuffer, id)
			mb.bufferLock.Unlock()
			return castMessage[T](msg), nil
		}
		mb.bufferLock.Unlock()

		msg, err := Receive[json.RawMessage](mb)
		if err != nil {
			return nil, err
		}

		if msg.Id == id {
			return castMessage[T](msg), nil
		}

		mb.bufferLock.Lock()
		mb.recvBuffer[msg.Id] = msg
		mb.bufferLock.Unlock()
	}
}

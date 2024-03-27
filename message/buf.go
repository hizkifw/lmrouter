package message

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func NewId() string {
	return uuid.New().String()
}

type MessageBuffer struct {
	conn       *websocket.Conn
	recvBuffer map[string]*TypedMessage[json.RawMessage]
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

func ReceiveId[T any](mb *MessageBuffer, id string) (*TypedMessage[T], error) {
	for {
		if msg, ok := mb.recvBuffer[id]; ok {
			delete(mb.recvBuffer, id)
			return castMessage[T](msg), nil
		}

		var msg TypedMessage[json.RawMessage]
		if err := mb.conn.ReadJSON(&msg); err != nil {
			return nil, err
		}

		if msg.Id == id {
			return castMessage[T](&msg), nil
		}

		mb.recvBuffer[msg.Id] = &msg
	}
}

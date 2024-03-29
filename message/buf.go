package message

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func NewId() string {
	return uuid.New().String()
}

type MessageBuffer struct {
	conn       *websocket.Conn
	recvBuffer map[string]*TypedMessage[json.RawMessage]
	recvLock   sync.Mutex
	sendLock   sync.Mutex
	bufferLock sync.Mutex
}

func NewMessageBuffer(conn *websocket.Conn) *MessageBuffer {
	return &MessageBuffer{
		conn:       conn,
		recvBuffer: make(map[string]*TypedMessage[json.RawMessage]),
	}
}

func (mb *MessageBuffer) RecvLoop() {
	for {
		msg, err := receive[json.RawMessage](mb)
		if err != nil {
			log.Printf("failed to receive message: %v", err)
			return
		}

		for !mb.tryInsert(msg) {
			time.Sleep(time.Millisecond)
		}
	}
}

func (mb *MessageBuffer) tryInsert(msg *TypedMessage[json.RawMessage]) bool {
	mb.bufferLock.Lock()
	defer mb.bufferLock.Unlock()
	if _, ok := mb.recvBuffer[msg.Id]; ok {
		return false
	}
	mb.recvBuffer[msg.Id] = msg
	return true
}

func (mb *MessageBuffer) Close() error {
	err := mb.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)

	mb.bufferLock.Lock()
	mb.recvBuffer = nil

	return err
}

func Send[T any](mb *MessageBuffer, msg *TypedMessage[T]) (string, error) {
	if msg.Id == "" {
		msg.Id = NewId()
	}

	mb.sendLock.Lock()
	defer mb.sendLock.Unlock()

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

func receive[T any](mb *MessageBuffer) (*TypedMessage[T], error) {
	var msg TypedMessage[T]

	mb.recvLock.Lock()
	defer mb.recvLock.Unlock()

	if err := mb.conn.ReadJSON(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func ReceiveType[T any](mb *MessageBuffer, typ MessageType, ctx context.Context) (*TypedMessage[T], error) {
	for {
		mb.bufferLock.Lock()
		for id, msg := range mb.recvBuffer {
			if msg.Type == typ {
				delete(mb.recvBuffer, id)
				mb.bufferLock.Unlock()
				return castMessage[T](msg), nil
			}
		}
		mb.bufferLock.Unlock()

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		time.Sleep(time.Millisecond)
	}
}

func ReceiveId[T any](mb *MessageBuffer, id string, ctx context.Context) (*TypedMessage[T], error) {
	for {
		mb.bufferLock.Lock()
		if msg, ok := mb.recvBuffer[id]; ok {
			delete(mb.recvBuffer, id)
			mb.bufferLock.Unlock()
			return castMessage[T](msg), nil
		}
		mb.bufferLock.Unlock()

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		time.Sleep(time.Millisecond)
	}
}

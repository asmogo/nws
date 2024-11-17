package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type MessageType string

const (
	MessageTypeSocks5     = MessageType("SOCKS5RESPONSE")
	MessageConnect        = MessageType("CONNECT")
	MessageConnectReverse = MessageType("CONNECTR")
)

type Message struct {
	Key                uuid.UUID   `json:"key,omitempty"`                // unique identifier for the message
	Type               MessageType `json:"type,omitempty"`               // type of message
	Data               []byte      `json:"data,omitempty"`               // data to be sent
	Destination        string      `json:"destination,omitempty"`        // destination to send the message
	EntryPublicAddress string      `json:"entryPublicAddress,omitempty"` // public ip address of the entry node (used for reverse connect)
}

type MessageOption func(*Message)

func WithUUID(uuid uuid.UUID) MessageOption {
	return func(m *Message) {
		m.Key = uuid
	}
}

func WithType(messageType MessageType) MessageOption {
	return func(m *Message) {
		m.Type = messageType
	}
}

func WithDestination(destination string) MessageOption {
	return func(m *Message) {
		m.Destination = destination
	}
}

func WithEntryPublicAddress(entryPublicAddress string) MessageOption {
	return func(m *Message) {
		m.EntryPublicAddress = entryPublicAddress
	}
}
func WithData(data []byte) MessageOption {
	return func(m *Message) {
		m.Data = data
	}
}

func NewMessage(configs ...MessageOption) *Message {
	m := &Message{}
	for _, config := range configs {
		config(m)
	}
	return m
}
func MarshalJSON(m *Message) ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("could not marshal message: %w", err)
	}
	return data, nil
}

func UnmarshalJSON(data []byte) (*Message, error) {
	m := NewMessage()
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("could not unmarshal message: %w", err)
	}
	return m, nil
}

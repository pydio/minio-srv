package websocket

import "encoding/json"

type MessageType string

const (
	MsgSubscribe   MessageType = "subscribe"
	MsgUnsubscribe MessageType = "unsubscribe"
	MsgError       MessageType = "error"
)

// Should pass JWT instead of username
type Message struct {
	Type  MessageType `json:"@type"`
	JWT   string      `json:"jwt"`
	Error string      `json:"error"`
}

func NewErrorMessage(e error) []byte {
	return Marshal(Message{
		Type:  MsgError,
		Error: e.Error(),
	})
}

func NewErrorMessageString(e string) []byte {
	return Marshal(Message{
		Type:  MsgError,
		Error: e,
	})
}

func Marshal(m Message) []byte {
	data, _ := json.Marshal(m)
	return data
}

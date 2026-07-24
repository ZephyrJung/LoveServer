package hub

import "encoding/json"

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
	ID   string          `json:"id,omitempty"`
}

type Response struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	ID   string      `json:"id,omitempty"`
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
}

func NewResponse(msgType string, data interface{}, id string) *Response {
	return &Response{
		Type: msgType,
		Data: data,
		ID:   id,
		Code: 0,
		Msg:  "ok",
	}
}

func NewError(msgType string, code int, msg string, id string) *Response {
	return &Response{
		Type: msgType,
		Data: nil,
		ID:   id,
		Code: code,
		Msg:  msg,
	}
}

func NewEvent(eventType string, data interface{}) *Response {
	return &Response{
		Type: eventType,
		Data: data,
		Code: 0,
		Msg:  "ok",
	}
}

const (
	ErrSuccess            = 0
	ErrUnauthenticated    = 1001
	ErrAuthFailed         = 1002
	ErrInvalidMessage     = 1003
	ErrRoomNotFound       = 2001
	ErrRoomFull           = 2002
	ErrNotInRoom          = 2003
	ErrNotRoomOwner       = 2004
	ErrGameNotFound       = 3001
	ErrGameAlreadyStarted = 3002
	ErrMatchQueueFull     = 4001
	ErrInternal           = 5001
)

var errorMessages = map[int]string{
	ErrUnauthenticated:    "not authenticated",
	ErrAuthFailed:         "authentication failed",
	ErrInvalidMessage:     "invalid message format",
	ErrRoomNotFound:       "room not found",
	ErrRoomFull:           "room is full",
	ErrNotInRoom:          "not in a room",
	ErrNotRoomOwner:       "not the room owner",
	ErrGameNotFound:       "game type not found",
	ErrGameAlreadyStarted: "game already started",
	ErrMatchQueueFull:     "match queue is full",
	ErrInternal:           "internal server error",
}

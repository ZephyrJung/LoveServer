package game

import (
	"encoding/json"
)

type GameResult struct {
	GameName string
	RoomID   string
	WinnerID string
	RawData  json.RawMessage
}

type Game interface {
	Name() string
	DisplayName() string
	MinPlayers() int
	MaxPlayers() int

	OnInit(room interface{}, settings map[string]any) error
	OnPlayerJoin(room interface{}, player interface{}) error
	OnPlayerLeave(room interface{}, player interface{}) error
	OnStart(room interface{}) error
	OnMessage(room interface{}, player interface{}, data json.RawMessage) (any, error)
	OnTick(room interface{}, dt float64)
	OnEnd(room interface{}) (*GameResult, error)

	MarshalState() ([]byte, error)
	UnmarshalState(data []byte) error
}

package lobby

import "errors"

var (
	ErrRoomNotFound       = errors.New("room not found")
	ErrNotInRoom          = errors.New("not in this room")
	ErrAlreadyInRoom      = errors.New("already in a room")
	ErrGameNotFound       = errors.New("game type not found")
	ErrNotRoomOwner       = errors.New("not the room owner")
	ErrGameAlreadyStarted = errors.New("game already started")
)
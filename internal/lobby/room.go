package lobby

import (
	"sync"
	"time"

	"github.com/ZephyrJung/LoveServer/internal/game"
)

type RoomState int

const (
	RoomWaiting RoomState = iota
	RoomPlaying
	RoomEnded
	RoomClosed
)

type Player struct {
	ID       string
	Nickname string
	IsReady  bool
	JoinedAt time.Time
	Score    int
}

type Room struct {
	ID           string
	Name         string
	GameName     string
	State        RoomState
	OwnerID      string
	Players      map[string]*Player
	MaxPlayers   int
	MinPlayers   int
	Settings     map[string]any
	CreatedAt    time.Time
	GameInstance game.Game
	mu           sync.RWMutex
}

var ErrRoomFull = &roomFullError{}

type roomFullError struct{}

func (e *roomFullError) Error() string {
	return "room is full"
}

func NewRoom(id, name, gameName, ownerID string, minPlayers, maxPlayers int) *Room {
	return &Room{
		ID:         id,
		Name:       name,
		GameName:   gameName,
		State:      RoomWaiting,
		OwnerID:    ownerID,
		Players:    make(map[string]*Player),
		MaxPlayers: maxPlayers,
		MinPlayers: minPlayers,
		Settings:   make(map[string]any),
		CreatedAt:  time.Now(),
	}
}

func (r *Room) AddPlayer(playerID, nickname string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.Players) >= r.MaxPlayers {
		return ErrRoomFull
	}
	if _, ok := r.Players[playerID]; ok {
		return nil // already in room
	}
	r.Players[playerID] = &Player{
		ID:       playerID,
		Nickname: nickname,
		JoinedAt: time.Now(),
	}
	return nil
}

func (r *Room) RemovePlayer(playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Players, playerID)
	if len(r.Players) == 0 {
		r.State = RoomClosed
	}
}

func (r *Room) PlayerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Players)
}

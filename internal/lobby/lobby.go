package lobby

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/ZephyrJung/LoveServer/internal/game"
)

type Lobby struct {
	rooms        map[string]*Room
	gameRegistry *game.Registry
	gameManager  *game.InstanceManager
	matchQueue   *MatchQueue
	mu           sync.RWMutex
}

func NewLobby(registry *game.Registry, gameManager *game.InstanceManager) *Lobby {
	l := &Lobby{
		rooms:        make(map[string]*Room),
		gameRegistry: registry,
		gameManager:  gameManager,
	}
	l.matchQueue = NewMatchQueue(l, registry)
	return l
}

func (l *Lobby) CreateRoom(name, gameName, ownerID string, settings map[string]any) (*Room, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if game exists
	gameFactory, ok := l.gameRegistry.Get(gameName)
	if !ok {
		return nil, ErrGameNotFound
	}

	// Get game info from a temporary instance
	g := gameFactory()
	if g == nil {
		return nil, ErrGameNotFound
	}

	roomID := uuid.New().String()
	room := NewRoom(roomID, name, gameName, ownerID, g.MinPlayers(), g.MaxPlayers())
	room.Settings = settings

	// Create game instance
	gameInst, err := l.gameManager.CreateInstance(roomID, gameName, settings)
	if err != nil {
		return nil, err
	}
	room.GameInstance = gameInst

	// Add owner as first player
	room.AddPlayer(ownerID, ownerID) // nickname will be set from session

	l.rooms[roomID] = room
	return room, nil
}

func (l *Lobby) GetRoom(roomID string) (*Room, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	room, ok := l.rooms[roomID]
	return room, ok
}

func (l *Lobby) ListRooms(gameName string) []*Room {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var result []*Room
	for _, room := range l.rooms {
		if room.State == RoomWaiting {
			if gameName == "" || room.GameName == gameName {
				result = append(result, room)
			}
		}
	}
	return result
}

func (l *Lobby) RemoveRoom(roomID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gameManager.Remove(roomID)
	delete(l.rooms, roomID)
}

func (l *Lobby) StartMatchLoop(ctx context.Context) {
	l.matchQueue.Start(ctx)
}

func (l *Lobby) MatchQueue() *MatchQueue {
	return l.matchQueue
}
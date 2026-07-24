package lobby

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ZephyrJung/LoveServer/internal/game"
)

type MatchEntry struct {
	PlayerID string
	Nickname string
	JoinedAt time.Time
}

type MatchQueue struct {
	queues   map[string][]*MatchEntry // gameName -> queue
	mu       sync.Mutex
	lobby    *Lobby
	registry *game.Registry
	OnMatch  func(roomID string, playerIDs []string) // callback for match notification
}

func NewMatchQueue(lobby *Lobby, registry *game.Registry) *MatchQueue {
	return &MatchQueue{
		queues:   make(map[string][]*MatchEntry),
		lobby:    lobby,
		registry: registry,
	}
}

func (q *MatchQueue) Enqueue(gameName, playerID, nickname string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queues[gameName] = append(q.queues[gameName], &MatchEntry{
		PlayerID: playerID,
		Nickname: nickname,
		JoinedAt: time.Now(),
	})
	return nil
}

func (q *MatchQueue) Dequeue(gameName, playerID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	queue := q.queues[gameName]
	for i, entry := range queue {
		if entry.PlayerID == playerID {
			q.queues[gameName] = append(queue[:i], queue[i+1:]...)
			return
		}
	}
}

func (q *MatchQueue) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				q.tryMatch()
			}
		}
	}()
}

func (q *MatchQueue) tryMatch() {
	q.mu.Lock()
	defer q.mu.Unlock()

	for gameName, queue := range q.queues {
		factory, ok := q.registry.Get(gameName)
		if !ok {
			continue
		}
		g := factory()
		minPlayers := g.MinPlayers()
		maxPlayers := g.MaxPlayers()

		for len(queue) >= minPlayers {
			matchCount := minPlayers
			if maxPlayers > 0 && len(queue) >= maxPlayers {
				matchCount = maxPlayers
			}

			matched := queue[:matchCount]
			queue = queue[matchCount:]

			playerIDs := make([]string, len(matched))
			for i, e := range matched {
				playerIDs[i] = e.PlayerID
			}
			log.Printf("match found for %s: %v", gameName, playerIDs)

			// Create room for matched players
			room, err := q.lobby.CreateRoom(
				gameName+" match",
				gameName,
				matched[0].PlayerID,
				nil,
			)
			if err != nil {
				log.Printf("failed to create match room: %v", err)
				continue
			}

			// Add all matched players to the room
			for _, entry := range matched {
				room.AddPlayer(entry.PlayerID, entry.Nickname)
			}

			// Notify matched players via callback
			if q.OnMatch != nil {
				q.OnMatch(room.ID, playerIDs)
			}
		}
		q.queues[gameName] = queue
	}
}

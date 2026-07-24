package session

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Session struct {
	ID        string
	Conn      *websocket.Conn
	PlayerID  string
	Nickname  string
	RoomID    string
	Authed    bool
	CreatedAt time.Time
	mu        sync.Mutex
}

func NewSession(id string, conn *websocket.Conn) *Session {
	return &Session{
		ID:        id,
		Conn:      conn,
		CreatedAt: time.Now(),
		mu:        sync.Mutex{},
	}
}

func (s *Session) SendJSON(v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Conn.WriteJSON(v)
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Conn.Close()
}
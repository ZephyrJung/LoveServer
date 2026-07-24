package session

import (
	"encoding/json"
	"sync"
)

type Manager struct {
	byID     map[string]*Session // sessionID -> Session
	byPlayer map[string]*Session // playerID -> Session
	mu       sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		byID:     make(map[string]*Session),
		byPlayer: make(map[string]*Session),
	}
}

func (m *Manager) Add(s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[s.ID] = s
}

func (m *Manager) Get(sessionID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byID[sessionID]
}

func (m *Manager) GetByPlayer(playerID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byPlayer[playerID]
}

func (m *Manager) Remove(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byID[sessionID]
	if s != nil {
		delete(m.byPlayer, s.PlayerID)
	}
	delete(m.byID, sessionID)
}

func (m *Manager) BindPlayer(sessionID, playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byID[sessionID]
	if s != nil {
		s.PlayerID = playerID
		m.byPlayer[playerID] = s
	}
}

func (m *Manager) Broadcast(roomID string, msg []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.byID {
		if s.RoomID == roomID {
			s.SendJSON(json.RawMessage(msg))
		}
	}
}

func (m *Manager) SendToPlayer(playerID string, v interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.byPlayer[playerID]; ok {
		s.SendJSON(v)
	}
}
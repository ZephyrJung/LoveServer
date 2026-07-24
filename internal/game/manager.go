package game

import "sync"

type InstanceManager struct {
	instances map[string]Game // roomID -> Game instance
	mu        sync.RWMutex
	registry  *Registry
}

func NewInstanceManager(registry *Registry) *InstanceManager {
	return &InstanceManager{
		instances: make(map[string]Game),
		registry:  registry,
	}
}

func (m *InstanceManager) CreateInstance(roomID, gameName string, settings map[string]any) (Game, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, err := m.registry.Create(gameName)
	if err != nil {
		return nil, err
	}
	m.instances[roomID] = g
	return g, nil
}

func (m *InstanceManager) Get(roomID string) Game {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[roomID]
}

func (m *InstanceManager) Remove(roomID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.instances, roomID)
}
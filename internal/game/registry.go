package game

import "fmt"

type GameFactory func() Game

type Registry struct {
	games map[string]GameFactory
}

func NewRegistry() *Registry {
	return &Registry{
		games: make(map[string]GameFactory),
	}
}

func (r *Registry) Register(name string, factory GameFactory) {
	r.games[name] = factory
}

func (r *Registry) Create(name string) (Game, error) {
	factory, ok := r.games[name]
	if !ok {
		return nil, fmt.Errorf("game type %q not registered", name)
	}
	return factory(), nil
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.games))
	for name := range r.games {
		names = append(names, name)
	}
	return names
}

func (r *Registry) Get(name string) (GameFactory, bool) {
	f, ok := r.games[name]
	return f, ok
}

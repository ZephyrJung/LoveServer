package template

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ZephyrJung/LoveServer/internal/game"
)

type Game struct {
	name        string
	displayName string
	state       *GameState
}

type GameState struct {
	StartedAt time.Time     `json:"started_at"`
	Moves     []MoveRecord  `json:"moves"`
	Status    string        `json:"status"` // "waiting", "playing", "ended"
}

type MoveRecord struct {
	PlayerID string          `json:"player_id"`
	Move     json.RawMessage `json:"move"`
	At       time.Time       `json:"at"`
}

func NewGame() *Game {
	return &Game{
		name:        "template",
		displayName: "Template Game",
	}
}

func (g *Game) Name() string { return g.name }

func (g *Game) DisplayName() string { return g.displayName }

func (g *Game) MinPlayers() int { return 2 }

func (g *Game) MaxPlayers() int { return 4 }

func (g *Game) OnInit(room interface{}, settings map[string]any) error {
	g.state = &GameState{
		Status: "waiting",
	}
	log.Printf("template game initialized for room %v", room)
	return nil
}

func (g *Game) OnPlayerJoin(room interface{}, player interface{}) error {
	log.Printf("player %v joined template game", player)
	return nil
}

func (g *Game) OnPlayerLeave(room interface{}, player interface{}) error {
	log.Printf("player %v left template game", player)
	return nil
}

func (g *Game) OnStart(room interface{}) error {
	g.state = &GameState{
		StartedAt: time.Now(),
		Status:    "playing",
	}
	log.Printf("template game started")
	return nil
}

func (g *Game) OnMessage(room interface{}, player interface{}, data json.RawMessage) (any, error) {
	if g.state == nil || g.state.Status != "playing" {
		return nil, fmt.Errorf("game not started")
	}

	// Record the move
	g.state.Moves = append(g.state.Moves, MoveRecord{
		PlayerID: fmt.Sprintf("%v", player),
		Move:     data,
		At:       time.Now(),
	})

	// Echo the move to all players
	return map[string]any{
		"type":    "move",
		"content": data,
	}, nil
}

func (g *Game) OnTick(room interface{}, dt float64) {
	// No-op for turn-based template
}

func (g *Game) OnEnd(room interface{}) (*game.GameResult, error) {
	if g.state != nil {
		g.state.Status = "ended"
	}
	stateJSON, _ := json.Marshal(g.state)
	return &game.GameResult{
		GameName: g.name,
		RawData:  stateJSON,
	}, nil
}

func (g *Game) MarshalState() ([]byte, error) {
	return json.Marshal(g.state)
}

func (g *Game) UnmarshalState(data []byte) error {
	return json.Unmarshal(data, &g.state)
}
package tankbattle

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ZephyrJung/LoveServer/internal/game"
)

// BroadcastFunc is a callback for broadcasting game state to all players
type BroadcastFunc func(eventType string, data interface{})

// Game implements the game.Game interface for tank battle
type Game struct {
	name        string
	displayName string

	mu         sync.RWMutex
	mode       GameMode
	phase      RoomState
	tick       int64
	nextID     int64
	players    map[int]*Player // playerID → Player (playerID = game-assigned ID)
	playerIDs  []int           // ordered list of player IDs
	bullets    []*Bullet
	gameMap    *MapState
	scores     map[string]int
	winner     interface{}

	// Input buffer: real player ID → buffered input
	inputBuffer map[string]*PlayerInput
	// PlayerID lookup: real player ID → game player ID
	playerIDMap map[string]int
	// Reverse lookup: game player ID → real player ID
	realIDMap map[int]string

	// Broadcast callback (set by the lobby handler)
	broadcast BroadcastFunc

	// Tick control
	stopTick atomic.Bool
}

// NewGame creates a new tank battle game instance
func NewGame() *Game {
	return &Game{
		name:        "tankbattle",
		displayName: "Tank Battle",
		players:     make(map[int]*Player),
		playerIDs:   make([]int, 0),
		bullets:     make([]*Bullet, 0),
		scores:      map[string]int{"red": 0, "blue": 0},
		inputBuffer: make(map[string]*PlayerInput),
		playerIDMap: make(map[string]int),
		realIDMap:   make(map[int]string),
	}
}

// SetBroadcast sets the broadcast callback for sending state to clients
func (g *Game) SetBroadcast(fn BroadcastFunc) {
	g.broadcast = fn
}

// ---- game.Game interface ----

func (g *Game) Name() string        { return g.name }
func (g *Game) DisplayName() string { return g.displayName }
func (g *Game) MinPlayers() int     { return MinPlayers }
func (g *Game) MaxPlayers() int     { return MaxPlayers }

func (g *Game) OnInit(room interface{}, settings map[string]any) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	mode := ModeDM
	if settings != nil {
		if m, ok := settings["mode"].(string); ok {
			mode = GameMode(m)
		}
	}

	g.mode = mode
	g.phase = PhaseLobby
	g.tick = 0
	g.nextID = 0
	g.players = make(map[int]*Player)
	g.playerIDs = make([]int, 0)
	g.bullets = make([]*Bullet, 0)
	g.gameMap = NewDefaultMap()
	g.scores = map[string]int{"red": 0, "blue": 0}
	g.winner = nil
	g.inputBuffer = make(map[string]*PlayerInput)
	g.playerIDMap = make(map[string]int)
	g.realIDMap = make(map[int]string)
	g.stopTick.Store(false)

	log.Printf("Tank battle initialized, mode=%s", mode)
	return nil
}

func (g *Game) OnPlayerJoin(room interface{}, player interface{}) error {
	return nil
}

func (g *Game) OnPlayerLeave(room interface{}, player interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// p is the realPlayerID string
	realID, ok := player.(string)
	if !ok {
		return nil
	}

	gameID, exists := g.playerIDMap[realID]
	if !exists {
		return nil
	}

	// Remove player from game
	delete(g.players, gameID)
	delete(g.playerIDMap, realID)
	delete(g.realIDMap, gameID)
	delete(g.inputBuffer, realID)

	// Remove from ordered list
	for i, id := range g.playerIDs {
		if id == gameID {
			g.playerIDs = append(g.playerIDs[:i], g.playerIDs[i+1:]...)
			break
		}
	}

	return nil
}

func (g *Game) OnStart(room interface{}) error {
	g.mu.Lock()
	g.phase = PhasePlaying
	g.tick = 0

	// Spawn all currently registered players
	nextGameID := 1
	for _, realID := range g.playerIDs {
		player := g.players[realID]
		if player == nil {
			continue
		}
		spawn := g.gameMap.GetSpawnPoint(nextGameID)
		player.X = spawn.X
		player.Y = spawn.Y
		player.Angle = 0
		player.TurretAngle = 0
		player.Health = TankMaxHealth
		player.Alive = true
		player.Kills = 0
		player.Deaths = 0
		player.spawnTimer = 0
		player.lastFire = 0
		nextGameID++
	}
	g.mu.Unlock()

	log.Printf("Tank battle started with %d players", len(g.playerIDs))

	// Start tick loop
	go g.tickLoop()

	return nil
}

func (g *Game) OnMessage(room interface{}, player interface{}, data json.RawMessage) (any, error) {
	// Parse the incoming message
	var msg struct {
		Seq  int              `json:"seq"`
		Keys map[string]bool  `json:"keys"`
		Fire bool             `json:"fire"`
		Type string           `json:"type"` // "input" or "fire"
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	realID, ok := player.(string)
	if !ok {
		return nil, fmt.Errorf("invalid player identifier")
	}

	// Handle fire message
	if msg.Type == "fire" || msg.Fire {
		g.mu.Lock()
		if p, exists := g.players[g.playerIDMap[realID]]; exists && p.Alive {
			g.fireBullet(p)
		}
		g.mu.Unlock()
		return nil, nil
	}

	// Buffer input for processing on next tick
	g.mu.Lock()
	g.inputBuffer[realID] = &PlayerInput{
		Seq:  msg.Seq,
		Keys: msg.Keys,
		Fire: msg.Fire,
	}
	g.mu.Unlock()

	return nil, nil
}

func (g *Game) OnTick(room interface{}, dt float64) {
	// OnTick is called by the tick loop goroutine, not externally
}

func (g *Game) OnEnd(room interface{}) (*game.GameResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.phase = PhaseGameOver
	g.stopTick.Store(true)

	state, _ := g.MarshalSnapshot()
	return &game.GameResult{
		GameName: g.name,
		RawData:  state,
	}, nil
}

func (g *Game) MarshalState() ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.MarshalSnapshot()
}

func (g *Game) UnmarshalState(data []byte) error {
	return json.Unmarshal(data, g)
}

// ---- Internal game logic ----

func (g *Game) tickLoop() {
	ticker := time.NewTicker(time.Duration(TickDuration * float64(time.Second)))
	defer ticker.Stop()

	for range ticker.C {
		if g.stopTick.Load() {
			return
		}

		g.mu.Lock()
		if g.phase != PhasePlaying {
			g.mu.Unlock()
			return
		}

		g.tick++

		// 1. Process buffered inputs
		for realID, input := range g.inputBuffer {
			gameID, exists := g.playerIDMap[realID]
			if !exists {
				continue
			}
			p := g.players[gameID]
			if p == nil || !p.Alive {
				continue
			}
			g.processInput(p, input)
		}
		g.inputBuffer = make(map[string]*PlayerInput)

		// 2. Update bullets
		g.updateBullets()

		// 3. Update respawns
		g.updateRespawns()

		// 4. Check win condition
		g.checkWinCondition()

		phase := g.phase
		winner := g.winner
		g.mu.Unlock()

		// Broadcast state snapshot
		if g.broadcast != nil {
			snapshot := g.GetSnapshot()
			g.broadcast("event.game_state", snapshot)
		}

		// If game over, stop
		if phase == PhaseGameOver {
			g.stopTick.Store(true)
			if g.broadcast != nil {
				g.broadcast("event.game_over", map[string]interface{}{
					"winner": winner,
				})
			}
			return
		}
	}
}

// processInput applies a player's input for one tick
func (g *Game) processInput(p *Player, input *PlayerInput) {
	dt := TickDuration

	// Rotate body
	if input.Keys["a"] {
		p.Angle -= dt * math.Pi
	}
	if input.Keys["d"] {
		p.Angle += dt * math.Pi
	}
	p.Angle = math.Mod(p.Angle, 2*math.Pi)
	if p.Angle < 0 {
		p.Angle += 2 * math.Pi
	}

	// Rotate turret
	if input.Keys["u"] {
		p.TurretAngle -= dt * math.Pi
	}
	if input.Keys["i"] {
		p.TurretAngle += dt * math.Pi
	}
	p.TurretAngle = math.Mod(p.TurretAngle, 2*math.Pi)
	if p.TurretAngle < 0 {
		p.TurretAngle += 2 * math.Pi
	}

	// Movement
	var dx, dy float64
	if input.Keys["w"] {
		dx += math.Sin(p.Angle) * TankSpeed * dt
		dy -= math.Cos(p.Angle) * TankSpeed * dt
	}
	if input.Keys["s"] {
		dx -= math.Sin(p.Angle) * TankSpeed * dt
		dy += math.Cos(p.Angle) * TankSpeed * dt
	}

	newX := p.X + dx
	newY := p.Y + dy

	// Collision: tank vs walls
	tankRect := Rect{X: newX, Y: newY, W: TankBodyW, H: TankBodyH}
	if !CollidesWithWalls(tankRect, g.gameMap.Walls) {
		p.X = newX
		p.Y = newY
	}

	// Fire
	if input.Fire {
		g.fireBullet(p)
	}
}

// fireBullet creates a bullet from the player's turret position
func (g *Game) fireBullet(p *Player) {
	now := float64(g.tick) * TickDuration
	if now-p.lastFire < BulletCooldown {
		return
	}
	p.lastFire = now

	cannonLen := float64(TankCannonH)
	cx := p.X + TankBodyW/2
	cy := p.Y + TankBodyH/2
	tipX := cx + math.Cos(p.TurretAngle)*(TankHeadRadius+cannonLen)
	tipY := cy + math.Sin(p.TurretAngle)*(TankHeadRadius+cannonLen)

	g.nextID++
	bullet := &Bullet{
		ID:      int(g.nextID),
		X:       tipX,
		Y:       tipY,
		VX:      math.Cos(p.TurretAngle) * BulletSpeed,
		VY:      math.Sin(p.TurretAngle) * BulletSpeed,
		OwnerID: p.ID,
	}
	g.bullets = append(g.bullets, bullet)
}

// updateBullets moves bullets and checks collisions
func (g *Game) updateBullets() {
	dt := TickDuration
	toRemove := make(map[int]bool)

	for i, b := range g.bullets {
		b.X += b.VX * dt
		b.Y += b.VY * dt

		// Out of bounds
		if IsOutOfBounds(b.X, b.Y) {
			toRemove[i] = true
			continue
		}

		// Collision: bullet vs walls
		bulletCircle := Circle{X: b.X, Y: b.Y, R: BulletRadius}
		if wallIdx := BulletCollidesWithWalls(bulletCircle, g.gameMap.Walls); wallIdx >= 0 {
			g.gameMap.DestroyWall(wallIdx)
			toRemove[i] = true
			continue
		}

		// Collision: bullet vs players
		for _, p := range g.players {
			if p.Alive && p.ID != b.OwnerID {
				playerRect := Rect{X: p.X, Y: p.Y, W: TankBodyW, H: TankBodyH}
				if CircleRectCollision(bulletCircle, playerRect) {
					p.Health--
					toRemove[i] = true

					// Credit kill
					if owner, ok := g.players[b.OwnerID]; ok {
						owner.Kills++
						if g.mode == ModeTeam && owner.Team == 1 {
							g.scores["red"]++
						} else if g.mode == ModeTeam && owner.Team == 2 {
							g.scores["blue"]++
						}
					}
					p.Deaths++

					if p.Health <= 0 {
						p.Alive = false
						p.spawnTimer = TankRespawnTime
					}
					break
				}
			}
		}
	}

	// Remove marked bullets (iterate in reverse)
	newBullets := make([]*Bullet, 0, len(g.bullets))
	for i, b := range g.bullets {
		if !toRemove[i] {
			newBullets = append(newBullets, b)
		}
	}
	g.bullets = newBullets
}

// updateRespawns handles player respawn timers
func (g *Game) updateRespawns() {
	for _, p := range g.players {
		if !p.Alive && p.spawnTimer > 0 {
			p.spawnTimer -= TickDuration
			if p.spawnTimer <= 0 {
				spawn := g.gameMap.GetSpawnPoint(p.ID)
				p.X = spawn.X
				p.Y = spawn.Y
				p.Health = TankMaxHealth
				p.Alive = true
				p.Angle = 0
				p.TurretAngle = 0
			}
		}
	}
}

// checkWinCondition checks if the game should end
func (g *Game) checkWinCondition() {
	if g.mode == ModeDM {
		for _, p := range g.players {
			if p.Kills >= DMKillLimit {
				g.phase = PhaseGameOver
				g.winner = p.ID
				return
			}
		}
	} else if g.mode == ModeTeam {
		if g.scores["red"] >= TeamKillLimit {
			g.phase = PhaseGameOver
			g.winner = "red"
		} else if g.scores["blue"] >= TeamKillLimit {
			g.phase = PhaseGameOver
			g.winner = "blue"
		}
	}
}

// GetSnapshot returns a serializable snapshot of the current game state
func (g *Game) GetSnapshot() *GameSnapshot {
	players := make([]*Player, 0, len(g.players))
	for _, p := range g.players {
		players = append(players, &Player{
			ID:          p.ID,
			Name:        p.Name,
			X:           p.X,
			Y:           p.Y,
			Angle:       p.Angle,
			TurretAngle: p.TurretAngle,
			Health:      p.Health,
			MaxHealth:   p.MaxHealth,
			Team:        p.Team,
			Alive:       p.Alive,
			Kills:       p.Kills,
			Deaths:      p.Deaths,
		})
	}

	walls := make([]*Wall, len(g.gameMap.Walls))
	for i, w := range g.gameMap.Walls {
		walls[i] = &Wall{X: w.X, Y: w.Y, W: w.W, H: w.H, Type: w.Type, Alive: w.Alive}
	}

	bullets := make([]*Bullet, len(g.bullets))
	for i, b := range g.bullets {
		bullets[i] = &Bullet{
			ID: b.ID, X: b.X, Y: b.Y, VX: b.VX, VY: b.VY, OwnerID: b.OwnerID,
		}
	}

	scoreCopy := map[string]int{"red": g.scores["red"], "blue": g.scores["blue"]}

	return &GameSnapshot{
		Tick:    g.tick,
		Phase:   g.phase,
		Mode:    g.mode,
		Players: players,
		Bullets: bullets,
		Walls:   walls,
		Scores:  scoreCopy,
		Winner:  g.winner,
	}
}

// MarshalSnapshot serializes the game state to JSON
func (g *Game) MarshalSnapshot() ([]byte, error) {
	return json.Marshal(g.GetSnapshot())
}

// RegisterPlayer adds a player to the game and assigns a game ID
func (g *Game) RegisterPlayer(realPlayerID, nickname string, team int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nextID++
	gameID := int(g.nextID)
	g.playerIDs = append(g.playerIDs, gameID)
	g.playerIDMap[realPlayerID] = gameID
	g.realIDMap[gameID] = realPlayerID

	g.players[gameID] = &Player{
		ID:        gameID,
		Name:      nickname,
		Team:      team,
		MaxHealth: TankMaxHealth,
		Health:    TankMaxHealth,
		Alive:     true,
	}
}

// GetPlayerID returns the game player ID for a real player ID
func (g *Game) GetPlayerID(realPlayerID string) (int, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	id, ok := g.playerIDMap[realPlayerID]
	return id, ok
}

// GetRealPlayerID returns the real player ID for a game player ID
func (g *Game) GetRealPlayerID(gameID int) (string, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	id, ok := g.realIDMap[gameID]
	return id, ok
}
package tankbattle

import "encoding/json"

// GameMode represents the game mode
type GameMode string

const (
	ModeDM   GameMode = "dm"
	ModeTeam GameMode = "team"
)

// RoomState represents the game phase
type RoomState string

const (
	PhaseLobby    RoomState = "lobby"
	PhasePlaying  RoomState = "playing"
	PhaseGameOver RoomState = "game_over"
)

// WallType represents wall material
type WallType string

const (
	WallSteel WallType = "steel"
	WallBrick WallType = "brick"
)

// Wall represents a wall in the map
type Wall struct {
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
	W     float64  `json:"w"`
	H     float64  `json:"h"`
	Type  WallType `json:"type"`
	Alive bool     `json:"alive"`
}

// Player represents a tank player
type Player struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Angle       float64 `json:"angle"`
	TurretAngle float64 `json:"turret_angle"`
	Health      int     `json:"health"`
	MaxHealth   int     `json:"max_health"`
	Team        int     `json:"team"`
	Alive       bool    `json:"alive"`
	Kills       int     `json:"kills"`
	Deaths      int     `json:"deaths"`

	// Internal state (not serialized)
	spawnTimer float64
	lastFire   float64
}

// Bullet represents a bullet in flight
type Bullet struct {
	ID      int     `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	VX      float64 `json:"vx"`
	VY      float64 `json:"vy"`
	OwnerID int     `json:"owner_id"`
}

// GameSnapshot is the serializable game state sent to clients
type GameSnapshot struct {
	Tick    int64              `json:"tick"`
	Phase   RoomState          `json:"phase"`
	Mode    GameMode           `json:"mode"`
	Players []*Player          `json:"players"`
	Bullets []*Bullet          `json:"bullets"`
	Walls   []*Wall            `json:"walls"`
	Scores  map[string]int     `json:"scores"`
	Winner  interface{}        `json:"winner"`
}

// PlayerInput represents a single input frame from a player
type PlayerInput struct {
	Seq  int            `json:"seq"`
	Keys map[string]bool `json:"keys"`
	Fire bool           `json:"fire"`
}

// SpawnPoint represents a player spawn location
type SpawnPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// MapState holds the map data
type MapState struct {
	Walls       []*Wall       `json:"walls"`
	SpawnPoints []SpawnPoint  `json:"spawn_points"`
}

// Spawn points (up to 6 players)
var DefaultSpawnPoints = []SpawnPoint{
	{X: 100, Y: 100}, {X: 700, Y: 100},
	{X: 100, Y: 500}, {X: 700, Y: 500},
	{X: 400, Y: 80}, {X: 400, Y: 520},
}

// NewDefaultMap creates the standard map layout
func NewDefaultMap() *MapState {
	m := &MapState{
		Walls:       make([]*Wall, 0),
		SpawnPoints: DefaultSpawnPoints,
	}

	// Border walls (steel, indestructible)
	m.Walls = append(m.Walls,
		&Wall{X: 0, Y: 0, W: ScreenWidth, H: WallThickness, Type: WallSteel, Alive: true},
		&Wall{X: 0, Y: ScreenHeight - WallThickness, W: ScreenWidth, H: WallThickness, Type: WallSteel, Alive: true},
		&Wall{X: 0, Y: 0, W: WallThickness, H: ScreenHeight, Type: WallSteel, Alive: true},
		&Wall{X: ScreenWidth - WallThickness, Y: 0, W: WallThickness, H: ScreenHeight, Type: WallSteel, Alive: true},
	)

	// Interior steel walls (decoration + cover)
	steelPositions := [][2]float64{
		{140, 140}, {380, 140}, {460, 140},
		{140, 460}, {380, 460}, {460, 460},
		{260, 290}, {540, 290},
	}
	for _, pos := range steelPositions {
		m.Walls = append(m.Walls, &Wall{X: pos[0], Y: pos[1], W: SteelW, H: SteelH, Type: WallSteel, Alive: true})
	}

	// Brick walls (destructible)
	brickPositions := [][2]float64{
		{380, 240}, {420, 240}, {380, 260}, {420, 260},
		{380, 340}, {420, 340}, {380, 360}, {420, 360},
		{80, 240}, {80, 260}, {80, 340}, {80, 360},
		{720, 240}, {720, 260}, {720, 340}, {720, 360},
		{260, 80}, {300, 80}, {500, 80}, {540, 80},
		{260, 520}, {300, 520}, {500, 520}, {540, 520},
	}
	for _, pos := range brickPositions {
		m.Walls = append(m.Walls, &Wall{X: pos[0], Y: pos[1], W: BrickW, H: BrickH, Type: WallBrick, Alive: true})
	}

	return m
}

// GetSpawnPoint returns the spawn point for a player by index
func (m *MapState) GetSpawnPoint(index int) SpawnPoint {
	points := m.SpawnPoints
	idx := ((index - 1) % len(points) + len(points)) % len(points)
	return points[idx]
}

// DestroyWall attempts to destroy a brick wall at the given index
func (m *MapState) DestroyWall(idx int) bool {
	if idx >= 0 && idx < len(m.Walls) {
		w := m.Walls[idx]
		if w.Type == WallBrick && w.Alive {
			w.Alive = false
			return true
		}
	}
	return false
}

// MarshalSnapshot serializes the game state to JSON
func (gs *GameSnapshot) MarshalJSON() ([]byte, error) {
	type Alias GameSnapshot
	return json.Marshal((*Alias)(gs))
}
# Game Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a multiplayer game server with extensible game types, room management, matchmaking, and whitelist auth.

**Architecture:** Hub-based message routing. WebSocket connections → Session layer → Auth → Hub dispatches to Lobby/Room/Game handlers. Redis for real-time state, PostgreSQL for persistence. Game types implement a common `Game` interface and register by name.

**Tech Stack:** Go 1.21+, gorilla/websocket, go-redis, pgx (PostgreSQL driver), gopkg.in/yaml.v3, google/uuid

## Global Constraints

- All message types use the `"type"` field for routing and `"data"` for payload
- All responses include `code` (int) and `msg` (string) fields
- All Game implementations go in `games/` directory as separate Go packages
- Session write operations use `sync.Mutex` for concurrent safety
- Room/Game state operations use `sync.RWMutex`
- Error codes follow the spec table (0 success, 1001-5001 ranges)
- Redis keys follow the format: `love:session:<playerID>`, `love:room:<roomID>`, `love:match:<gameName>`
- PostgreSQL table names use snake_case: `whitelist_tokens`, `game_records`
- Dependencies: gorilla/websocket, github.com/redis/go-redis/v9, github.com/jackc/pgx/v5, gopkg.in/yaml.v3, github.com/google/uuid

---
## Task 1: Project Scaffolding and Core Dependencies

**Files:**
- Create: `LoveServer/go.mod`
- Create: `LoveServer/config.yaml`
- Create: `LoveServer/main.go`
- Create: `LoveServer/internal/config/config.go`
- Modify: `LoveServer/.gitignore`

**Interfaces:**
- Consumes: nothing
- Produces: `Config` struct, `config.yaml` format, `main()` entry point, module path

- [ ] **Step 1: Initialize go module**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go mod init github.com/ZephyrJung/LoveServer
```

- [ ] **Step 2: Write config.yaml**

```yaml
server:
  addr: ":8080"
  auth_timeout: 10

postgres:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  dbname: "loveserver"
  sslmode: "disable"

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
```

- [ ] **Step 3: Write config.go**

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Postgres PostgresConfig `yaml:"postgres"`
	Redis    RedisConfig    `yaml:"redis"`
}

type ServerConfig struct {
	Addr        string `yaml:"addr"`
	AuthTimeout int    `yaml:"auth_timeout"` // seconds
}

type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DBName, p.SSLMode)
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
```

- [ ] **Step 4: Write main.go (stub)**

```go
package main

import (
	"log"

	"github.com/ZephyrJung/LoveServer/internal/config"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	_ = cfg
	log.Println("LoveServer starting...")
}
```

- [ ] **Step 5: Add .gitignore entry for .idea**

```gitignore
.idea
```

- [ ] **Step 6: Install dependencies**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go get github.com/gorilla/websocket@latest
go get github.com/redis/go-redis/v9@latest
go get github.com/jackc/pgx/v5@latest
go get gopkg.in/yaml.v3@latest
go get github.com/google/uuid@latest
go mod tidy
```

- [ ] **Step 7: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: project scaffolding with config and dependencies"
```

---

## Task 2: Storage Layer — PostgreSQL Client

**Files:**
- Create: `LoveServer/internal/storage/postgres.go`
- Create: `LoveServer/internal/storage/whitelist.go`

**Interfaces:**
- Consumes: `config.PostgresConfig` from Task 1
- Produces: `*pgx.Conn`, `WhitelistStore` with `CheckToken(token string) (playerID, nickname string, ok bool)`

- [ ] **Step 1: Write postgres.go**

```go
package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ZephyrJung/LoveServer/internal/config"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, cfg config.PostgresConfig) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) Pool() *pgxpool.Pool {
	return s.pool
}
```

- [ ] **Step 2: Write whitelist.go**

```go
package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrTokenNotFound = errors.New("whitelist token not found")

type WhitelistEntry struct {
	Token    string
	PlayerID string
	Nickname string
}

type WhitelistStore struct {
	db *PostgresStore
}

func NewWhitelistStore(db *PostgresStore) *WhitelistStore {
	return &WhitelistStore{db: db}
}

func (s *WhitelistStore) CheckToken(ctx context.Context, token string) (*WhitelistEntry, error) {
	entry := &WhitelistEntry{}
	err := s.db.Pool().QueryRow(ctx,
		"SELECT token, player_id, nickname FROM whitelist_tokens WHERE token = $1", token,
	).Scan(&entry.Token, &entry.PlayerID, &entry.Nickname)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	return entry, nil
}

func (s *WhitelistStore) CreateTable(ctx context.Context) error {
	_, err := s.db.Pool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS whitelist_tokens (
			token      VARCHAR(64) PRIMARY KEY,
			player_id  VARCHAR(32) UNIQUE NOT NULL,
			nickname   VARCHAR(64) NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	return err
}
```

- [ ] **Step 3: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: postgresql client and whitelist store"
```

---

## Task 3: Storage Layer — Redis Client

**Files:**
- Create: `LoveServer/internal/storage/redis.go`
- Create: `LoveServer/internal/storage/gamerecord.go`

**Interfaces:**
- Consumes: `config.RedisConfig` from Task 1
- Produces: `*RedisStore`, `GameRecordStore` with `SaveGameResult(ctx, result)`

- [ ] **Step 1: Write redis.go**

```go
package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/ZephyrJung/LoveServer/internal/config"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(ctx context.Context, cfg config.RedisConfig) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return &RedisStore{client: client}, nil
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}

func (s *RedisStore) Client() *redis.Client {
	return s.client
}
```

- [ ] **Step 2: Write gamerecord.go**

```go
package storage

import (
	"context"
	"encoding/json"
	"time"
)

type GameResult struct {
	GameName   string          `json:"game_name"`
	RoomID     string          `json:"room_id"`
	Players    []PlayerResult  `json:"players"`
	WinnerID   string          `json:"winner_id,omitempty"`
	RawData    json.RawMessage `json:"raw_data,omitempty"`
	StartedAt  time.Time       `json:"started_at"`
	EndedAt    time.Time       `json:"ended_at"`
}

type PlayerResult struct {
	PlayerID string `json:"player_id"`
	Nickname string `json:"nickname"`
	Score    int    `json:"score"`
	Rank     int    `json:"rank"`
}

type GameRecordStore struct {
	db *PostgresStore
}

func NewGameRecordStore(db *PostgresStore) *GameRecordStore {
	return &GameRecordStore{db: db}
}

func (s *GameRecordStore) CreateTable(ctx context.Context) error {
	_, err := s.db.Pool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS game_records (
			id          SERIAL PRIMARY KEY,
			game_name   VARCHAR(64) NOT NULL,
			room_id     VARCHAR(36) NOT NULL,
			players     JSONB NOT NULL,
			winner_id   VARCHAR(32),
			raw_data    JSONB,
			started_at  TIMESTAMP NOT NULL,
			ended_at    TIMESTAMP NOT NULL,
			created_at  TIMESTAMP DEFAULT NOW()
		)
	`)
	return err
}

func (s *GameRecordStore) SaveGameResult(ctx context.Context, result *GameResult) error {
	playersJSON, _ := json.Marshal(result.Players)
	var rawDataJSON []byte
	if result.RawData != nil {
		rawDataJSON = result.RawData
	}
	_, err := s.db.Pool().Exec(ctx, `
		INSERT INTO game_records (game_name, room_id, players, winner_id, raw_data, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, result.GameName, result.RoomID, playersJSON, result.WinnerID, rawDataJSON, result.StartedAt, result.EndedAt)
	return err
}
```

- [ ] **Step 3: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: redis client and game record store"
```

---

## Task 4: Message Protocol Definition

**Files:**
- Create: `LoveServer/internal/hub/message.go`

**Interfaces:**
- Consumes: nothing
- Produces: `Message`, `Response` structs, `ErrorCode` constants

- [ ] **Step 1: Write message.go**

```go
package hub

import "encoding/json"

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
	ID   string          `json:"id,omitempty"`
}

type Response struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	ID   string      `json:"id,omitempty"`
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
}

func NewResponse(msgType string, data interface{}, id string) *Response {
	return &Response{
		Type: msgType,
		Data: data,
		ID:   id,
		Code: 0,
		Msg:  "ok",
	}
}

func NewError(msgType string, code int, msg string, id string) *Response {
	return &Response{
		Type: msgType,
		Data: nil,
		ID:   id,
		Code: code,
		Msg:  msg,
	}
}

func NewEvent(eventType string, data interface{}) *Response {
	return &Response{
		Type: eventType,
		Data: data,
		Code: 0,
		Msg:  "ok",
	}
}

const (
	ErrSuccess            = 0
	ErrUnauthenticated    = 1001
	ErrAuthFailed         = 1002
	ErrInvalidMessage     = 1003
	ErrRoomNotFound       = 2001
	ErrRoomFull           = 2002
	ErrNotInRoom          = 2003
	ErrNotRoomOwner       = 2004
	ErrGameNotFound       = 3001
	ErrGameAlreadyStarted = 3002
	ErrMatchQueueFull     = 4001
	ErrInternal           = 5001
)

var errorMessages = map[int]string{
	ErrUnauthenticated:    "not authenticated",
	ErrAuthFailed:         "authentication failed",
	ErrInvalidMessage:     "invalid message format",
	ErrRoomNotFound:       "room not found",
	ErrRoomFull:           "room is full",
	ErrNotInRoom:          "not in a room",
	ErrNotRoomOwner:       "not the room owner",
	ErrGameNotFound:       "game type not found",
	ErrGameAlreadyStarted: "game already started",
	ErrMatchQueueFull:     "match queue is full",
	ErrInternal:           "internal server error",
}
```

- [ ] **Step 2: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: message protocol definitions and error codes"
```

---

## Task 5: Session Management

**Files:**
- Create: `LoveServer/internal/session/session.go`
- Create: `LoveServer/internal/session/manager.go`

**Interfaces:**
- Consumes: `*websocket.Conn` (gorilla/websocket), `Message`/`Response` from Task 4
- Produces: `Session`, `SessionManager` with `Get/GetByPlayer/Remove/Broadcast/SendToPlayer`

- [ ] **Step 1: Write session.go**

```go
package session

import (
	"sync"
	"time"
	"encoding/json"

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
```

- [ ] **Step 2: Write manager.go**

```go
package session

import (
	"sync"
	"encoding/json"
)

type Manager struct {
	byID      map[string]*Session   // sessionID → Session
	byPlayer  map[string]*Session   // playerID → Session
	mu        sync.RWMutex
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
```

- [ ] **Step 3: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: session management"
```

---

## Task 6: Auth System

**Files:**
- Create: `LoveServer/internal/auth/auth.go`

**Interfaces:**
- Consumes: `WhitelistStore` from Task 2, `SessionManager` from Task 5
- Produces: `AuthHandler` with `HandleLogin(session, msg) error`

- [ ] **Step 1: Write auth.go**

```go
package auth

import (
	"context"
	"encoding/json"
	"log"

	"github.com/ZephyrJung/LoveServer/internal/hub"
	"github.com/ZephyrJung/LoveServer/internal/session"
	"github.com/ZephyrJung/LoveServer/internal/storage"
)

type AuthHandler struct {
	whitelist *storage.WhitelistStore
	sessions  *session.Manager
	ctx       context.Context
}

func NewAuthHandler(ctx context.Context, whitelist *storage.WhitelistStore, sessions *session.Manager) *AuthHandler {
	return &AuthHandler{
		whitelist: whitelist,
		sessions:  sessions,
		ctx:       ctx,
	}
}

type LoginRequest struct {
	Token string `json:"token"`
}

type LoginResponse struct {
	PlayerID string `json:"player_id"`
	Nickname string `json:"nickname"`
}

func (h *AuthHandler) HandleLogin(s *session.Session, msg *hub.Message) error {
	var req LoginRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return s.SendJSON(hub.NewError("auth.login", hub.ErrInvalidMessage, "invalid login data", msg.ID))
	}

	entry, err := h.whitelist.CheckToken(h.ctx, req.Token)
	if err != nil {
		log.Printf("auth failed for token %s: %v", req.Token, err)
		return s.SendJSON(hub.NewError("auth.login", hub.ErrAuthFailed, "invalid token", msg.ID))
	}

	s.Authed = true
	s.Nickname = entry.Nickname
	h.sessions.BindPlayer(s.ID, entry.PlayerID)

	resp := hub.NewResponse("auth.login", LoginResponse{
		PlayerID: entry.PlayerID,
		Nickname: entry.Nickname,
	}, msg.ID)
	return s.SendJSON(resp)
}
```

- [ ] **Step 2: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: whitelist token auth handler"
```

---

## Task 7: Hub — Message Routing and Middleware

**Files:**
- Create: `LoveServer/internal/hub/hub.go`
- Create: `LoveServer/internal/hub/middleware.go`

**Interfaces:**
- Consumes: `Session`, `SessionManager` from Task 5, `Message`/`Response` from Task 4, `AuthHandler` from Task 6
- Produces: `Hub` with `HandleMessage`, `RegisterHandler`, middleware chain

- [ ] **Step 1: Write hub.go**

```go
package hub

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/ZephyrJung/LoveServer/internal/session"
)

type Handler interface {
	Routes() []string
	Handle(s *session.Session, msg *Message) error
}

type Middleware func(s *session.Session, msg *Message, next func() error) error

type Hub struct {
	handlers    []Handler
	middlewares []Middleware
}

func New() *Hub {
	return &Hub{
		handlers: make([]Handler, 0),
	}
}

func (h *Hub) RegisterHandler(handler Handler) {
	h.handlers = append(h.handlers, handler)
}

func (h *Hub) Use(m Middleware) {
	h.middlewares = append(h.middlewares, m)
}

func (h *Hub) HandleMessage(s *session.Session, raw []byte) {
	msg := &Message{}
	if err := json.Unmarshal(raw, msg); err != nil {
		log.Printf("invalid message from %s: %v", s.ID, err)
		s.SendJSON(NewError("", ErrInvalidMessage, "invalid json", ""))
		return
	}

	chain := h.buildChain(s, msg, func() error {
		return h.dispatch(s, msg)
	})

	if err := chain(); err != nil {
		log.Printf("handle message error: %v", err)
	}
}

func (h *Hub) dispatch(s *session.Session, msg *Message) error {
	for _, handler := range h.handlers {
		for _, route := range handler.Routes() {
			if strings.HasPrefix(msg.Type, route) {
				return handler.Handle(s, msg)
			}
		}
	}
	return s.SendJSON(NewError(msg.Type, ErrInvalidMessage, "unknown message type: "+msg.Type, msg.ID))
}

func (h *Hub) buildChain(s *session.Session, msg *Message, final func() error) func() error {
	chain := final
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		mw := h.middlewares[i]
		next := chain
		chain = func() error {
			return mw(s, msg, next)
		}
	}
	return chain
}
```

- [ ] **Step 2: Write middleware.go**

```go
package hub

import (
	"log"
	"strings"
	"time"

	"github.com/ZephyrJung/LoveServer/internal/session"
)

func LoggerMiddleware() Middleware {
	return func(s *session.Session, msg *Message, next func() error) error {
		start := time.Now()
		log.Printf("[%s] ← %s from %s", s.ID, msg.Type, s.PlayerID)
		err := next()
		log.Printf("[%s] → %s (%v)", s.ID, msg.Type, time.Since(start))
		return err
	}
}

func AuthMiddleware() Middleware {
	return func(s *session.Session, msg *Message, next func() error) error {
		if strings.HasPrefix(msg.Type, "auth.") {
			return next()
		}
		if !s.Authed {
			return s.SendJSON(NewError(msg.Type, ErrUnauthenticated, "not authenticated", msg.ID))
		}
		return next()
	}
}

func RateLimitMiddleware() Middleware {
	return func(s *session.Session, msg *Message, next func() error) error {
		return next()
	}
}
```

- [ ] **Step 3: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: hub message routing and middleware"
```

---

## Task 8: Game Interface and Registry

**Files:**
- Create: `LoveServer/internal/game/interface.go`
- Create: `LoveServer/internal/game/registry.go`
- Create: `LoveServer/internal/game/manager.go`
- Create: `LoveServer/internal/lobby/room.go`

**Interfaces:**
- Consumes: `Room` (defined here)
- Produces: `Game` interface, `GameRegistry`, `GameManager`, `Room` struct

- [ ] **Step 1: Write game/interface.go**

```go
package game

import (
	"encoding/json"
)

type GameResult struct {
	GameName string
	RoomID   string
	WinnerID string
	RawData  json.RawMessage
}

type Game interface {
	Name() string
	DisplayName() string
	MinPlayers() int
	MaxPlayers() int

	OnInit(room interface{}, settings map[string]any) error
	OnPlayerJoin(room interface{}, player interface{}) error
	OnPlayerLeave(room interface{}, player interface{}) error
	OnStart(room interface{}) error
	OnMessage(room interface{}, player interface{}, data json.RawMessage) (any, error)
	OnTick(room interface{}, dt float64)
	OnEnd(room interface{}) (*GameResult, error)

	MarshalState() ([]byte, error)
	UnmarshalState(data []byte) error
}
```

- [ ] **Step 2: Write game/registry.go**

```go
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
```

- [ ] **Step 3: Write game/manager.go**

```go
package game

import "sync"

type InstanceManager struct {
	instances map[string]Game // roomID → Game instance
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
```

- [ ] **Step 4: Write lobby/room.go**

```go
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
```

- [ ] **Step 5: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: game interface, registry, manager, and room struct"
```

---

## Task 9: Lobby — Room Management and Matching

**Files:**
- Create: `LoveServer/internal/lobby/lobby.go`
- Create: `LoveServer/internal/lobby/match.go`
- Create: `LoveServer/internal/lobby/errors.go`

**Interfaces:**
- Consumes: `Room`, `Player` from Task 8, `GameRegistry`/`InstanceManager` from Task 8, `SessionManager` from Task 5
- Produces: `Lobby` with `CreateRoom/JoinRoom/LeaveRoom/ListRooms`

- [ ] **Step 1: Write lobby/errors.go**

```go
package lobby

import "errors"

var (
	ErrRoomNotFound       = errors.New("room not found")
	ErrRoomFull           = errors.New("room is full")
	ErrNotInRoom          = errors.New("not in this room")
	ErrAlreadyInRoom      = errors.New("already in a room")
	ErrGameNotFound       = errors.New("game type not found")
	ErrNotRoomOwner       = errors.New("not the room owner")
	ErrGameAlreadyStarted = errors.New("game already started")
)
```

- [ ] **Step 2: Write lobby.go**

```go
package lobby

import (
	"sync"
	"time"

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
```

- [ ] **Step 3: Write match.go**

```go
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
	queues   map[string][]*MatchEntry // gameName → queue
	mu       sync.Mutex
	lobby    *Lobby
	registry *game.Registry
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
		}
		q.queues[gameName] = queue
	}
}
```

- [ ] **Step 4: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: lobby room management and matchmaking"
```

---

## Task 10: Lobby and Room Handlers

**Files:**
- Create: `LoveServer/internal/lobby/handler.go`

**Interfaces:**
- Consumes: `Lobby`, `Room`, `MatchQueue` from Task 9, `SessionManager` from Task 5, `Hub` from Task 7
- Produces: `LobbyHandler` implementing `hub.Handler`

- [ ] **Step 1: Write handler.go**

```go
package lobby

import (
	"encoding/json"
	"log"

	"github.com/ZephyrJung/LoveServer/internal/hub"
	"github.com/ZephyrJung/LoveServer/internal/session"
)

type LobbyHandler struct {
	lobby    *Lobby
	sessions *session.Manager
}

func NewLobbyHandler(lobby *Lobby, sessions *session.Manager) *LobbyHandler {
	return &LobbyHandler{
		lobby:    lobby,
		sessions: sessions,
	}
}

func (h *LobbyHandler) Routes() []string { return []string{"lobby", "match", "room", "game"} }

func (h *LobbyHandler) Handle(s *session.Session, msg *hub.Message) error {
	switch msg.Type {
	case "lobby.create_room":
		return h.handleCreateRoom(s, msg)
	case "lobby.list_rooms":
		return h.handleListRooms(s, msg)
	case "lobby.join_room":
		return h.handleJoinRoom(s, msg)
	case "lobby.leave_room":
		return h.handleLeaveRoom(s, msg)
	case "lobby.room_info":
		return h.handleRoomInfo(s, msg)
	case "match.start":
		return h.handleMatchStart(s, msg)
	case "match.cancel":
		return h.handleMatchCancel(s, msg)
	case "room.ready":
		return h.handleReady(s, msg)
	case "room.start_game":
		return h.handleStartGame(s, msg)
	case "room.chat":
		return h.handleChat(s, msg)
	case "game.move":
		return h.handleGameMove(s, msg)
	default:
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrInvalidMessage, "unknown lobby message", msg.ID))
	}
}

type CreateRoomRequest struct {
	Name     string         `json:"name"`
	GameName string         `json:"game_name"`
	Settings map[string]any `json:"settings,omitempty"`
}

func (h *LobbyHandler) handleCreateRoom(s *session.Session, msg *hub.Message) error {
	var req CreateRoomRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrInvalidMessage, "invalid data", msg.ID))
	}
	room, err := h.lobby.CreateRoom(req.Name, req.GameName, s.PlayerID, req.Settings)
	if err != nil {
		code := hub.ErrInternal
		switch err {
		case ErrGameNotFound:
			code = hub.ErrGameNotFound
		}
		return s.SendJSON(hub.NewError(msg.Type, code, err.Error(), msg.ID))
	}
	room.AddPlayer(s.PlayerID, s.Nickname)
	s.RoomID = room.ID
	return s.SendJSON(hub.NewResponse(msg.Type, roomToJSON(room), msg.ID))
}

type ListRoomsRequest struct {
	GameName string `json:"game_name,omitempty"`
}

func (h *LobbyHandler) handleListRooms(s *session.Session, msg *hub.Message) error {
	var req ListRoomsRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		req.GameName = ""
	}
	rooms := h.lobby.ListRooms(req.GameName)
	roomList := make([]map[string]any, 0, len(rooms))
	for _, r := range rooms {
		roomList = append(roomList, roomToJSON(r))
	}
	return s.SendJSON(hub.NewResponse(msg.Type, map[string]any{"rooms": roomList}, msg.ID))
}

type JoinRoomRequest struct {
	RoomID string `json:"room_id"`
}

func (h *LobbyHandler) handleJoinRoom(s *session.Session, msg *hub.Message) error {
	var req JoinRoomRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrInvalidMessage, "invalid data", msg.ID))
	}
	if s.RoomID != "" {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrInvalidMessage, "already in a room", msg.ID))
	}
	room, ok := h.lobby.GetRoom(req.RoomID)
	if !ok {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrRoomNotFound, "room not found", msg.ID))
	}
	if err := room.AddPlayer(s.PlayerID, s.Nickname); err != nil {
		code := hub.ErrRoomFull
		return s.SendJSON(hub.NewError(msg.Type, code, err.Error(), msg.ID))
	}
	s.RoomID = room.ID

	// Notify all players in room
	event := hub.NewEvent("event.player_joined", map[string]any{
		"room_id":  room.ID,
		"player_id": s.PlayerID,
		"nickname": s.Nickname,
	})
	eventJSON, _ := json.Marshal(event)
	h.sessions.Broadcast(room.ID, eventJSON)

	return s.SendJSON(hub.NewResponse(msg.Type, roomToJSON(room), msg.ID))
}

func (h *LobbyHandler) handleLeaveRoom(s *session.Session, msg *hub.Message) error {
	if s.RoomID == "" {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrNotInRoom, "not in a room", msg.ID))
	}
	room, ok := h.lobby.GetRoom(s.RoomID)
	if !ok {
		s.RoomID = ""
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrRoomNotFound, "room not found", msg.ID))
	}
	room.RemovePlayer(s.PlayerID)
	if room.PlayerCount() == 0 {
		h.lobby.RemoveRoom(room.ID)
	}
	s.RoomID = ""

	// Notify remaining players
	event := hub.NewEvent("event.player_left", map[string]any{
		"room_id":   room.ID,
		"player_id": s.PlayerID,
	})
	eventJSON, _ := json.Marshal(event)
	h.sessions.Broadcast(room.ID, eventJSON)

	return s.SendJSON(hub.NewResponse(msg.Type, map[string]string{"room_id": room.ID}, msg.ID))
}

func (h *LobbyHandler) handleRoomInfo(s *session.Session, msg *hub.Message) error {
	var req struct {
		RoomID string `json:"room_id"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil || req.RoomID == "" {
		req.RoomID = s.RoomID
	}
	room, ok := h.lobby.GetRoom(req.RoomID)
	if !ok {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrRoomNotFound, "room not found", msg.ID))
	}
	return s.SendJSON(hub.NewResponse(msg.Type, roomToJSON(room), msg.ID))
}

func (h *LobbyHandler) handleMatchStart(s *session.Session, msg *hub.Message) error {
	var req struct {
		GameName string `json:"game_name"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil || req.GameName == "" {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrInvalidMessage, "game_name required", msg.ID))
	}
	if err := h.lobby.MatchQueue().Enqueue(req.GameName, s.PlayerID, s.Nickname); err != nil {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrInternal, err.Error(), msg.ID))
	}
	return s.SendJSON(hub.NewResponse(msg.Type, map[string]string{"status": "queued"}, msg.ID))
}

func (h *LobbyHandler) handleMatchCancel(s *session.Session, msg *hub.Message) error {
	var req struct {
		GameName string `json:"game_name"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil || req.GameName == "" {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrInvalidMessage, "game_name required", msg.ID))
	}
	h.lobby.MatchQueue().Dequeue(req.GameName, s.PlayerID)
	return s.SendJSON(hub.NewResponse(msg.Type, map[string]string{"status": "cancelled"}, msg.ID))
}

func (h *LobbyHandler) handleReady(s *session.Session, msg *hub.Message) error {
	if s.RoomID == "" {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrNotInRoom, "not in a room", msg.ID))
	}
	room, ok := h.lobby.GetRoom(s.RoomID)
	if !ok {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrRoomNotFound, "room not found", msg.ID))
	}
	room.mu.Lock()
	if p, ok := room.Players[s.PlayerID]; ok {
		p.IsReady = !p.IsReady
	}
	room.mu.Unlock()

	event := hub.NewEvent("event.player_ready", map[string]any{
		"room_id":   room.ID,
		"player_id": s.PlayerID,
		"ready":     room.Players[s.PlayerID].IsReady,
	})
	eventJSON, _ := json.Marshal(event)
	h.sessions.Broadcast(room.ID, eventJSON)

	return s.SendJSON(hub.NewResponse(msg.Type, map[string]any{"ready": room.Players[s.PlayerID].IsReady}, msg.ID))
}

func (h *LobbyHandler) handleStartGame(s *session.Session, msg *hub.Message) error {
	if s.RoomID == "" {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrNotInRoom, "not in a room", msg.ID))
	}
	room, ok := h.lobby.GetRoom(s.RoomID)
	if !ok {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrRoomNotFound, "room not found", msg.ID))
	}
	if room.OwnerID != s.PlayerID {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrNotRoomOwner, "only room owner can start", msg.ID))
	}
	if room.State != RoomWaiting {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrGameAlreadyStarted, "game already started", msg.ID))
	}

	room.mu.Lock()
	room.State = RoomPlaying
	room.mu.Unlock()

	if room.GameInstance != nil {
		room.GameInstance.OnStart(room)
	}

	event := hub.NewEvent("event.game_started", map[string]any{
		"room_id": room.ID,
		"game":    room.GameName,
	})
	eventJSON, _ := json.Marshal(event)
	h.sessions.Broadcast(room.ID, eventJSON)

	return s.SendJSON(hub.NewResponse(msg.Type, map[string]string{"status": "started"}, msg.ID))
}

func (h *LobbyHandler) handleChat(s *session.Session, msg *hub.Message) error {
	if s.RoomID == "" {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrNotInRoom, "not in a room", msg.ID))
	}
	chatEvent := hub.NewEvent("event.chat", map[string]any{
		"room_id":   s.RoomID,
		"player_id": s.PlayerID,
		"nickname":  s.Nickname,
		"data":      msg.Data,
	})
	eventJSON, _ := json.Marshal(chatEvent)
	h.sessions.Broadcast(s.RoomID, eventJSON)
	return nil
}

func (h *LobbyHandler) handleGameMove(s *session.Session, msg *hub.Message) error {
	if s.RoomID == "" {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrNotInRoom, "not in a room", msg.ID))
	}
	room, ok := h.lobby.GetRoom(s.RoomID)
	if !ok {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrRoomNotFound, "room not found", msg.ID))
	}
	if room.State != RoomPlaying {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrGameAlreadyStarted, "game not started", msg.ID))
	}
	if room.GameInstance == nil {
		return s.SendJSON(hub.NewError(msg.Type, hub.ErrInternal, "no game instance", msg.ID))
	}

	player := room.Players[s.PlayerID]
	result, err := room.GameInstance.OnMessage(room, player, msg.Data)
	if err != nil {
		log.Printf("game move error: %v", err)
	}
	if result != nil {
		// Send game move result to all players
		gameEvent := hub.NewEvent("event.game_move", map[string]any{
			"room_id": room.ID,
			"player":  s.PlayerID,
			"result":  result,
		})
		eventJSON, _ := json.Marshal(gameEvent)
		h.sessions.Broadcast(room.ID, eventJSON)
	}
	return nil
}

func roomToJSON(room *Room) map[string]any {
	room.mu.RLock()
	defer room.mu.RUnlock()

	players := make([]map[string]any, 0, len(room.Players))
	for _, p := range room.Players {
		players = append(players, map[string]any{
			"id":       p.ID,
			"nickname": p.Nickname,
			"ready":    p.IsReady,
			"score":    p.Score,
		})
	}

	return map[string]any{
		"id":          room.ID,
		"name":        room.Name,
		"game_name":   room.GameName,
		"state":       roomStateToString(room.State),
		"owner_id":    room.OwnerID,
		"players":     players,
		"player_count": len(room.Players),
		"max_players": room.MaxPlayers,
		"min_players": room.MinPlayers,
		"created_at":  room.CreatedAt,
	}
}

func roomStateToString(s RoomState) string {
	switch s {
	case RoomWaiting:
		return "waiting"
	case RoomPlaying:
		return "playing"
	case RoomEnded:
		return "ended"
	case RoomClosed:
		return "closed"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 2: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: lobby and room message handlers"
```

---

## Task 11: WebSocket Server and Wire Everything Together

**Files:**
- Create: `LoveServer/internal/server/server.go`
- Modify: `LoveServer/main.go`
- Delete: `LoveServer/server/server.go`
- Delete: `LoveServer/client/client.go`

**Interfaces:**
- Consumes: Everything from Tasks 1-10
- Produces: Running server

- [ ] **Step 1: Write server.go**

```go
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/ZephyrJung/LoveServer/internal/auth"
	"github.com/ZephyrJung/LoveServer/internal/config"
	"github.com/ZephyrJung/LoveServer/internal/game"
	"github.com/ZephyrJung/LoveServer/internal/hub"
	"github.com/ZephyrJung/LoveServer/internal/lobby"
	"github.com/ZephyrJung/LoveServer/internal/session"
	"github.com/ZephyrJung/LoveServer/internal/storage"
)

type Server struct {
	cfg          *config.Config
	hub          *hub.Hub
	sessions     *session.Manager
	lobby        *lobby.Lobby
	gameRegistry *game.Registry
	gameManager  *game.InstanceManager
	pgStore      *storage.PostgresStore
	redisStore   *storage.RedisStore
	authHandler  *auth.AuthHandler
	upgrader     websocket.Upgrader
}

func New(cfg *config.Config) *Server {
	return &Server{
		cfg:      cfg,
		sessions: session.NewManager(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) Init(ctx context.Context) error {
	// Init storage
	pg, err := storage.NewPostgresStore(ctx, s.cfg.Postgres)
	if err != nil {
		return err
	}
	s.pgStore = pg

	rd, err := storage.NewRedisStore(ctx, s.cfg.Redis)
	if err != nil {
		return err
	}
	s.redisStore = rd

	// Create tables
	whitelistStore := storage.NewWhitelistStore(pg)
	if err := whitelistStore.CreateTable(ctx); err != nil {
		return err
	}
	recordStore := storage.NewGameRecordStore(pg)
	if err := recordStore.CreateTable(ctx); err != nil {
		return err
	}

	// Init game registry
	s.gameRegistry = game.NewRegistry()
	s.gameManager = game.NewInstanceManager(s.gameRegistry)

	// Init lobby
	s.lobby = lobby.NewLobby(s.gameRegistry, s.gameManager)
	s.lobby.StartMatchLoop(ctx)

	// Init hub
	s.hub = hub.New()
	s.hub.Use(hub.LoggerMiddleware())
	s.hub.Use(hub.AuthMiddleware())
	s.hub.Use(hub.RateLimitMiddleware())

	// Register handlers
	authHandler := auth.NewAuthHandler(ctx, whitelistStore, s.sessions)

	lobbyHandler := lobby.NewLobbyHandler(s.lobby, s.sessions)
	s.hub.RegisterHandler(lobbyHandler)

	// Store auth handler for WS connection use
	s.authHandler = authHandler

	log.Println("Server initialized")
	return nil
}

func (s *Server) Start() error {
	http.HandleFunc("/ws", s.handleWS)
	log.Printf("LoveServer listening on %s", s.cfg.Server.Addr)
	return http.ListenAndServe(s.cfg.Server.Addr, nil)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}

	sessionID := uuid.New().String()
	ses := session.NewSession(sessionID, conn)
	s.sessions.Add(ses)

	log.Printf("session %s connected", sessionID)

	// Auth timeout
	authTimer := time.AfterFunc(time.Duration(s.cfg.Server.AuthTimeout)*time.Second, func() {
		if !ses.Authed {
			log.Printf("session %s auth timeout", sessionID)
			ses.Close()
			s.sessions.Remove(sessionID)
		}
	})

	defer func() {
		authTimer.Stop()
		// Clean up: leave room, remove from match queue
		if ses.RoomID != "" {
			if room, ok := s.lobby.GetRoom(ses.RoomID); ok {
				room.RemovePlayer(ses.PlayerID)
				if room.PlayerCount() == 0 {
					s.lobby.RemoveRoom(room.ID)
				}
			}
		}
		s.sessions.Remove(sessionID)
		conn.Close()
		log.Printf("session %s disconnected", sessionID)
	}()

	// Read loop
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read error: %v", err)
			}
			break
		}

		// Handle auth message separately
		msg := &hub.Message{}
		if err := json.Unmarshal(raw, msg); err != nil {
			ses.SendJSON(hub.NewError("", hub.ErrInvalidMessage, "invalid json", ""))
			continue
		}

		if msg.Type == "auth.login" {
			s.authHandler.HandleLogin(ses, msg)
			if ses.Authed {
				authTimer.Stop()
			}
			continue
		}

		s.hub.HandleMessage(ses, raw)
	}
}
```

- [ ] **Step 2: Rewrite main.go**

```go
package main

import (
	"context"
	"log"

	"github.com/ZephyrJung/LoveServer/internal/config"
	"github.com/ZephyrJung/LoveServer/internal/server"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	srv := server.New(cfg)
	if err := srv.Init(ctx); err != nil {
		log.Fatalf("failed to init server: %v", err)
	}

	log.Fatal(srv.Start())
}
```

- [ ] **Step 3: Remove old server and client files**

```bash
rm /Users/zephyr/Documents/Projects/LoveServer/server/server.go
rm /Users/zephyr/Documents/Projects/LoveServer/client/client.go
```

- [ ] **Step 5: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: websocket server and integration wiring"
```

---

## Task 12: Template Game Implementation

**Files:**
- Create: `LoveServer/games/template/game.go`

**Interfaces:**
- Consumes: `game.Game` interface from Task 8, `lobby.Room`/`lobby.Player` from Task 8
- Produces: A working game type that can be registered and tested

- [ ] **Step 1: Write template game.go**

```go
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
	StartedAt time.Time   `json:"started_at"`
	Moves     []MoveRecord `json:"moves"`
	Status    string       `json:"status"` // "playing", "ended"
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
```

- [ ] **Step 2: Register template game in main.go**

```go
// After s.gameRegistry = game.NewRegistry()
s.gameRegistry.Register("template", func() game.Game { return template.NewGame() })
```

- [ ] **Step 3: Add import**

```go
"github.com/ZephyrJung/LoveServer/games/template"
```

- [ ] **Step 4: Build check**

```bash
cd /Users/zephyr/Documents/Projects/LoveServer
go build ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: template game implementation and registration"
```

---

## Task 13: Integration Test — Seed Data Script

**Files:**
- Create: `LoveServer/scripts/seed.sql`

- [ ] **Step 1: Write seed.sql**

```sql
-- Seed whitelist tokens for testing
INSERT INTO whitelist_tokens (token, player_id, nickname) VALUES
('test-token-001', 'player-001', 'PlayerOne'),
('test-token-002', 'player-002', 'PlayerTwo'),
('test-token-003', 'player-003', 'PlayerThree')
ON CONFLICT (token) DO NOTHING;
```

- [ ] **Step 2: Write a test client script

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: seed data script for testing"
```
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

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

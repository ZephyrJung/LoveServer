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
		"room_id":   room.ID,
		"player_id": s.PlayerID,
		"nickname":  s.Nickname,
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
		"id":           room.ID,
		"name":         room.Name,
		"game_name":    room.GameName,
		"state":        roomStateToString(room.State),
		"owner_id":     room.OwnerID,
		"players":      players,
		"player_count": len(room.Players),
		"max_players":  room.MaxPlayers,
		"min_players":  room.MinPlayers,
		"created_at":   room.CreatedAt,
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
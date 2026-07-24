package storage

import (
	"context"
	"encoding/json"
	"time"
)

type GameResult struct {
	GameName  string          `json:"game_name"`
	RoomID    string          `json:"room_id"`
	Players   []PlayerResult  `json:"players"`
	WinnerID  string          `json:"winner_id,omitempty"`
	RawData   json.RawMessage `json:"raw_data,omitempty"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   time.Time       `json:"ended_at"`
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

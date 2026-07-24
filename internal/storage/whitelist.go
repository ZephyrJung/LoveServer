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
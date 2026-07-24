-- Seed whitelist tokens for testing
INSERT INTO whitelist_tokens (token, player_id, nickname) VALUES
('test-token-001', 'player-001', 'PlayerOne'),
('test-token-002', 'player-002', 'PlayerTwo'),
('test-token-003', 'player-003', 'PlayerThree')
ON CONFLICT (token) DO NOTHING;
package store

import (
	"context"

	"unittrace/model"
)

// UpsertClient inserts or updates a client record by client_id (UUID string).
func (s *Store) UpsertClient(ctx context.Context, clientID string, displayName string) (*model.Client, error) {
	const q = `
		INSERT INTO clients (client_id, display_name, first_seen_at, last_seen_at)
		VALUES ($1::uuid, $2, now(), now())
		ON CONFLICT (client_id) DO UPDATE
			SET display_name = $2,
			    last_seen_at = now(),
			    updated_at   = now()
		RETURNING id, client_id, display_name, first_seen_at, last_seen_at`

	row := s.pool.QueryRow(ctx, q, clientID, displayName)

	c := &model.Client{}
	if err := row.Scan(&c.ID, &c.ClientID, &c.DisplayName, &c.FirstSeenAt, &c.LastSeenAt); err != nil {
		return nil, err
	}
	return c, nil
}

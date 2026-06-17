package store

import (
	"context"

	"unittrace/model"
)

// CreateNote adds a note to a tracked unit.
func (s *Store) CreateNote(ctx context.Context, unitID int64, authorClientID *int64, note string) (*model.Note, error) {
	const q = `
		INSERT INTO notes (tracked_unit_id, author_client_id, note)
		VALUES ($1, $2, $3)
		RETURNING id, tracked_unit_id, author_client_id, note, created_at`

	row := s.pool.QueryRow(ctx, q, unitID, authorClientID, note)

	n := &model.Note{}
	if err := row.Scan(&n.ID, &n.TrackedUnitID, &n.AuthorClientID, &n.Note, &n.CreatedAt); err != nil {
		return nil, err
	}
	return n, nil
}

// ListNotes returns all notes for a tracked unit, with author names, ordered by created_at ASC.
func (s *Store) ListNotes(ctx context.Context, unitID int64) ([]*model.Note, error) {
	const q = `
		SELECT n.id, n.tracked_unit_id, n.author_client_id, COALESCE(c.display_name, 'Unknown'), n.note, n.created_at
		FROM notes n
		LEFT JOIN clients c ON c.id = n.author_client_id
		WHERE n.tracked_unit_id = $1
		ORDER BY n.created_at ASC`

	rows, err := s.pool.Query(ctx, q, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		n := &model.Note{}
		if err := rows.Scan(&n.ID, &n.TrackedUnitID, &n.AuthorClientID, &n.AuthorName, &n.Note, &n.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

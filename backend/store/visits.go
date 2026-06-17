package store

import (
	"context"
	"time"

	"unittrace/model"
)

// CreateVisit records a listing visit and returns it.
func (s *Store) CreateVisit(ctx context.Context, unitID int64, snapshotID *int64, clientID *int64, source string, listingURL string, listingID string, at time.Time) (*model.ListingVisit, error) {
	const q = `
		INSERT INTO listing_visits (tracked_unit_id, snapshot_id, client_id, source, listing_url, listing_id, visited_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tracked_unit_id, snapshot_id, client_id, source, listing_url, listing_id, visited_at, created_at`

	row := s.pool.QueryRow(ctx, q, unitID, snapshotID, clientID, source, listingURL, listingID, at)

	v := &model.ListingVisit{}
	err := row.Scan(
		&v.ID, &v.TrackedUnitID, &v.SnapshotID, &v.ClientID,
		&v.Source, &v.ListingURL, &v.ListingID,
		&v.VisitedAt, &v.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}

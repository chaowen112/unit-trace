package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"unittrace/model"
)

// CreateVisit records a listing visit. If the same client visited the same listing
// within the last 30 minutes, it updates visited_at on the existing row instead of
// inserting a new one. Returns the visit and whether a new row was created.
func (s *Store) CreateVisit(ctx context.Context, unitID int64, snapshotID *int64, clientID *int64, source string, listingURL string, listingID string, at time.Time) (*model.ListingVisit, bool, error) {
	// Dedup: look for a recent visit from the same client for the same listing.
	if clientID != nil && listingID != "" {
		const dedup = `
			SELECT id, tracked_unit_id, snapshot_id, client_id, source, listing_url, listing_id, visited_at, created_at
			FROM listing_visits
			WHERE client_id = $1 AND listing_id = $2 AND source = $3
			  AND visited_at > now() - INTERVAL '30 minutes'
			ORDER BY visited_at DESC
			LIMIT 1`

		row := s.pool.QueryRow(ctx, dedup, *clientID, listingID, source)
		v := &model.ListingVisit{}
		err := row.Scan(
			&v.ID, &v.TrackedUnitID, &v.SnapshotID, &v.ClientID,
			&v.Source, &v.ListingURL, &v.ListingID,
			&v.VisitedAt, &v.CreatedAt,
		)
		if err == nil {
			// Found a recent visit — just refresh its timestamp.
			_, _ = s.pool.Exec(ctx, `UPDATE listing_visits SET visited_at = $1 WHERE id = $2`, at, v.ID)
			v.VisitedAt = at
			return v, false, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, false, err
		}
	}

	// No recent visit found — insert a new one.
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
		return nil, false, err
	}
	return v, true, nil
}

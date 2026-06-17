package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"unittrace/model"
)

// CreateSnapshot inserts a new listing snapshot and returns it.
func (s *Store) CreateSnapshot(ctx context.Context, params *CreateSnapshotParams) (*model.ListingSnapshot, error) {
	var rawPayload interface{}
	if params.RawPayload != nil {
		rawPayload = params.RawPayload
	}

	const q = `
		INSERT INTO listing_snapshots (
			source, listing_url, canonical_url, listing_id,
			captured_at, captured_by_client_id,
			title, asking_price, property_type, project_name, address_text, district,
			bedrooms, bathrooms, floor_area, floor_level_text,
			agent_name, agency_name,
			description_text, description_hash, image_set_hash, content_hash, raw_payload
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18,
			$19, $20, $21, $22, $23
		)
		RETURNING id, source, listing_url, canonical_url, listing_id,
		          captured_at, captured_by_client_id,
		          title, asking_price, property_type, project_name, address_text, district,
		          bedrooms, bathrooms, floor_area, floor_level_text,
		          agent_name, agency_name,
		          description_text, description_hash, image_set_hash, content_hash,
		          created_at`

	row := s.pool.QueryRow(ctx, q,
		params.Source, params.ListingURL, params.CanonicalURL, params.ListingID,
		params.CapturedAt, params.CapturedByClientID,
		params.Title, params.AskingPrice, params.PropertyType, params.ProjectName,
		params.AddressText, params.District,
		params.Bedrooms, params.Bathrooms, params.FloorArea, params.FloorLevelText,
		params.AgentName, params.AgencyName,
		params.DescriptionText, params.DescriptionHash, params.ImageSetHash, params.ContentHash,
		rawPayload,
	)

	return scanSnapshot(row)
}

// LinkSnapshotToUnit creates the tracked_unit_snapshots link record.
func (s *Store) LinkSnapshotToUnit(ctx context.Context, unitID int64, snapshotID int64, matchType string, score int, reasons []string) error {
	reasonsJSON, err := json.Marshal(reasons)
	if err != nil {
		return err
	}

	const q = `
		INSERT INTO tracked_unit_snapshots (tracked_unit_id, snapshot_id, match_type, match_score, match_reasons)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tracked_unit_id, snapshot_id) DO NOTHING`

	_, err = s.pool.Exec(ctx, q, unitID, snapshotID, matchType, score, reasonsJSON)
	return err
}

// GetLatestSnapshotForUnit retrieves the most recent snapshot for a tracked unit.
func (s *Store) GetLatestSnapshotForUnit(ctx context.Context, unitID int64) (*model.ListingSnapshot, error) {
	const q = `
		SELECT ls.id, ls.source, ls.listing_url, ls.canonical_url, ls.listing_id,
		       ls.captured_at, ls.captured_by_client_id,
		       ls.title, ls.asking_price, ls.property_type, ls.project_name, ls.address_text, ls.district,
		       ls.bedrooms, ls.bathrooms, ls.floor_area, ls.floor_level_text,
		       ls.agent_name, ls.agency_name,
		       ls.description_text, ls.description_hash, ls.image_set_hash, ls.content_hash,
		       ls.created_at
		FROM listing_snapshots ls
		JOIN tracked_unit_snapshots tus ON tus.snapshot_id = ls.id
		WHERE tus.tracked_unit_id = $1
		ORDER BY ls.captured_at DESC
		LIMIT 1`

	row := s.pool.QueryRow(ctx, q, unitID)
	snap, err := scanSnapshot(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return snap, err
}

// FindByListingID finds a tracked unit by source + listing_id.
// Returns (nil, nil, nil) if not found.
func (s *Store) FindByListingID(ctx context.Context, source string, listingID string) (*model.TrackedUnit, *model.ListingSnapshot, error) {
	const snapQ = `
		SELECT id, source, listing_url, canonical_url, listing_id,
		       captured_at, captured_by_client_id,
		       title, asking_price, property_type, project_name, address_text, district,
		       bedrooms, bathrooms, floor_area, floor_level_text,
		       agent_name, agency_name,
		       description_text, description_hash, image_set_hash, content_hash,
		       created_at
		FROM listing_snapshots
		WHERE source = $1 AND listing_id = $2
		ORDER BY captured_at DESC
		LIMIT 1`

	snapRow := s.pool.QueryRow(ctx, snapQ, source, listingID)
	snap, err := scanSnapshot(snapRow)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	unit, err := s.findUnitBySnapshotID(ctx, snap.ID)
	if err != nil {
		return nil, snap, err
	}
	return unit, snap, nil
}

// FindByCanonicalURL finds a tracked unit by source + canonical URL.
// Returns (nil, nil, nil) if not found.
func (s *Store) FindByCanonicalURL(ctx context.Context, source string, canonicalURL string) (*model.TrackedUnit, *model.ListingSnapshot, error) {
	const snapQ = `
		SELECT id, source, listing_url, canonical_url, listing_id,
		       captured_at, captured_by_client_id,
		       title, asking_price, property_type, project_name, address_text, district,
		       bedrooms, bathrooms, floor_area, floor_level_text,
		       agent_name, agency_name,
		       description_text, description_hash, image_set_hash, content_hash,
		       created_at
		FROM listing_snapshots
		WHERE source = $1 AND canonical_url = $2
		ORDER BY captured_at DESC
		LIMIT 1`

	snapRow := s.pool.QueryRow(ctx, snapQ, source, canonicalURL)
	snap, err := scanSnapshot(snapRow)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	unit, err := s.findUnitBySnapshotID(ctx, snap.ID)
	if err != nil {
		return nil, snap, err
	}
	return unit, snap, nil
}

// GetAllSnapshotsForUnit retrieves all snapshots for a tracked unit, ordered by captured_at.
func (s *Store) GetAllSnapshotsForUnit(ctx context.Context, unitID int64) ([]*model.ListingSnapshot, error) {
	const q = `
		SELECT ls.id, ls.source, ls.listing_url, ls.canonical_url, ls.listing_id,
		       ls.captured_at, ls.captured_by_client_id,
		       ls.title, ls.asking_price, ls.property_type, ls.project_name, ls.address_text, ls.district,
		       ls.bedrooms, ls.bathrooms, ls.floor_area, ls.floor_level_text,
		       ls.agent_name, ls.agency_name,
		       ls.description_text, ls.description_hash, ls.image_set_hash, ls.content_hash,
		       ls.created_at
		FROM listing_snapshots ls
		JOIN tracked_unit_snapshots tus ON tus.snapshot_id = ls.id
		WHERE tus.tracked_unit_id = $1
		ORDER BY ls.captured_at DESC`

	rows, err := s.pool.Query(ctx, q, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []*model.ListingSnapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}

// UpdateSnapshotImageSetHash updates the image_set_hash on a snapshot.
func (s *Store) UpdateSnapshotImageSetHash(ctx context.Context, snapshotID int64, hash string) error {
	const q = `UPDATE listing_snapshots SET image_set_hash = $1 WHERE id = $2`
	_, err := s.pool.Exec(ctx, q, hash, snapshotID)
	return err
}

// findUnitBySnapshotID looks up the tracked unit linked to a snapshot.
func (s *Store) findUnitBySnapshotID(ctx context.Context, snapshotID int64) (*model.TrackedUnit, error) {
	const q = `
		SELECT
			u.id, u.source, u.property_type, u.project_name, u.address_text, u.district, u.bedrooms,
			u.first_seen_at, u.last_seen_at, u.last_visited_at,
			u.first_seen_by_client_id, u.last_seen_by_client_id, u.last_visited_by_client_id,
			COALESCE(c1.display_name, ''), COALESCE(c2.display_name, ''), COALESCE(c3.display_name, ''),
			u.first_seen_price, u.current_price, u.lowest_seen_price, u.highest_seen_price,
			u.possible_relist_count, u.snapshot_count, u.visit_count, u.interest_label,
			u.created_at, u.updated_at
		FROM tracked_units u
		JOIN tracked_unit_snapshots tus ON tus.tracked_unit_id = u.id
		LEFT JOIN clients c1 ON c1.id = u.first_seen_by_client_id
		LEFT JOIN clients c2 ON c2.id = u.last_seen_by_client_id
		LEFT JOIN clients c3 ON c3.id = u.last_visited_by_client_id
		WHERE tus.snapshot_id = $1
		LIMIT 1`

	row := s.pool.QueryRow(ctx, q, snapshotID)
	unit, err := scanTrackedUnitFull(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return unit, err
}

func scanSnapshot(row scannerRow) (*model.ListingSnapshot, error) {
	snap := &model.ListingSnapshot{}
	err := row.Scan(
		&snap.ID, &snap.Source, &snap.ListingURL, &snap.CanonicalURL, &snap.ListingID,
		&snap.CapturedAt, &snap.CapturedByClientID,
		&snap.Title, &snap.AskingPrice, &snap.PropertyType, &snap.ProjectName,
		&snap.AddressText, &snap.District,
		&snap.Bedrooms, &snap.Bathrooms, &snap.FloorArea, &snap.FloorLevelText,
		&snap.AgentName, &snap.AgencyName,
		&snap.DescriptionText, &snap.DescriptionHash, &snap.ImageSetHash, &snap.ContentHash,
		&snap.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return snap, nil
}

package store

import (
	"context"
	"fmt"
	"time"

	"unittrace/matching"
	"unittrace/model"
)

// ListTrackedUnitsFilter holds optional filters for listing tracked units.
type ListTrackedUnitsFilter struct {
	PriceDropped   bool
	LikelyRelisted bool
	MinVisitCount  int
	InterestLabel  string
}

// CreateTrackedUnit inserts a new tracked unit and returns it.
func (s *Store) CreateTrackedUnit(ctx context.Context, params *CreateTrackedUnitParams) (*model.TrackedUnit, error) {
	const q = `
		INSERT INTO tracked_units (
			source, property_type, project_name, address_text, district, bedrooms,
			first_seen_by_client_id, last_seen_by_client_id,
			first_seen_price, current_price, lowest_seen_price, highest_seen_price,
			first_seen_at, last_seen_at,
			snapshot_count, visit_count, possible_relist_count,
			interest_label, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $7,
			$8, $8, $8, $8,
			$9, $9,
			0, 0, 0,
			'low', now()
		)
		RETURNING id, source, property_type, project_name, address_text, district, bedrooms,
		          first_seen_at, last_seen_at, last_visited_at,
		          first_seen_by_client_id, last_seen_by_client_id, last_visited_by_client_id,
		          first_seen_price, current_price, lowest_seen_price, highest_seen_price,
		          possible_relist_count, snapshot_count, visit_count, interest_label,
		          created_at, updated_at`

	row := s.pool.QueryRow(ctx, q,
		params.Source, params.PropertyType, params.ProjectName,
		params.AddressText, params.District, params.Bedrooms,
		params.FirstSeenByClientID, params.FirstSeenPrice, params.FirstSeenAt,
	)

	return scanTrackedUnit(row)
}

// GetTrackedUnit retrieves a tracked unit by ID, with client display names via joins.
func (s *Store) GetTrackedUnit(ctx context.Context, id int64) (*model.TrackedUnit, error) {
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
		LEFT JOIN clients c1 ON c1.id = u.first_seen_by_client_id
		LEFT JOIN clients c2 ON c2.id = u.last_seen_by_client_id
		LEFT JOIN clients c3 ON c3.id = u.last_visited_by_client_id
		WHERE u.id = $1`

	row := s.pool.QueryRow(ctx, q, id)
	return scanTrackedUnitFull(row)
}

// UpdateTrackedUnitOnSnapshot updates fields when a new snapshot is seen for a unit.
func (s *Store) UpdateTrackedUnitOnSnapshot(ctx context.Context, unitID int64, clientID int64, newPrice int64, at time.Time, isRelist bool) error {
	const q = `
		WITH stats AS (
			SELECT
				(SELECT COUNT(*) FROM notes WHERE tracked_unit_id = $1) AS note_count,
				(SELECT COUNT(DISTINCT client_id) FROM listing_visits WHERE tracked_unit_id = $1) AS unique_clients
		)
		UPDATE tracked_units SET
			last_seen_by_client_id  = $2,
			last_seen_at            = $3,
			current_price           = $4,
			lowest_seen_price       = LEAST(COALESCE(lowest_seen_price, $4), $4),
			highest_seen_price      = GREATEST(COALESCE(highest_seen_price, $4), $4),
			snapshot_count          = snapshot_count + 1,
			possible_relist_count   = possible_relist_count + CASE WHEN $5 THEN 1 ELSE 0 END,
			interest_label          = CASE
				WHEN (snapshot_count + 1) >= 8
				     OR visit_count >= 8
				     OR (stats.unique_clients >= 2 AND visit_count >= 5)
				     OR stats.note_count >= 2 THEN 'high'
				WHEN visit_count >= 3 THEN 'medium'
				ELSE 'low'
			END,
			updated_at = now()
		FROM stats
		WHERE tracked_units.id = $1`

	_, err := s.pool.Exec(ctx, q, unitID, clientID, at, newPrice, isRelist)
	return err
}

// UpdateTrackedUnitOnVisit updates fields when a unit is visited.
func (s *Store) UpdateTrackedUnitOnVisit(ctx context.Context, unitID int64, clientID int64, at time.Time) error {
	const q = `
		WITH stats AS (
			SELECT
				(SELECT COUNT(*) FROM notes WHERE tracked_unit_id = $1) AS note_count,
				(SELECT COUNT(DISTINCT client_id) FROM listing_visits WHERE tracked_unit_id = $1) AS unique_clients
		)
		UPDATE tracked_units SET
			visit_count             = visit_count + 1,
			last_visited_by_client_id = $2,
			last_visited_at         = $3,
			interest_label          = CASE
				WHEN (visit_count + 1) >= 8
				     OR snapshot_count >= 8
				     OR (stats.unique_clients >= 2 AND (visit_count + 1) >= 5)
				     OR stats.note_count >= 2 THEN 'high'
				WHEN (visit_count + 1) >= 3 THEN 'medium'
				ELSE 'low'
			END,
			updated_at = now()
		FROM stats
		WHERE tracked_units.id = $1`

	_, err := s.pool.Exec(ctx, q, unitID, clientID, at)
	return err
}

// GetClientVisitCounts returns visit counts per client for a given tracked unit.
func (s *Store) GetClientVisitCounts(ctx context.Context, unitID int64) ([]model.ClientVisitCount, error) {
	const q = `
		SELECT c.display_name, COUNT(*)::int AS visit_count
		FROM listing_visits lv
		JOIN clients c ON c.id = lv.client_id
		WHERE lv.tracked_unit_id = $1
		GROUP BY c.display_name
		ORDER BY visit_count DESC`

	rows, err := s.pool.Query(ctx, q, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.ClientVisitCount
	for rows.Next() {
		var vc model.ClientVisitCount
		if err := rows.Scan(&vc.DisplayName, &vc.VisitCount); err != nil {
			return nil, err
		}
		results = append(results, vc)
	}
	return results, rows.Err()
}

// ListTrackedUnits returns tracked units with optional filters.
func (s *Store) ListTrackedUnits(ctx context.Context, filter ListTrackedUnitsFilter) ([]*model.TrackedUnit, error) {
	q := `
		SELECT
			u.id, u.source, u.property_type, u.project_name, u.address_text, u.district, u.bedrooms,
			u.first_seen_at, u.last_seen_at, u.last_visited_at,
			u.first_seen_by_client_id, u.last_seen_by_client_id, u.last_visited_by_client_id,
			COALESCE(c1.display_name, ''), COALESCE(c2.display_name, ''), COALESCE(c3.display_name, ''),
			u.first_seen_price, u.current_price, u.lowest_seen_price, u.highest_seen_price,
			u.possible_relist_count, u.snapshot_count, u.visit_count, u.interest_label,
			u.created_at, u.updated_at
		FROM tracked_units u
		LEFT JOIN clients c1 ON c1.id = u.first_seen_by_client_id
		LEFT JOIN clients c2 ON c2.id = u.last_seen_by_client_id
		LEFT JOIN clients c3 ON c3.id = u.last_visited_by_client_id
		WHERE 1=1`

	args := []interface{}{}
	idx := 1

	if filter.PriceDropped {
		q += fmt.Sprintf(" AND u.current_price < u.first_seen_price")
	}
	if filter.LikelyRelisted {
		q += fmt.Sprintf(" AND u.possible_relist_count > 0")
	}
	if filter.MinVisitCount > 0 {
		q += fmt.Sprintf(" AND u.visit_count >= $%d", idx)
		args = append(args, filter.MinVisitCount)
		idx++
	}
	if filter.InterestLabel != "" {
		q += fmt.Sprintf(" AND u.interest_label = $%d", idx)
		args = append(args, filter.InterestLabel)
		idx++
	}

	q += " ORDER BY u.last_visited_at DESC NULLS LAST"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var units []*model.TrackedUnit
	for rows.Next() {
		u, err := scanTrackedUnitFull(rows)
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, rows.Err()
}

// GetFuzzyMatchCandidates retrieves candidate units for fuzzy matching.
// Filters by district, propertyType, and bedrooms where non-nil.
func (s *Store) GetFuzzyMatchCandidates(ctx context.Context, district *string, propertyType *string, bedrooms *int) ([]matching.Candidate, error) {
	q := `
		SELECT
			u.id, u.project_name, u.district, u.property_type, u.bedrooms
		FROM tracked_units u
		WHERE 1=1`

	args := []interface{}{}
	idx := 1

	if district != nil {
		q += fmt.Sprintf(" AND u.district = $%d", idx)
		args = append(args, *district)
		idx++
	}
	if propertyType != nil {
		q += fmt.Sprintf(" AND u.property_type = $%d", idx)
		args = append(args, *propertyType)
		idx++
	}
	if bedrooms != nil {
		q += fmt.Sprintf(" AND u.bedrooms = $%d", idx)
		args = append(args, *bedrooms)
		idx++
	}

	q += " LIMIT 200"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type unitRow struct {
		id           int64
		projectName  *string
		district     *string
		propertyType *string
		bedrooms     *int
	}

	var unitRows []unitRow
	for rows.Next() {
		var r unitRow
		if err := rows.Scan(&r.id, &r.projectName, &r.district, &r.propertyType, &r.bedrooms); err != nil {
			return nil, err
		}
		unitRows = append(unitRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	candidates := make([]matching.Candidate, 0, len(unitRows))
	for _, ur := range unitRows {
		snap, err := s.GetLatestSnapshotForUnit(ctx, ur.id)
		if err != nil {
			continue
		}

		c := matching.Candidate{
			TrackedUnitID: ur.id,
			ProjectName:   ur.projectName,
			District:      ur.district,
			PropertyType:  ur.propertyType,
			Bedrooms:      ur.bedrooms,
		}

		if snap != nil {
			c.Bathrooms = snap.Bathrooms
			c.FloorArea = snap.FloorArea
			c.FloorLevelText = snap.FloorLevelText
			c.AgentName = snap.AgentName
			c.Title = snap.Title
			c.DescriptionText = snap.DescriptionText
			c.AskingPrice = snap.AskingPrice

			// Get image URLs
			imageURLs, err := s.GetImageURLsForSnapshot(ctx, snap.ID)
			if err == nil {
				c.ImageURLs = imageURLs
			}
		}

		candidates = append(candidates, c)
	}

	return candidates, nil
}

// scannerRow is a common interface for pgx Row and Rows.
type scannerRow interface {
	Scan(dest ...any) error
}

func scanTrackedUnit(row scannerRow) (*model.TrackedUnit, error) {
	u := &model.TrackedUnit{}
	err := row.Scan(
		&u.ID, &u.Source, &u.PropertyType, &u.ProjectName, &u.AddressText, &u.District, &u.Bedrooms,
		&u.FirstSeenAt, &u.LastSeenAt, &u.LastVisitedAt,
		&u.FirstSeenByClientID, &u.LastSeenByClientID, &u.LastVisitedByClientID,
		&u.FirstSeenPrice, &u.CurrentPrice, &u.LowestSeenPrice, &u.HighestSeenPrice,
		&u.PossibleRelistCount, &u.SnapshotCount, &u.VisitCount, &u.InterestLabel,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func scanTrackedUnitFull(row scannerRow) (*model.TrackedUnit, error) {
	u := &model.TrackedUnit{}
	err := row.Scan(
		&u.ID, &u.Source, &u.PropertyType, &u.ProjectName, &u.AddressText, &u.District, &u.Bedrooms,
		&u.FirstSeenAt, &u.LastSeenAt, &u.LastVisitedAt,
		&u.FirstSeenByClientID, &u.LastSeenByClientID, &u.LastVisitedByClientID,
		&u.FirstSeenByName, &u.LastSeenByName, &u.LastVisitedByName,
		&u.FirstSeenPrice, &u.CurrentPrice, &u.LowestSeenPrice, &u.HighestSeenPrice,
		&u.PossibleRelistCount, &u.SnapshotCount, &u.VisitCount, &u.InterestLabel,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

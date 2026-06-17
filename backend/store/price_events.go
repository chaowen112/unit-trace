package store

import (
	"context"
	"time"

	"unittrace/model"
)

// CreatePriceEvent records a price event for a tracked unit.
// It computes event_type, price_delta, and price_delta_pct automatically.
func (s *Store) CreatePriceEvent(ctx context.Context, unitID int64, snapshotID int64, oldPrice *int64, newPrice int64, at time.Time) error {
	var eventType string
	var delta *int64
	var deltaPct *float64

	if oldPrice == nil {
		eventType = "first_seen"
	} else {
		d := newPrice - *oldPrice
		delta = &d
		if *oldPrice != 0 {
			pct := float64(d) / float64(*oldPrice) * 100
			deltaPct = &pct
		}
		switch {
		case newPrice < *oldPrice:
			eventType = "price_decreased"
		case newPrice > *oldPrice:
			eventType = "price_increased"
		default:
			eventType = "same_price_seen_again"
		}
	}

	const q = `
		INSERT INTO price_events (tracked_unit_id, snapshot_id, event_type, old_price, new_price, price_delta, price_delta_pct, detected_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := s.pool.Exec(ctx, q, unitID, snapshotID, eventType, oldPrice, newPrice, delta, deltaPct, at)
	return err
}

// GetPriceHistory returns all price events for a tracked unit, ordered by detected_at.
func (s *Store) GetPriceHistory(ctx context.Context, unitID int64) ([]*model.PriceEvent, error) {
	const q = `
		SELECT id, tracked_unit_id, snapshot_id, event_type, old_price, new_price, price_delta, price_delta_pct, detected_at
		FROM price_events
		WHERE tracked_unit_id = $1
		ORDER BY detected_at ASC`

	rows, err := s.pool.Query(ctx, q, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.PriceEvent
	for rows.Next() {
		e := &model.PriceEvent{}
		if err := rows.Scan(
			&e.ID, &e.TrackedUnitID, &e.SnapshotID, &e.EventType,
			&e.OldPrice, &e.NewPrice, &e.PriceDelta, &e.PriceDeltaPct,
			&e.DetectedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

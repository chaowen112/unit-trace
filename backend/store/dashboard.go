package store

import (
	"context"
	"time"
)

type DashboardStats struct {
	TotalUnits      int64
	TotalVisits     int64
	PossibleRelists int64
	RecentUnits     int64
}

type DashboardUnit struct {
	ID                  int64
	PropertyType        *string
	ProjectName         *string
	AddressText         *string
	District            *string
	Bedrooms            *int
	Bathrooms           *int
	FloorArea           *float64
	AgentName           *string
	Title               *string
	ListingURL          *string
	FirstSeenAt         *time.Time
	LastVisitedAt       *time.Time
	FirstSeenPrice      *int64
	CurrentPrice        *int64
	PSF                 *float64
	PriceChangePct      *float64 // e.g. -3.1 means −3.1%
	VisitCount          int
	SnapshotCount       int
	PossibleRelistCount int
	InterestLabel       *string
}

func (s *Store) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(visit_count), 0),
			COALESCE(SUM(possible_relist_count), 0),
			COUNT(*) FILTER (WHERE last_visited_at > now() - INTERVAL '7 days')
		FROM tracked_units
	`)
	st := &DashboardStats{}
	return st, row.Scan(&st.TotalUnits, &st.TotalVisits, &st.PossibleRelists, &st.RecentUnits)
}

func (s *Store) GetDashboardUnits(ctx context.Context) ([]DashboardUnit, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			tu.id,
			tu.property_type,
			tu.project_name,
			tu.address_text,
			tu.district,
			tu.bedrooms,
			tu.first_seen_at,
			tu.last_visited_at,
			tu.first_seen_price,
			tu.current_price,
			tu.visit_count,
			tu.snapshot_count,
			tu.possible_relist_count,
			tu.interest_label,
			snap.title,
			snap.floor_area,
			snap.bathrooms,
			snap.listing_url,
			snap.agent_name
		FROM tracked_units tu
		LEFT JOIN LATERAL (
			SELECT s.title, s.floor_area, s.bathrooms, s.listing_url, s.agent_name
			FROM listing_snapshots s
			JOIN tracked_unit_snapshots tus
				ON tus.snapshot_id = s.id AND tus.tracked_unit_id = tu.id
			ORDER BY s.captured_at DESC
			LIMIT 1
		) snap ON true
		ORDER BY tu.last_visited_at DESC NULLS LAST, tu.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var units []DashboardUnit
	for rows.Next() {
		var u DashboardUnit
		if err := rows.Scan(
			&u.ID, &u.PropertyType, &u.ProjectName, &u.AddressText,
			&u.District, &u.Bedrooms, &u.FirstSeenAt, &u.LastVisitedAt,
			&u.FirstSeenPrice, &u.CurrentPrice,
			&u.VisitCount, &u.SnapshotCount, &u.PossibleRelistCount, &u.InterestLabel,
			&u.Title, &u.FloorArea, &u.Bathrooms, &u.ListingURL, &u.AgentName,
		); err != nil {
			return nil, err
		}
		if u.FloorArea != nil && *u.FloorArea > 0 && u.CurrentPrice != nil && *u.CurrentPrice > 0 {
			v := float64(*u.CurrentPrice) / *u.FloorArea
			u.PSF = &v
		}
		if u.FirstSeenPrice != nil && *u.FirstSeenPrice > 0 &&
			u.CurrentPrice != nil && *u.CurrentPrice != *u.FirstSeenPrice {
			v := float64(*u.CurrentPrice-*u.FirstSeenPrice) / float64(*u.FirstSeenPrice) * 100
			u.PriceChangePct = &v
		}
		units = append(units, u)
	}
	return units, rows.Err()
}

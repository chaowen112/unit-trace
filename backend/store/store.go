package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps a pgxpool.Pool and provides all database operations.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a new Store, connects to the database, pings it, and runs the schema SQL.
func New(ctx context.Context, dsn string, schema string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	if schema != "" {
		if _, err := pool.Exec(ctx, schema); err != nil {
			pool.Close()
			return nil, err
		}
	}

	return &Store{pool: pool}, nil
}

// Close closes the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// CreateSnapshotParams holds all fields needed to create a listing snapshot.
type CreateSnapshotParams struct {
	Source             string
	ListingURL         string
	CanonicalURL       *string
	ListingID          *string
	CapturedAt         time.Time
	CapturedByClientID *int64
	Title              *string
	AskingPrice        *int64
	PropertyType       *string
	ProjectName        *string
	AddressText        *string
	District           *string
	Bedrooms           *int
	Bathrooms          *int
	FloorArea          *float64
	FloorLevelText     *string
	AgentName          *string
	AgencyName         *string
	DescriptionText    *string
	DescriptionHash    *string
	ImageSetHash       *string
	ContentHash        *string
	RawPayload         []byte
}

// CreateTrackedUnitParams holds fields needed to create a new tracked unit.
type CreateTrackedUnitParams struct {
	Source              *string
	PropertyType        *string
	ProjectName         *string
	AddressText         *string
	District            *string
	Bedrooms            *int
	FirstSeenByClientID int64
	FirstSeenPrice      int64
	FirstSeenAt         time.Time
}

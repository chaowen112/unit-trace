package store

import (
	"context"

	"unittrace/model"
)

// UpsertImage inserts or updates an image record by sha256 hash.
func (s *Store) UpsertImage(ctx context.Context, sha256 string, storagePath string, originalURL string, phash *string, width *int, height *int, contentType *string, fileSize *int64) (*model.Image, error) {
	const q = `
		INSERT INTO images (sha256_hash, storage_path, original_url, phash, width, height, content_type, file_size)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (sha256_hash) DO UPDATE
			SET phash        = EXCLUDED.phash,
			    storage_path = EXCLUDED.storage_path
		RETURNING id, original_url, storage_path, sha256_hash, phash, width, height, content_type, file_size, first_seen_at`

	row := s.pool.QueryRow(ctx, q, sha256, storagePath, originalURL, phash, width, height, contentType, fileSize)

	img := &model.Image{}
	if err := row.Scan(
		&img.ID, &img.OriginalURL, &img.StoragePath, &img.SHA256Hash,
		&img.PHash, &img.Width, &img.Height, &img.ContentType, &img.FileSize,
		&img.FirstSeenAt,
	); err != nil {
		return nil, err
	}
	return img, nil
}

// LinkImageToSnapshot links an image to a snapshot via listing_snapshot_images.
func (s *Store) LinkImageToSnapshot(ctx context.Context, snapshotID int64, imageID int64, sortOrder int, originalURL string) error {
	const q = `
		INSERT INTO listing_snapshot_images (snapshot_id, image_id, sort_order, original_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (snapshot_id, image_id) DO NOTHING`

	_, err := s.pool.Exec(ctx, q, snapshotID, imageID, sortOrder, originalURL)
	return err
}

// GetImageURLsForSnapshot returns original_urls for a snapshot's images, ordered by sort_order.
func (s *Store) GetImageURLsForSnapshot(ctx context.Context, snapshotID int64) ([]string, error) {
	const q = `
		SELECT original_url
		FROM listing_snapshot_images
		WHERE snapshot_id = $1
		ORDER BY sort_order ASC`

	rows, err := s.pool.Query(ctx, q, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []string
	for rows.Next() {
		var u *string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		if u != nil {
			urls = append(urls, *u)
		}
	}
	return urls, rows.Err()
}

// UpdateSnapshotImageSetHash is defined in snapshots.go.
// Store satisfies imgworker.Storer via that method.

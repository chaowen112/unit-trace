package imgworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/corona10/goimagehash"

	"unittrace/model"
)

// Storer is the interface imgworker needs from the store.
type Storer interface {
	UpsertImage(ctx context.Context, sha256 string, storagePath string, originalURL string, phash *string, width *int, height *int, contentType *string, fileSize *int64) (*model.Image, error)
	LinkImageToSnapshot(ctx context.Context, snapshotID int64, imageID int64, sortOrder int, originalURL string) error
	UpdateSnapshotImageSetHash(ctx context.Context, snapshotID int64, hash string) error
}

// Worker downloads, stores, and indexes listing images.
type Worker struct {
	imageDir string
	store    Storer
	client   *http.Client
}

// NewWorker creates a new Worker. It creates the imageDir if it doesn't exist.
func NewWorker(imageDir string, store Storer) (*Worker, error) {
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return nil, fmt.Errorf("imgworker: create image dir: %w", err)
	}
	return &Worker{
		imageDir: imageDir,
		store:    store,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Process downloads all image URLs, stores them to disk, upserts them in the DB,
// links them to the snapshot, then updates the snapshot's image_set_hash.
func (w *Worker) Process(ctx context.Context, snapshotID int64, imageURLs []string) {
	if len(imageURLs) == 0 {
		return
	}

	sha256s := make([]string, 0, len(imageURLs))

	for i, rawURL := range imageURLs {
		hash, err := w.processOne(ctx, snapshotID, i, rawURL)
		if err != nil {
			log.Printf("imgworker: snapshot %d image %d (%s): %v", snapshotID, i, rawURL, err)
			continue
		}
		sha256s = append(sha256s, hash)
	}

	if len(sha256s) == 0 {
		return
	}

	// Compute image_set_hash = sha256(sorted sha256s joined by ",")
	sorted := make([]string, len(sha256s))
	copy(sorted, sha256s)
	sort.Strings(sorted)
	combined := strings.Join(sorted, ",")
	setHash := fmt.Sprintf("%x", sha256.Sum256([]byte(combined)))

	if err := w.store.UpdateSnapshotImageSetHash(ctx, snapshotID, setHash); err != nil {
		log.Printf("imgworker: update image_set_hash for snapshot %d: %v", snapshotID, err)
	}
}

// processOne downloads a single image, stores it, and upserts it in the DB.
// Returns the sha256 hex string.
func (w *Worker) processOne(ctx context.Context, snapshotID int64, sortOrder int, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	// Compute sha256
	hashBytes := sha256.Sum256(data)
	hashHex := fmt.Sprintf("%x", hashBytes)

	// Determine content type and extension
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	ext := extFromContentType(contentType)

	// Storage path: imageDir/ab/abcd1234...jpg
	dir := filepath.Join(w.imageDir, hashHex[:2])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	storagePath := filepath.Join(dir, hashHex+ext)

	// Write file (only if not already present)
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		if err := os.WriteFile(storagePath, data, 0644); err != nil {
			return "", fmt.Errorf("write file: %w", err)
		}
	}

	// Compute image dimensions and perceptual hash
	var phash *string
	var width, height *int
	img, _, decodeErr := image.Decode(bytes.NewReader(data))
	if decodeErr == nil {
		bounds := img.Bounds()
		w2 := bounds.Dx()
		h2 := bounds.Dy()
		width = &w2
		height = &h2

		if hash, err := goimagehash.PerceptionHash(img); err == nil {
			hs := hash.ToString()
			phash = &hs
		}
	}

	fileSize := int64(len(data))

	imgRecord, err := w.store.UpsertImage(ctx, hashHex, storagePath, rawURL, phash, width, height, &contentType, &fileSize)
	if err != nil {
		return "", fmt.Errorf("upsert image: %w", err)
	}

	if err := w.store.LinkImageToSnapshot(ctx, snapshotID, imgRecord.ID, sortOrder, rawURL); err != nil {
		return "", fmt.Errorf("link image to snapshot: %w", err)
	}

	return hashHex, nil
}

func extFromContentType(ct string) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "webp"):
		return ".webp"
	default:
		return ".bin"
	}
}

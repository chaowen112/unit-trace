package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"unittrace/matching"
	"unittrace/model"
	"unittrace/store"
)

// SubmitListingViewRequest is the payload sent by the browser extension on each listing view.
type SubmitListingViewRequest struct {
	ClientID    string    `json:"client_id"`
	DisplayName string    `json:"display_name"`
	Source      string    `json:"source"`
	ListingURL  string    `json:"listing_url"`
	ListingID   string    `json:"listing_id"`
	CapturedAt  time.Time `json:"captured_at"`

	Title           string   `json:"title"`
	AskingPrice     int64    `json:"asking_price"`
	PropertyType    string   `json:"property_type"`
	ProjectName     string   `json:"project_name"`
	AddressText     string   `json:"address_text"`
	District        string   `json:"district"`
	Bedrooms        int      `json:"bedrooms"`
	Bathrooms       int      `json:"bathrooms"`
	FloorArea       float64  `json:"floor_area"`
	FloorLevelText  string   `json:"floor_level_text"`
	AgentName       string   `json:"agent_name"`
	AgencyName      string   `json:"agency_name"`
	DescriptionText string   `json:"description_text"`
	ImageURLs       []string `json:"image_urls"`
}

// SubmitListingViewResponse is returned to the caller (and drives the overlay UI).
type SubmitListingViewResponse struct {
	TrackedUnitID int64  `json:"tracked_unit_id"`
	Status        string `json:"status"` // "new", "seen_before", "likely_relisted", "almost_certain_same_unit", "possible_duplicate"
	MatchConfidence int  `json:"match_confidence"`

	FirstSeenAt   *time.Time `json:"first_seen_at,omitempty"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	LastVisitedAt *time.Time `json:"last_visited_at,omitempty"`
	FirstSeenBy   string     `json:"first_seen_by,omitempty"`
	LastSeenBy    string     `json:"last_seen_by,omitempty"`
	LastVisitedBy string     `json:"last_visited_by,omitempty"`

	FirstSeenPrice *int64   `json:"first_seen_price,omitempty"`
	CurrentPrice   *int64   `json:"current_price,omitempty"`
	PSF            *float64 `json:"psf,omitempty"`
	PriceChange    *int64   `json:"price_change,omitempty"`
	PriceChangePct *float64 `json:"price_change_pct,omitempty"`

	VisitCount          int `json:"visit_count"`
	SnapshotCount       int `json:"snapshot_count"`
	PossibleRelistCount int `json:"possible_relist_count"`

	ClientVisitCounts []model.ClientVisitCount `json:"client_visit_counts"`
	MatchReasons      []string                 `json:"match_reasons,omitempty"`
	PossibleDupOf     *int64                   `json:"possible_dup_of,omitempty"`
}

func (s *Server) handleSubmitListingView(w http.ResponseWriter, r *http.Request) {
	var req SubmitListingViewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.ClientID == "" {
		writeError(w, http.StatusBadRequest, "client_id is required")
		return
	}
	if req.Source == "" || req.ListingURL == "" {
		writeError(w, http.StatusBadRequest, "source and listing_url are required")
		return
	}

	ctx := r.Context()

	// 1. Upsert client
	client, err := s.store.UpsertClient(ctx, req.ClientID, req.DisplayName)
	if err != nil {
		log.Printf("handleSubmitListingView: upsert client: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to upsert client")
		return
	}

	capturedAt := req.CapturedAt
	if capturedAt.IsZero() {
		capturedAt = time.Now()
	}

	// Compute hashes
	descHash := hashString(req.DescriptionText)
	imgSetHash := imageSetHash(req.ImageURLs)
	contentHash := computeContentHash(&req, descHash)

	// Helper: pointer to non-zero string
	strPtr := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	intPtr := func(v int) *int {
		if v == 0 {
			return nil
		}
		return &v
	}
	floatPtr := func(v float64) *float64 {
		if v == 0 {
			return nil
		}
		return &v
	}
	int64Ptr := func(v int64) *int64 {
		if v == 0 {
			return nil
		}
		return &v
	}

	// 2. Try exact match
	exactMatch, err := s.matcher.ExactMatch(ctx, req.Source, req.ListingID, req.ListingURL)
	if err != nil {
		log.Printf("handleSubmitListingView: exact match: %v", err)
		writeError(w, http.StatusInternalServerError, "match error")
		return
	}

	type matchInfo struct {
		trackedUnitID int64
		score         int
		status        string
		isRelist      bool
		reasons       []string
	}

	var mi *matchInfo

	if exactMatch != nil {
		mi = &matchInfo{
			trackedUnitID: exactMatch.TrackedUnitID,
			score:         exactMatch.Score,
			status:        exactMatch.Status,
			isRelist:      exactMatch.IsRelist,
			reasons:       exactMatch.Reasons,
		}
	} else {
		// 3. Try fuzzy match
		mreq := &matching.MatchRequest{
			Source:          req.Source,
			Title:           req.Title,
			ProjectName:     req.ProjectName,
			District:        req.District,
			PropertyType:    req.PropertyType,
			Bedrooms:        req.Bedrooms,
			Bathrooms:       req.Bathrooms,
			FloorArea:       req.FloorArea,
			FloorLevelText:  req.FloorLevelText,
			AgentName:       req.AgentName,
			AskingPrice:     req.AskingPrice,
			DescriptionText: req.DescriptionText,
			ImageURLs:       req.ImageURLs,
		}
		fuzzy, err := s.matcher.FuzzyMatch(ctx, mreq)
		if err != nil {
			log.Printf("handleSubmitListingView: fuzzy match: %v", err)
			// non-fatal, proceed as new
		}
		if fuzzy != nil {
			mi = &matchInfo{
				trackedUnitID: fuzzy.TrackedUnitID,
				score:         fuzzy.Score,
				status:        fuzzy.Status,
				isRelist:      fuzzy.IsRelist,
				reasons:       fuzzy.Reasons,
			}
		}
	}

	// Build snapshot params
	snapParams := &store.CreateSnapshotParams{
		Source:             req.Source,
		ListingURL:         req.ListingURL,
		CanonicalURL:       strPtr(req.ListingURL),
		ListingID:          strPtr(req.ListingID),
		CapturedAt:         capturedAt,
		CapturedByClientID: &client.ID,
		Title:              strPtr(req.Title),
		AskingPrice:        int64Ptr(req.AskingPrice),
		PropertyType:       strPtr(req.PropertyType),
		ProjectName:        strPtr(req.ProjectName),
		AddressText:        strPtr(req.AddressText),
		District:           strPtr(req.District),
		Bedrooms:           intPtr(req.Bedrooms),
		Bathrooms:          intPtr(req.Bathrooms),
		FloorArea:          floatPtr(req.FloorArea),
		FloorLevelText:     strPtr(req.FloorLevelText),
		AgentName:          strPtr(req.AgentName),
		AgencyName:         strPtr(req.AgencyName),
		DescriptionText:    strPtr(req.DescriptionText),
		DescriptionHash:    &descHash,
		ImageSetHash:       &imgSetHash,
		ContentHash:        &contentHash,
	}

	var (
		trackedUnitID   int64
		snapshotID      int64
		responseStatus  string
		possibleDupOf   *int64
		newSnapshotMade bool
	)

	if mi != nil && mi.score >= 75 {
		// --- Matched existing unit ---
		trackedUnitID = mi.trackedUnitID
		responseStatus = mi.status

		latestSnap, err := s.store.GetLatestSnapshotForUnit(ctx, trackedUnitID)
		if err != nil {
			log.Printf("handleSubmitListingView: get latest snapshot: %v", err)
			writeError(w, http.StatusInternalServerError, "snapshot error")
			return
		}

		needsNewSnapshot := latestSnap == nil ||
			latestSnap.ContentHash == nil ||
			*latestSnap.ContentHash != contentHash

		if needsNewSnapshot {
			snap, err := s.store.CreateSnapshot(ctx, snapParams)
			if err != nil {
				log.Printf("handleSubmitListingView: create snapshot: %v", err)
				writeError(w, http.StatusInternalServerError, "snapshot error")
				return
			}
			snapshotID = snap.ID
			newSnapshotMade = true

			matchType := "fuzzy"
			if exactMatch != nil {
				matchType = "exact"
			}
			if err := s.store.LinkSnapshotToUnit(ctx, trackedUnitID, snapshotID, matchType, mi.score, mi.reasons); err != nil {
				log.Printf("handleSubmitListingView: link snapshot: %v", err)
			}

			var oldPrice *int64
			if latestSnap != nil {
				oldPrice = latestSnap.AskingPrice
			}
			if err := s.store.CreatePriceEvent(ctx, trackedUnitID, snapshotID, oldPrice, req.AskingPrice, capturedAt); err != nil {
				log.Printf("handleSubmitListingView: create price event: %v", err)
			}

			if err := s.store.UpdateTrackedUnitOnSnapshot(ctx, trackedUnitID, client.ID, req.AskingPrice, capturedAt, mi.isRelist); err != nil {
				log.Printf("handleSubmitListingView: update unit on snapshot: %v", err)
			}
		} else {
			snapshotID = latestSnap.ID
			// Still update seen timestamp even if content unchanged
			if err := s.store.UpdateTrackedUnitOnSnapshot(ctx, trackedUnitID, client.ID, req.AskingPrice, capturedAt, false); err != nil {
				log.Printf("handleSubmitListingView: update unit on snapshot (same): %v", err)
			}
		}

	} else if mi != nil && mi.score >= 60 {
		// --- Possible duplicate: create new unit but note the potential dup ---
		dupID := mi.trackedUnitID
		possibleDupOf = &dupID
		responseStatus = "possible_duplicate"

		newUnit, snap, err := s.createNewUnitAndSnapshot(ctx, client, &req, snapParams, capturedAt, strPtr, intPtr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		trackedUnitID = newUnit.ID
		snapshotID = snap.ID
		newSnapshotMade = true

	} else {
		// --- New listing ---
		responseStatus = "new"

		newUnit, snap, err := s.createNewUnitAndSnapshot(ctx, client, &req, snapParams, capturedAt, strPtr, intPtr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		trackedUnitID = newUnit.ID
		snapshotID = snap.ID
		newSnapshotMade = true
	}

	// Create visit record
	_, err = s.store.CreateVisit(ctx, trackedUnitID, &snapshotID, &client.ID, req.Source, req.ListingURL, req.ListingID, capturedAt)
	if err != nil {
		log.Printf("handleSubmitListingView: create visit: %v", err)
		writeError(w, http.StatusInternalServerError, "visit error")
		return
	}

	// Update unit on visit
	if err := s.store.UpdateTrackedUnitOnVisit(ctx, trackedUnitID, client.ID, capturedAt); err != nil {
		log.Printf("handleSubmitListingView: update unit on visit: %v", err)
	}

	// Async image processing
	if newSnapshotMade && len(req.ImageURLs) > 0 {
		go s.imageWorker.Process(context.Background(), snapshotID, req.ImageURLs)
	}

	// Get client visit counts
	clientVisits, err := s.store.GetClientVisitCounts(ctx, trackedUnitID)
	if err != nil {
		log.Printf("handleSubmitListingView: get client visit counts: %v", err)
	}
	if clientVisits == nil {
		clientVisits = []model.ClientVisitCount{}
	}

	// Get updated unit for response fields
	trackedUnit, err := s.store.GetTrackedUnit(ctx, trackedUnitID)
	if err != nil {
		log.Printf("handleSubmitListingView: get tracked unit: %v", err)
	}

	resp := SubmitListingViewResponse{
		TrackedUnitID:     trackedUnitID,
		Status:            responseStatus,
		ClientVisitCounts: clientVisits,
		PossibleDupOf:     possibleDupOf,
	}

	if mi != nil {
		resp.MatchConfidence = mi.score
		resp.MatchReasons = mi.reasons
	} else if responseStatus == "new" {
		resp.MatchConfidence = 0
	}

	if req.AskingPrice > 0 && req.FloorArea > 0 {
		psfVal := float64(req.AskingPrice) / req.FloorArea
		resp.PSF = &psfVal
	}

	if trackedUnit != nil {
		resp.VisitCount = trackedUnit.VisitCount
		resp.SnapshotCount = trackedUnit.SnapshotCount
		resp.PossibleRelistCount = trackedUnit.PossibleRelistCount
		resp.CurrentPrice = trackedUnit.CurrentPrice
		resp.FirstSeenAt = trackedUnit.FirstSeenAt
		resp.LastSeenAt = trackedUnit.LastSeenAt
		resp.LastVisitedAt = trackedUnit.LastVisitedAt
		resp.FirstSeenBy = trackedUnit.FirstSeenByName
		resp.LastSeenBy = trackedUnit.LastSeenByName
		resp.LastVisitedBy = trackedUnit.LastVisitedByName
		resp.FirstSeenPrice = trackedUnit.FirstSeenPrice

		if trackedUnit.FirstSeenPrice != nil && trackedUnit.CurrentPrice != nil &&
			*trackedUnit.FirstSeenPrice != *trackedUnit.CurrentPrice {
			delta := *trackedUnit.CurrentPrice - *trackedUnit.FirstSeenPrice
			resp.PriceChange = &delta
			if *trackedUnit.FirstSeenPrice != 0 {
				pct := float64(delta) / float64(*trackedUnit.FirstSeenPrice) * 100
				resp.PriceChangePct = &pct
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// createNewUnitAndSnapshot creates a new tracked unit with its first snapshot.
func (s *Server) createNewUnitAndSnapshot(
	ctx context.Context,
	client *model.Client,
	req *SubmitListingViewRequest,
	snapParams *store.CreateSnapshotParams,
	capturedAt time.Time,
	strPtr func(string) *string,
	intPtr func(int) *int,
) (*model.TrackedUnit, *model.ListingSnapshot, error) {
	unitParams := &store.CreateTrackedUnitParams{
		Source:              strPtr(req.Source),
		PropertyType:        strPtr(req.PropertyType),
		ProjectName:         strPtr(req.ProjectName),
		AddressText:         strPtr(req.AddressText),
		District:            strPtr(req.District),
		Bedrooms:            intPtr(req.Bedrooms),
		FirstSeenByClientID: client.ID,
		FirstSeenPrice:      req.AskingPrice,
		FirstSeenAt:         capturedAt,
	}

	unit, err := s.store.CreateTrackedUnit(ctx, unitParams)
	if err != nil {
		return nil, nil, fmt.Errorf("create tracked unit: %w", err)
	}

	snap, err := s.store.CreateSnapshot(ctx, snapParams)
	if err != nil {
		return nil, nil, fmt.Errorf("create snapshot: %w", err)
	}

	if err := s.store.LinkSnapshotToUnit(ctx, unit.ID, snap.ID, "new", 100, []string{"first_seen"}); err != nil {
		log.Printf("createNewUnitAndSnapshot: link snapshot: %v", err)
	}

	// Record first_seen price event
	if err := s.store.CreatePriceEvent(ctx, unit.ID, snap.ID, nil, req.AskingPrice, capturedAt); err != nil {
		log.Printf("createNewUnitAndSnapshot: price event: %v", err)
	}

	// Increment snapshot count
	if err := s.store.UpdateTrackedUnitOnSnapshot(ctx, unit.ID, client.ID, req.AskingPrice, capturedAt, false); err != nil {
		log.Printf("createNewUnitAndSnapshot: update unit on snapshot: %v", err)
	}

	return unit, snap, nil
}

// hashString returns the sha256 hex of s.
func hashString(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

// imageSetHash returns a stable hash of a sorted list of image URLs.
func imageSetHash(urls []string) string {
	sorted := make([]string, len(urls))
	copy(sorted, urls)
	sort.Strings(sorted)
	return hashString(strings.Join(sorted, ","))
}

// computeContentHash produces a hash of canonical listing fields.
func computeContentHash(req *SubmitListingViewRequest, descHash string) string {
	type canonical struct {
		Source         string  `json:"source"`
		ListingID      string  `json:"listing_id"`
		Title          string  `json:"title"`
		AskingPrice    int64   `json:"asking_price"`
		PropertyType   string  `json:"property_type"`
		ProjectName    string  `json:"project_name"`
		AddressText    string  `json:"address_text"`
		District       string  `json:"district"`
		Bedrooms       int     `json:"bedrooms"`
		Bathrooms      int     `json:"bathrooms"`
		FloorArea      float64 `json:"floor_area"`
		FloorLevelText string  `json:"floor_level_text"`
		AgentName      string  `json:"agent_name"`
		DescHash       string  `json:"description_hash"`
	}
	c := canonical{
		Source:         req.Source,
		ListingID:      req.ListingID,
		Title:          req.Title,
		AskingPrice:    req.AskingPrice,
		PropertyType:   req.PropertyType,
		ProjectName:    req.ProjectName,
		AddressText:    req.AddressText,
		District:       req.District,
		Bedrooms:       req.Bedrooms,
		Bathrooms:      req.Bathrooms,
		FloorArea:      req.FloorArea,
		FloorLevelText: req.FloorLevelText,
		AgentName:      req.AgentName,
		DescHash:       descHash,
	}
	b, _ := json.Marshal(c)
	return hashString(string(b))
}

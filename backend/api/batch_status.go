package api

import (
	"encoding/json"
	"log"
	"net/http"
)

type batchStatusRequest struct {
	Source     string   `json:"source"`
	ListingIDs []string `json:"listing_ids"`
}

type listingStatusResult struct {
	Status          string  `json:"status"`
	FirstSeenAt     *string `json:"first_seen_at,omitempty"`
	CurrentPrice    *int64  `json:"current_price,omitempty"`
	PriceChangePct  *float64 `json:"price_change_pct,omitempty"`
}

type batchStatusResponse struct {
	Results map[string]listingStatusResult `json:"results"`
}

func (s *Server) handleBatchListingStatus(w http.ResponseWriter, r *http.Request) {
	var req batchStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Source == "" {
		writeError(w, http.StatusBadRequest, "source is required")
		return
	}

	ctx := r.Context()
	results := make(map[string]listingStatusResult, len(req.ListingIDs))

	for _, listingID := range req.ListingIDs {
		unit, _, err := s.store.FindByListingID(ctx, req.Source, listingID)
		if err != nil {
			log.Printf("handleBatchListingStatus: FindByListingID(%s, %s): %v", req.Source, listingID, err)
			results[listingID] = listingStatusResult{Status: "error"}
			continue
		}

		if unit == nil {
			results[listingID] = listingStatusResult{Status: "not_tracked"}
			continue
		}

		status := "seen_before"
		if unit.PossibleRelistCount > 0 {
			status = "likely_relisted"
		}

		res := listingStatusResult{
			Status:       status,
			CurrentPrice: unit.CurrentPrice,
		}

		if unit.FirstSeenAt != nil {
			t := unit.FirstSeenAt.Format("2006-01-02T15:04:05Z07:00")
			res.FirstSeenAt = &t
		}

		if unit.FirstSeenPrice != nil && *unit.FirstSeenPrice > 0 && unit.CurrentPrice != nil {
			pct := float64(*unit.CurrentPrice-*unit.FirstSeenPrice) / float64(*unit.FirstSeenPrice) * 100
			res.PriceChangePct = &pct
		}

		results[listingID] = res
	}

	writeJSON(w, http.StatusOK, batchStatusResponse{Results: results})
}

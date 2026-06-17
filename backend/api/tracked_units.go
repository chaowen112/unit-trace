package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"unittrace/model"
	"unittrace/store"
)

func (s *Server) handleListTrackedUnits(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := store.ListTrackedUnitsFilter{}

	if q.Get("price_dropped") == "true" {
		filter.PriceDropped = true
	}
	if q.Get("likely_relisted") == "true" {
		filter.LikelyRelisted = true
	}
	if v := q.Get("min_visit_count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MinVisitCount = n
		}
	}
	if v := q.Get("interest"); v != "" {
		filter.InterestLabel = v
	}

	units, err := s.store.ListTrackedUnits(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tracked units: "+err.Error())
		return
	}
	if units == nil {
		units = []*model.TrackedUnit{}
	}

	writeJSON(w, http.StatusOK, units)
}

func (s *Server) handleGetTrackedUnit(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	ctx := r.Context()

	unit, err := s.store.GetTrackedUnit(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "tracked unit not found")
		return
	}

	snapshots, err := s.store.GetAllSnapshotsForUnit(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get snapshots: "+err.Error())
		return
	}
	if snapshots == nil {
		snapshots = []*model.ListingSnapshot{}
	}

	notes, err := s.store.ListNotes(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get notes: "+err.Error())
		return
	}
	if notes == nil {
		notes = []*model.Note{}
	}

	clientVisits, err := s.store.GetClientVisitCounts(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get visit counts: "+err.Error())
		return
	}
	if clientVisits == nil {
		clientVisits = []model.ClientVisitCount{}
	}

	priceHistory, err := s.store.GetPriceHistory(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get price history: "+err.Error())
		return
	}
	if priceHistory == nil {
		priceHistory = []*model.PriceEvent{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"unit":          unit,
		"snapshots":     snapshots,
		"notes":         notes,
		"client_visits": clientVisits,
		"price_history": priceHistory,
	})
}

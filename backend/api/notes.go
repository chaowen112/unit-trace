package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type addNoteRequest struct {
	ClientID    string `json:"client_id"`
	DisplayName string `json:"display_name"`
	Note        string `json:"note"`
}

func (s *Server) handleAddNote(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	unitID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req addNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Note == "" {
		writeError(w, http.StatusBadRequest, "note is required")
		return
	}

	ctx := r.Context()

	var clientID *int64
	if req.ClientID != "" {
		client, err := s.store.UpsertClient(ctx, req.ClientID, req.DisplayName)
		if err != nil {
			log.Printf("handleAddNote: upsert client: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to upsert client")
			return
		}
		clientID = &client.ID
	}

	note, err := s.store.CreateNote(ctx, unitID, clientID, req.Note)
	if err != nil {
		log.Printf("handleAddNote: create note: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create note")
		return
	}

	writeJSON(w, http.StatusCreated, note)
}

package httpapi

import (
	"encoding/json"
	"net/http"

	"rerum/backend/pkg/storage/sqlite"
)

func (s *Server) previewPropertyBatch(w http.ResponseWriter, r *http.Request) {
	var batch sqlite.PropertyBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "lote inválido", http.StatusUnprocessableEntity)
		return
	}
	items, err := batch.Preview()
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (s *Server) createPropertyBatch(w http.ResponseWriter, r *http.Request) {
	var batch sqlite.PropertyBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "lote inválido", http.StatusUnprocessableEntity)
		return
	}
	items, err := s.Store.SavePropertyBatch(r.Context(), batch)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, items)
}

func (s *Server) reactivateProject(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.ReactivateProject(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	s.getProject(w, r)
}

func (s *Server) deleteUnusedProject(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.DeleteUnusedProject(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

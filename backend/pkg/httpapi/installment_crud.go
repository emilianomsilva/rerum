package httpapi

import "net/http"

func (s *Server) deleteInstallment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	backup, err := s.Store.DeleteInstallment(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"backup_path": backup})
}

func (s *Server) cancelInstallment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.CancelInstallment(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

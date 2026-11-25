package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"rerum/backend/pkg/backup"
)

func (s *Server) backupFull(w http.ResponseWriter, r *http.Request) {
	file, err := os.CreateTemp("", "rerum-backup-*.zip")
	if err != nil {
		apiError(w, err)
		return
	}
	path := file.Name()
	_ = file.Close()
	defer os.Remove(path)
	if err := backup.Create(r.Context(), s.Store.DB, s.Store.ReceiptsDir, path); err != nil {
		apiError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="rerum-backup-%s.zip"`, time.Now().Format("2006-01-02")))
	http.ServeFile(w, r, path)
}

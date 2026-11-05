package handlers

import (
	"net/http"

	"banana-backend/store"
)

func HandleFileVersions(dbStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		fileID := r.PathValue("id")

		file, err := dbStore.GetFileForUser(fileID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"versions": file.Versions})
	}
}

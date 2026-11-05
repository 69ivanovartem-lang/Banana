package handlers

import (
	"net/http"

	"banana-backend/storage"
	"banana-backend/store"
)

func HandleListTrash(dbStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		items := dbStore.ListTrash(userID)
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	}
}

func HandleRestoreFromTrash(dbStore *store.Store, fileStorage *storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		trashID := r.PathValue("id")

		trash, err := dbStore.GetTrashItem(trashID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "trash item not found")
			return
		}

		newHash, err := fileStorage.RestoreFromTrash(trash.StorageKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot restore file")
			return
		}

		file, err := dbStore.RestoreFromTrash(trashID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "trash item not found")
			return
		}

		file.Versions[0].StorageKey = newHash

		writeJSON(w, http.StatusOK, file)
	}
}

func HandleDeleteFromTrash(dbStore *store.Store, fileStorage *storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		trashID := r.PathValue("id")

		trash, err := dbStore.GetTrashItem(trashID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "trash item not found")
			return
		}

		fileStorage.DeletePermanently(trash.StorageKey, true)
		dbStore.DeleteTrashItem(trashID, userID)

		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	}
}

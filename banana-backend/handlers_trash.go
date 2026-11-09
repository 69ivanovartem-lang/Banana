package main

import "net/http"

func handleListTrash(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		items := store.ListTrash(userID)
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	}
}

func handleRestoreTrash(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		if err := store.RestoreTrash(r.PathValue("id"), userID); err != nil {
			writeError(w, http.StatusNotFound, "trash item not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"restored": true})
	}
}

func handleDeleteTrashPermanent(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		if _, err := store.DeleteTrashPermanent(r.PathValue("id"), userID); err != nil {
			writeError(w, http.StatusNotFound, "trash item not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	}
}

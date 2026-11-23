package main

import (
	"net/http"
)

type UsageResponse struct {
	Used  int64 `json:"used"`
	Quota int64 `json:"quota"`
	Files int   `json:"files"`
}

func handleUsage(store *Store, quota int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())

		var used int64
		var fileCount int

		store.mu.RLock()
		for _, f := range store.db.Files {
			if f.UserID == userID {
				used += f.Size
				fileCount++
			}
		}
		store.mu.RUnlock()

		writeJSON(w, http.StatusOK, UsageResponse{
			Used:  used,
			Quota: quota,
			Files: fileCount,
		})
	}
}

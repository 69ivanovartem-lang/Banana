package main

import (
	"fmt"
	"net/http"
	"os"
)

func handleCreateShareLink(store *Store) http.HandlerFunc {
	type req struct {
		ResourceType string `json:"resourceType"`
		ResourceID   string `json:"resourceID"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if q.ResourceType != "file" && q.ResourceType != "folder" {
			writeError(w, http.StatusBadRequest, "resourceType must be 'file' or 'folder'")
			return
		}
		link, err := store.CreateShareLink(userID, q.ResourceType, q.ResourceID)
		if err != nil {
			writeError(w, http.StatusNotFound, "resource not found")
			return
		}
		writeJSON(w, http.StatusCreated, toShareLinkResponse(link))
	}
}

func handleAccessSharedResource(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		link, ok := store.GetShareLink(r.PathValue("token"))
		if !ok {
			writeError(w, http.StatusNotFound, "share link not found")
			return
		}
		if link.ResourceType == "file" {
			f, err := store.GetFile(link.ResourceID)
			if err != nil {
				writeError(w, http.StatusNotFound, "file not found")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"resourceType": "file", "file": toFileResponse(f)})
			return
		}
		folder, err := store.GetFolderForUser(link.ResourceID, link.UserID)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}
		files, folders := store.GetFolderItems(link.UserID, link.ResourceID)
		writeJSON(w, http.StatusOK, map[string]any{"resourceType": "folder", "folder": toFolderResponse(folder), "items": map[string]any{"files": files, "folders": folders}})
	}
}

func handleDownloadSharedFile(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		link, ok := store.GetShareLink(r.PathValue("token"))
		if !ok {
			writeError(w, http.StatusNotFound, "share link not found")
			return
		}
		fileID := r.URL.Query().Get("fileID")
		if link.ResourceType == "file" {
			if fileID == "" {
				fileID = link.ResourceID
			}
			if fileID != link.ResourceID {
				writeError(w, http.StatusNotFound, "file not found")
				return
			}
		} else if fileID == "" {
			writeError(w, http.StatusBadRequest, "fileID is required for folder share")
			return
		}
		f, err := store.GetFile(fileID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		if link.ResourceType == "folder" && f.FolderID != link.ResourceID {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		path, ok := store.blobFind(f.StorageKey)
		if !ok {
			writeError(w, http.StatusNotFound, "file data not found")
			return
		}
		fh, err := os.Open(path)
		if err != nil {
			writeError(w, http.StatusNotFound, "file data not found")
			return
		}
		defer fh.Close()
		w.Header().Set("Content-Type", f.ContentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFilename(f.Name)))
		http.ServeContent(w, r, f.Name, f.UpdatedAt, fh)
	}
}

func handleDeleteShareLink(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		if err := store.DeleteShareLink(r.PathValue("id"), userID); err != nil {
			writeError(w, http.StatusNotFound, "share link not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	}
}

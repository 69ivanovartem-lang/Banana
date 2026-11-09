package main

import (
	"net/http"
	"strings"
)

func handleCreateFolder(store *Store) http.HandlerFunc {
	type req struct {
		Name     string `json:"name"`
		ParentID string `json:"parentID"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if strings.TrimSpace(q.Name) == "" {
			writeError(w, http.StatusBadRequest, "folder name is required")
			return
		}
		if store.CheckDuplicateName(userID, q.ParentID, q.Name, "") {
			writeError(w, http.StatusConflict, "folder with this name already exists")
			return
		}
		folder, err := store.CreateFolder(userID, q.Name, q.ParentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid parent folder")
			return
		}
		writeJSON(w, http.StatusCreated, toFolderResponse(folder))
	}
}

func handleListFolders(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		parentID := r.URL.Query().Get("parentID")
		folders := store.ListFolders(userID, parentID)
		items := make([]folderResponse, 0, len(folders))
		for _, f := range folders {
			fr := toFolderResponse(f)
			fr.Size, fr.FileCount = store.folderStats(f.ID)
			items = append(items, fr)
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	}
}

func handleGetFolder(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		f, err := store.GetFolderForUser(r.PathValue("id"), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}
		writeJSON(w, http.StatusOK, toFolderResponse(f))
	}
}

func handleGetFolderItems(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.PathValue("id")
		if folderID == "root" {
			folderID = ""
		}
		files, folders := store.GetFolderItems(userID, folderID)
		writeJSON(w, http.StatusOK, map[string]any{"files": files, "folders": folders})
	}
}

func handleRenameFolder(store *Store) http.HandlerFunc {
	type req struct {
		Name string `json:"name"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.PathValue("id")
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if strings.TrimSpace(q.Name) == "" {
			writeError(w, http.StatusBadRequest, "folder name is required")
			return
		}
		folder, err := store.GetFolderForUser(folderID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}
		if store.CheckDuplicateName(userID, folder.ParentID, q.Name, folderID) {
			writeError(w, http.StatusConflict, "folder with this name already exists")
			return
		}
		renamed, err := store.RenameFolder(folderID, userID, q.Name)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}
		writeJSON(w, http.StatusOK, toFolderResponse(renamed))
	}
}

func handleDeleteFolder(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		item, err := store.DeleteFolder(r.PathValue("id"), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "trashId": item.ID})
	}
}

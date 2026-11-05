package handlers

import (
	"net/http"
	"strings"

	"banana-backend/store"
)

func HandleCreateFolder(dbStore *store.Store) http.HandlerFunc {
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

		q.Name = strings.TrimSpace(q.Name)
		if q.Name == "" {
			writeError(w, http.StatusBadRequest, "folder name is required")
			return
		}

		isDuplicate, _ := dbStore.CheckDuplicateName(userID, q.ParentID, q.Name, "")
		if isDuplicate {
			writeError(w, http.StatusConflict, "folder with this name already exists")
			return
		}

		folder, err := dbStore.CreateFolder(userID, q.Name, q.ParentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid parent folder")
			return
		}

		writeJSON(w, http.StatusCreated, folder)
	}
}

func HandleListFolders(dbStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		parentID := r.URL.Query().Get("parentID")

		folders := dbStore.ListFolders(userID, parentID)
		writeJSON(w, http.StatusOK, map[string]any{"items": folders, "total": len(folders)})
	}
}

func HandleGetFolder(dbStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.PathValue("id")

		folder, err := dbStore.GetFolderForUser(folderID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}

		writeJSON(w, http.StatusOK, folder)
	}
}

func HandleGetFolderItems(dbStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.PathValue("id")

		if folderID == "root" {
			folderID = ""
		}

		files, folders := dbStore.GetFolderItems(userID, folderID)

		for i := range folders {
			var size int64
			var count int
			dbStore.CountFilesRecursive(folders[i].ID, &size, &count)
			folders[i].Size = size
			folders[i].FileCount = count
		}

		writeJSON(w, http.StatusOK, map[string]any{"files": files, "folders": folders})
	}
}

func HandleRenameFolder(dbStore *store.Store) http.HandlerFunc {
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

		folder, err := dbStore.RenameFolder(folderID, userID, q.Name)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}

		writeJSON(w, http.StatusOK, folder)
	}
}

func HandleDeleteFolder(dbStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.PathValue("id")

		folder, err := dbStore.GetFolderForUser(folderID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}

		parentID := folder.ParentID

		if err := dbStore.DeleteFolder(folderID, userID); err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}

		if parentID != "" {
			dbStore.UpdateFolderStats(parentID)
		}

		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	}
}

package handlers

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"banana-backend/models"
	"banana-backend/storage"
	"banana-backend/store"
)

func determineFileType(contentType, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if strings.HasPrefix(contentType, "image/") {
		return "image"
	}
	if strings.HasPrefix(contentType, "video/") {
		return "video"
	}
	if strings.Contains(contentType, "pdf") {
		return "document"
	}
	if ext == ".txt" || ext == ".go" || ext == ".py" || ext == ".js" || ext == ".html" || ext == ".json" {
		return "code"
	}
	if ext == ".zip" || ext == ".rar" || ext == ".gz" {
		return "archive"
	}
	return "other"
}

func HandleListFiles(dbStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.URL.Query().Get("folderID")

		files := dbStore.ListFiles(userID, folderID)

		sort.Slice(files, func(i, j int) bool {
			if files[i].FileType != files[j].FileType {
				return files[i].FileType < files[j].FileType
			}
			return files[i].Name < files[j].Name
		})

		writeJSON(w, http.StatusOK, map[string]any{"items": files, "total": len(files)})
	}
}

func HandleUploadFile(dbStore *store.Store, fileStorage *storage.Storage, maxUploadBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.URL.Query().Get("folderID")

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			writeError(w, http.StatusBadRequest, "invalid multipart form")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file is required")
			return
		}
		defer file.Close()

		hash, size, err := fileStorage.SaveFile(file)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot save file")
			return
		}

		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		originalName := header.Filename
		uniqueName := dbStore.GenerateUniqueName(userID, folderID, originalName, "")

		meta := models.FileMeta{
			ID:          randomHex(16),
			UserID:      userID,
			Name:        uniqueName,
			Size:        size,
			ContentType: contentType,
			FileType:    determineFileType(contentType, uniqueName),
			FolderID:    folderID,
			Versions:    []models.FileVersion{{StorageKey: hash, Size: size}},
			CreatedAt:   time.Now().UTC(),
		}

		if err := dbStore.AddFile(meta); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot save file metadata")
			return
		}

		dbStore.UpdateFolderStats(folderID)

		writeJSON(w, http.StatusCreated, meta)
	}
}

func HandleGetFile(dbStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		fileID := r.PathValue("id")

		file, err := dbStore.GetFileForUser(fileID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}

		writeJSON(w, http.StatusOK, file)
	}
}

func HandleDownloadFile(dbStore *store.Store, fileStorage *storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		fileID := r.PathValue("id")

		file, err := dbStore.GetFileForUser(fileID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}

		latestVersion := file.Versions[len(file.Versions)-1]
		f, err := fileStorage.GetFile(latestVersion.StorageKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "file data not found")
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", file.ContentType)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Name+"\"")
		http.ServeContent(w, r, file.Name, file.UpdatedAt, f)
	}
}

func HandlePreview(dbStore *store.Store, fileStorage *storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		fileID := r.PathValue("id")

		file, err := dbStore.GetFileForUser(fileID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}

		latestVersion := file.Versions[len(file.Versions)-1]
		f, err := fileStorage.GetFile(latestVersion.StorageKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "file data not found")
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", file.ContentType)
		w.Header().Set("Content-Disposition", "inline; filename=\""+file.Name+"\"")
		http.ServeContent(w, r, file.Name, file.UpdatedAt, f)
	}
}

func HandleDeleteFile(dbStore *store.Store, fileStorage *storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		fileID := r.PathValue("id")

		file, err := dbStore.GetFileForUser(fileID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}

		folderID := file.FolderID

		latestVersion := file.Versions[len(file.Versions)-1]
		trashKey, _, err := fileStorage.CompressAndMoveToTrash(latestVersion.StorageKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot move to trash")
			return
		}

		trashItem, err := dbStore.MoveToTrash(fileID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}

		trashItem.StorageKey = trashKey

		dbStore.UpdateFolderStats(folderID)

		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "trashID": trashItem.ID})
	}
}

func HandleMoveFile(dbStore *store.Store) http.HandlerFunc {
	type req struct {
		FolderID string `json:"folderID"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		fileID := r.PathValue("id")

		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		file, err := dbStore.MoveFile(fileID, userID, q.FolderID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid folder")
			return
		}

		writeJSON(w, http.StatusOK, file)
	}
}

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func determineFileType(contentType, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if strings.HasPrefix(contentType, "image/") {
		return "image"
	}
	if strings.HasPrefix(contentType, "video/") {
		return "video"
	}
	if strings.HasPrefix(contentType, "audio/") {
		return "audio"
	}
	if strings.Contains(contentType, "pdf") || ext == ".ppt" || ext == ".pptx" {
		return "document"
	}
	if ext == ".txt" || ext == ".go" || ext == ".py" || ext == ".js" || ext == ".ts" || ext == ".html" || ext == ".css" || ext == ".json" || ext == ".md" || ext == ".c" || ext == ".cpp" || ext == ".java" || ext == ".sh" {
		return "code"
	}
	if ext == ".zip" || ext == ".rar" || ext == ".gz" || ext == ".7z" || ext == ".tar" {
		return "archive"
	}
	return "other"
}

func handleListFiles(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.URL.Query().Get("folderID")
		files := store.ListFiles(userID, folderID)
		items := make([]fileResponse, 0, len(files))
		for _, f := range files {
			items = append(items, toFileResponse(f))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	}
}

func handleUploadFile(store *Store, maxUpload int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.URL.Query().Get("folderID")
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if err := r.ParseMultipartForm(maxUpload); err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "file too large")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file is required, use field name 'file'")
			return
		}
		defer file.Close()
		hash, size, err := store.saveBlob(file)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot save blob")
			return
		}
		original := safeFilename(header.Filename)
		if original == "download" {
			original = "file-" + randomHex(8)
		}
		name := store.uniqueIfTaken(userID, folderID, original)
		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		now := time.Now().UTC()
		meta := FileMeta{
			ID: randomHex(16), UserID: userID, Name: name, Size: size,
			ContentType: contentType, FileType: determineFileType(contentType, name),
			StorageKey: hash, FolderID: folderID, CreatedAt: now, UpdatedAt: now,
		}
		if err := store.AddFile(meta); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot save metadata")
			return
		}
		writeJSON(w, http.StatusCreated, toFileResponse(meta))
	}
}

func handleGetFile(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		f, err := store.GetFileForUser(r.PathValue("id"), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeJSON(w, http.StatusOK, toFileResponse(f))
	}
}

func handleDownloadFile(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		f, err := store.GetFileForUser(r.PathValue("id"), userID)
		if err != nil {
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

func handlePreviewFile(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		f, err := store.GetFileForUser(r.PathValue("id"), userID)
		if err != nil {
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
		ct := f.ContentType
		if f.FileType == "code" {
			ct = "text/plain; charset=utf-8"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", safeFilename(f.Name)))
		http.ServeContent(w, r, f.Name, f.UpdatedAt, fh)
	}
}

func handleDeleteFile(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		item, err := store.TrashFile(r.PathValue("id"), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "trashId": item.ID})
	}
}

func handleMoveFile(store *Store) http.HandlerFunc {
	type req struct {
		FolderID string `json:"folderID"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		f, err := store.MoveFile(r.PathValue("id"), userID, q.FolderID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid folder")
			return
		}
		writeJSON(w, http.StatusOK, toFileResponse(f))
	}
}

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

// ================= CHUNKED UPLOAD =================

type chunkSession struct {
	mu          sync.Mutex
	userID      string
	folderID    string
	name        string
	contentType string
	total       int
	got         map[int]bool
	dir         string
}

var chunkSessions sync.Map

func handleChunkInit(store *Store) http.HandlerFunc {
	type req struct {
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
		TotalChunks int    `json:"totalChunks"`
		FolderID    string `json:"folderID"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if q.TotalChunks <= 0 || q.TotalChunks > 100000 {
			writeError(w, http.StatusBadRequest, "invalid totalChunks")
			return
		}
		name := safeFilename(q.Filename)
		if name == "download" {
			name = "file-" + randomHex(8)
		}
		ct := q.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		sid := randomHex(16)
		dir := filepath.Join(store.dataDir, "chunks", sid)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot create session dir")
			return
		}
		chunkSessions.Store(sid, &chunkSession{
			userID: userID, folderID: q.FolderID, name: name, contentType: ct,
			total: q.TotalChunks, got: make(map[int]bool), dir: dir,
		})
		writeJSON(w, http.StatusCreated, map[string]any{"sessionId": sid, "totalChunks": q.TotalChunks})
	}
}

func handleChunkPart(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		sid := r.URL.Query().Get("sessionId")
		idx, err := strconv.Atoi(r.URL.Query().Get("index"))
		if err != nil || idx < 0 {
			writeError(w, http.StatusBadRequest, "invalid index")
			return
		}
		v, ok := chunkSessions.Load(sid)
		if !ok {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		cs := v.(*chunkSession)
		if cs.userID != userID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if idx >= cs.total {
			writeError(w, http.StatusBadRequest, "index out of range")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
		tmp, err := os.CreateTemp(cs.dir, "tmp-*")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot write chunk")
			return
		}
		if _, err := io.Copy(tmp, r.Body); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			writeError(w, http.StatusBadRequest, "cannot read chunk body")
			return
		}
		tmp.Close()
		if err := os.Rename(tmp.Name(), filepath.Join(cs.dir, fmt.Sprintf("part_%d", idx))); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot store chunk")
			return
		}

		cs.mu.Lock()
		cs.got[idx] = true
		n := len(cs.got)
		cs.mu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{"index": idx, "received": n, "total": cs.total})
	}
}

func handleChunkComplete(store *Store) http.HandlerFunc {
	type req struct {
		SessionID string `json:"sessionId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		v, ok := chunkSessions.Load(q.SessionID)
		if !ok {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		cs := v.(*chunkSession)
		if cs.userID != userID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		cs.mu.Lock()
		missing := make([]int, 0)
		for i := 0; i < cs.total; i++ {
			if !cs.got[i] {
				missing = append(missing, i)
			}
		}
		cs.mu.Unlock()
		if len(missing) > 0 {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":   map[string]any{"code": "conflict", "message": "not all chunks uploaded"},
				"missing": missing,
			})
			return
		}

		readers := make([]io.Reader, 0, cs.total)
		opened := make([]*os.File, 0, cs.total)
		for i := 0; i < cs.total; i++ {
			fh, err := os.Open(filepath.Join(cs.dir, fmt.Sprintf("part_%d", i)))
			if err != nil {
				for _, f := range opened {
					f.Close()
				}
				writeError(w, http.StatusInternalServerError, "chunk missing on disk")
				return
			}
			opened = append(opened, fh)
			readers = append(readers, fh)
		}
		hash, size, err := store.saveBlob(io.MultiReader(readers...))
		for _, f := range opened {
			f.Close()
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot save blob")
			return
		}

		name := store.uniqueIfTaken(cs.userID, cs.folderID, cs.name)
		now := time.Now().UTC()
		meta := FileMeta{
			ID: randomHex(16), UserID: cs.userID, Name: name, Size: size,
			ContentType: cs.contentType, FileType: determineFileType(cs.contentType, name),
			StorageKey: hash, FolderID: cs.folderID, CreatedAt: now, UpdatedAt: now,
		}
		if err := store.AddFile(meta); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot save metadata")
			return
		}

		chunkSessions.Delete(q.SessionID)
		os.RemoveAll(cs.dir)
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

package main

import (
    "bytes"
    "fmt"
    "io"
    "log"
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

        // ЧИТАЕМ ФАЙЛ В ПАМЯТЬ
        fileData, err := io.ReadAll(file)
        if err != nil {
            writeError(w, http.StatusInternalServerError, "cannot read file")
            return
        }
        size := int64(len(fileData))

        // НОВАЯ ЛОГИКА: если файл больше 10 МБ — режем на чанки
        const CHUNK_SIZE = 5 * 1024 * 1024 // 5 МБ
        var storageKey string

        if size > 10*1024*1024 {
            // БОЛЬШОЙ ФАЙЛ — сохраняем через чанки
            chunksDir := filepath.Join(store.dataDir, "chunks")
            os.MkdirAll(chunksDir, 0750)

            fileID := randomHex(16)
            chunkDir := filepath.Join(chunksDir, fileID)
            os.MkdirAll(chunkDir, 0750)

            totalChunks := (len(fileData) + CHUNK_SIZE - 1) / CHUNK_SIZE
            for i := 0; i < totalChunks; i++ {
                start := i * CHUNK_SIZE
                end := start + CHUNK_SIZE
                if end > len(fileData) {
                    end = len(fileData)
                }
                chunkPath := filepath.Join(chunkDir, fmt.Sprintf("part_%d", i))
                os.WriteFile(chunkPath, fileData[start:end], 0644)
            }
            storageKey = "chunked:" + fileID
            log.Printf("✅ Файл %s разбит на %d чанков", header.Filename, totalChunks)
        } else {
            // МАЛЕНЬКИЙ ФАЙЛ — обычное сохранение
            hash, _, err := store.saveBlob(bytes.NewReader(fileData))
            if err != nil {
                writeError(w, http.StatusInternalServerError, "cannot save blob")
                return
            }
            storageKey = hash
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
            StorageKey: storageKey, FolderID: folderID, CreatedAt: now, UpdatedAt: now,
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

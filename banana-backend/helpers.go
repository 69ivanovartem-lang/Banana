package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h != "" && strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return r.URL.Query().Get("token")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	code := "error"
	switch status {
	case http.StatusBadRequest:
		code = "bad_request"
	case http.StatusUnauthorized:
		code = "unauthorized"
	case http.StatusForbidden:
		code = "forbidden"
	case http.StatusNotFound:
		code = "not_found"
	case http.StatusConflict:
		code = "conflict"
	case http.StatusRequestEntityTooLarge:
		code = "payload_too_large"
	default:
		code = "internal_error"
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

func toFileResponse(f FileMeta) fileResponse {
	return fileResponse{
		ID: f.ID, Name: f.Name, Size: f.Size, ContentType: f.ContentType,
		FileType: f.FileType, FolderID: f.FolderID, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
		DownloadURL: "/api/v1/files/" + f.ID + "/download",
		PreviewURL:  "/api/v1/files/" + f.ID + "/preview",
	}
}

func toFolderResponse(f Folder) folderResponse {
	return folderResponse{ID: f.ID, Name: f.Name, ParentID: f.ParentID, Size: f.Size, FileCount: f.FileCount, CreatedAt: f.CreatedAt}
}

func toShareLinkResponse(l ShareLink) shareLinkResponse {
	return shareLinkResponse{ID: l.ID, ResourceType: l.ResourceType, ResourceID: l.ResourceID, Token: l.Token, URL: "/api/v1/share/" + l.Token, CreatedAt: l.CreatedAt}
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func itoa(n int) string { return strconv.Itoa(n) }

func hashPassword(password, salt string) string {
	data := []byte(salt + ":" + password)
	for i := 0; i < 50000; i++ {
		h := sha256.Sum256(data)
		data = h[:]
	}
	return hex.EncodeToString(data)
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func validEmail(e string) bool {
	e = normalizeEmail(e)
	return strings.Contains(e, "@") && strings.Contains(e, ".")
}

func safeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\\", " ")
	name = strings.ReplaceAll(name, "/", " ")
	name = strings.ReplaceAll(name, "\n", " ")
	name = strings.ReplaceAll(name, "\r", " ")
	if name == "" || name == "." || name == ".." {
		name = "download"
	}
	return name
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func getEnvInt64(k string, d int64) int64 {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return d
}

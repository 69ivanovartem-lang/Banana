package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ===== Ошибки =====

var (
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrNotFound           = errors.New("not found")
	ErrInvalidParent      = errors.New("invalid parent folder")
)

// ===== Модели =====

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"passwordHash"`
	Salt         string    `json:"salt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Folder struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userID"`
	Name      string    `json:"name"`
	ParentID  string    `json:"parentID"`
	CreatedAt time.Time `json:"createdAt"`
}

type FileMeta struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userID"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	StorageKey  string    `json:"storageKey"`
	FolderID    string    `json:"folderID"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ShareLink struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userID"`
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceID"`
	Token        string    `json:"token"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userID"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ===== База и Store =====

type DB struct {
	Users      map[string]User      `json:"users"`
	Files      map[string]FileMeta  `json:"files"`
	Folders    map[string]Folder    `json:"folders"`
	ShareLinks map[string]ShareLink `json:"shareLinks"`
	EmailIndex map[string]string    `json:"emailIndex"`
}

type Store struct {
	mu       sync.RWMutex
	db       DB
	sessions map[string]Session
	dataDir  string
	dbPath   string
}

// ===== Ответы фронтенду =====

type fileResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	FolderID    string    `json:"folderID"`
	CreatedAt   time.Time `json:"createdAt"`
	DownloadURL string    `json:"downloadUrl"`
}

type folderResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ParentID  string    `json:"parentID"`
	CreatedAt time.Time `json:"createdAt"`
}

type shareLinkResponse struct {
	ID           string    `json:"id"`
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceID"`
	Token        string    `json:"token"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"createdAt"`
}

type folderItemsResponse struct {
	Files   []fileResponse   `json:"files"`
	Folders []folderResponse `json:"folders"`
}

// ===== main =====

func main() {
	dataDir := getEnv("BANANA_DATA_DIR", "./data")
	port := getEnv("BANANA_PORT", "8080")
	maxUploadBytes := getEnvInt64("BANANA_MAX_UPLOAD_SIZE", 100<<20)

	store, err := NewStore(dataDir)
	if err != nil {
		slog.Error("failed to init store", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Авторизация
	mux.HandleFunc("POST /api/v1/auth/register", handleRegister(store))
	mux.HandleFunc("POST /api/v1/auth/login", handleLogin(store))
	mux.HandleFunc("POST /api/v1/auth/logout", handleLogout(store))

	// Папки
	mux.Handle("POST /api/v1/folders", requireAuth(store, handleCreateFolder(store)))
	mux.Handle("GET /api/v1/folders", requireAuth(store, handleListFolders(store)))
	mux.Handle("GET /api/v1/folders/{id}", requireAuth(store, handleGetFolder(store)))
	mux.Handle("GET /api/v1/folders/{id}/items", requireAuth(store, handleGetFolderItems(store)))
	mux.Handle("PATCH /api/v1/folders/{id}", requireAuth(store, handleRenameFolder(store)))
	mux.Handle("DELETE /api/v1/folders/{id}", requireAuth(store, handleDeleteFolder(store)))

	// Файлы
	mux.Handle("GET /api/v1/files", requireAuth(store, handleListFiles(store)))
	mux.Handle("POST /api/v1/files", requireAuth(store, handleUploadFile(store, maxUploadBytes)))
	mux.Handle("GET /api/v1/files/{id}", requireAuth(store, handleGetFile(store)))
	mux.Handle("GET /api/v1/files/{id}/download", requireAuth(store, handleDownloadFile(store)))
	mux.Handle("DELETE /api/v1/files/{id}", requireAuth(store, handleDeleteFile(store)))
	mux.Handle("PATCH /api/v1/files/{id}/move", requireAuth(store, handleMoveFile(store)))

	// Общий доступ
	mux.Handle("POST /api/v1/share", requireAuth(store, handleCreateShareLink(store)))
	mux.Handle("GET /api/v1/share/{token}", handleAccessSharedResource(store))
	mux.Handle("GET /api/v1/share/{token}/download", handleDownloadSharedFile(store))
	mux.Handle("DELETE /api/v1/share/{id}", requireAuth(store, handleDeleteShareLink(store)))

	handler := logMiddleware(corsMiddleware(mux))

	slog.Info("Banana API started", "addr", ":"+port, "dataDir", dataDir)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// ===== Store =====

func NewStore(dataDir string) (*Store, error) {
	filesDir := filepath.Join(dataDir, "files")
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		return nil, err
	}

	s := &Store{
		db: DB{
			Users:      make(map[string]User),
			Files:      make(map[string]FileMeta),
			Folders:    make(map[string]Folder),
			ShareLinks: make(map[string]ShareLink),
			EmailIndex: make(map[string]string),
		},
		sessions: make(map[string]Session),
		dataDir:  dataDir,
		dbPath:   filepath.Join(dataDir, "db.json"),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	if s.db.Users == nil {
		s.db.Users = make(map[string]User)
	}
	if s.db.Files == nil {
		s.db.Files = make(map[string]FileMeta)
	}
	if s.db.Folders == nil {
		s.db.Folders = make(map[string]Folder)
	}
	if s.db.ShareLinks == nil {
		s.db.ShareLinks = make(map[string]ShareLink)
	}
	if s.db.EmailIndex == nil {
		s.db.EmailIndex = make(map[string]string)
	}

	return s, nil
}

func (s *Store) filesDir() string {
	return filepath.Join(s.dataDir, "files")
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.dbPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.db)
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.dbPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.dbPath)
}

// ===== Пользователи =====

func (s *Store) CreateUser(email, password string) (User, error) {
	email = normalizeEmail(email)
	salt := randomHex(16)
	passwordHash := hashPassword(password, salt)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.db.EmailIndex[email]; exists {
		return User{}, ErrEmailExists
	}

	user := User{
		ID:           randomHex(16),
		Email:        email,
		PasswordHash: passwordHash,
		Salt:         salt,
		CreatedAt:    time.Now().UTC(),
	}

	s.db.Users[user.ID] = user
	s.db.EmailIndex[email] = user.ID

	if err := s.saveLocked(); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) Authenticate(email, password string) (User, error) {
	email = normalizeEmail(email)

	s.mu.RLock()
	userID, ok := s.db.EmailIndex[email]
	var user User
	if ok {
		user = s.db.Users[userID]
	}
	s.mu.RUnlock()

	if !ok {
		return User{}, ErrInvalidCredentials
	}

	passwordHash := hashPassword(password, user.Salt)
	if subtle.ConstantTimeCompare([]byte(passwordHash), []byte(user.PasswordHash)) == 0 {
		return User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Store) CreateSession(userID string, ttl time.Duration) Session {
	token := randomHex(32)
	session := Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}
	s.mu.Lock()
	s.sessions[token] = session
	s.mu.Unlock()
	return session
}

func (s *Store) UserIDByToken(token string) (string, bool) {
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()

	if !ok {
		return "", false
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		return "", false
	}
	return session.UserID, true
}

func (s *Store) DeleteSession(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// ===== Папки =====

func (s *Store) CreateFolder(userID, name, parentID string) (Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if parentID != "" {
		parent, ok := s.db.Folders[parentID]
		if !ok {
			return Folder{}, ErrInvalidParent
		}
		if parent.UserID != userID {
			return Folder{}, ErrInvalidParent
		}
	}

	folder := Folder{
		ID:        randomHex(16),
		UserID:    userID,
		Name:      name,
		ParentID:  parentID,
		CreatedAt: time.Now().UTC(),
	}

	s.db.Folders[folder.ID] = folder

	if err := s.saveLocked(); err != nil {
		return Folder{}, err
	}
	return folder, nil
}

func (s *Store) ListFolders(userID, parentID string) []Folder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	folders := make([]Folder, 0, len(s.db.Folders))
	for _, folder := range s.db.Folders {
		if folder.UserID == userID && folder.ParentID == parentID {
			folders = append(folders, folder)
		}
	}

	sort.Slice(folders, func(i, j int) bool {
		return folders[i].Name < folders[j].Name
	})
	return folders
}

func (s *Store) GetFolderForUser(folderID, userID string) (Folder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	folder, ok := s.db.Folders[folderID]
	if !ok {
		return Folder{}, ErrNotFound
	}
	if folder.UserID != userID {
		return Folder{}, ErrNotFound
	}
	return folder, nil
}

func (s *Store) RenameFolder(folderID, userID, newName string) (Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	folder, ok := s.db.Folders[folderID]
	if !ok {
		return Folder{}, ErrNotFound
	}
	if folder.UserID != userID {
		return Folder{}, ErrNotFound
	}

	folder.Name = newName
	s.db.Folders[folderID] = folder

	if err := s.saveLocked(); err != nil {
		return Folder{}, err
	}
	return folder, nil
}

func (s *Store) DeleteFolder(folderID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	folder, ok := s.db.Folders[folderID]
	if !ok {
		return ErrNotFound
	}
	if folder.UserID != userID {
		return ErrNotFound
	}

	// Удаляем файлы в папке
	for id, file := range s.db.Files {
		if file.FolderID == folderID {
			_ = os.Remove(filepath.Join(s.filesDir(), file.StorageKey))
			delete(s.db.Files, id)
		}
	}

	// Удаляем подпапки
	for id, subfolder := range s.db.Folders {
		if subfolder.ParentID == folderID {
			delete(s.db.Folders, id)
		}
	}

	delete(s.db.Folders, folderID)

	return s.saveLocked()
}

func (s *Store) GetFolderItems(userID, folderID string) folderItemsResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files := make([]fileResponse, 0)
	folders := make([]folderResponse, 0)

	for _, file := range s.db.Files {
		if file.UserID == userID && file.FolderID == folderID {
			files = append(files, toFileResponse(file))
		}
	}

	for _, folder := range s.db.Folders {
		if folder.UserID == userID && folder.ParentID == folderID {
			folders = append(folders, toFolderResponse(folder))
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].CreatedAt.After(files[j].CreatedAt)
	})
	sort.Slice(folders, func(i, j int) bool {
		return folders[i].Name < folders[j].Name
	})

	return folderItemsResponse{Files: files, Folders: folders}
}

// ===== Файлы =====

func (s *Store) AddFile(file FileMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Files[file.ID] = file
	return s.saveLocked()
}

func (s *Store) ListFiles(userID, folderID string) []FileMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files := make([]FileMeta, 0, len(s.db.Files))
	for _, file := range s.db.Files {
		if file.UserID == userID && file.FolderID == folderID {
			files = append(files, file)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].CreatedAt.After(files[j].CreatedAt)
	})
	return files
}

func (s *Store) GetFileForUser(fileID, userID string) (FileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, ok := s.db.Files[fileID]
	if !ok {
		return FileMeta{}, ErrNotFound
	}
	if file.UserID != userID {
		return FileMeta{}, ErrNotFound
	}
	return file, nil
}

func (s *Store) GetFile(fileID string) (FileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, ok := s.db.Files[fileID]
	if !ok {
		return FileMeta{}, ErrNotFound
	}
	return file, nil
}

func (s *Store) DeleteFile(fileID, userID string) (FileMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, ok := s.db.Files[fileID]
	if !ok {
		return FileMeta{}, ErrNotFound
	}
	if file.UserID != userID {
		return FileMeta{}, ErrNotFound
	}

	delete(s.db.Files, fileID)
	if err := s.saveLocked(); err != nil {
		return FileMeta{}, err
	}
	return file, nil
}

func (s *Store) MoveFile(fileID, userID, newFolderID string) (FileMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, ok := s.db.Files[fileID]
	if !ok {
		return FileMeta{}, ErrNotFound
	}
	if file.UserID != userID {
		return FileMeta{}, ErrNotFound
	}

	if newFolderID != "" {
		folder, ok := s.db.Folders[newFolderID]
		if !ok {
			return FileMeta{}, ErrInvalidParent
		}
		if folder.UserID != userID {
			return FileMeta{}, ErrInvalidParent
		}
	}

	file.FolderID = newFolderID
	s.db.Files[fileID] = file

	if err := s.saveLocked(); err != nil {
		return FileMeta{}, err
	}
	return file, nil
}

// ===== Общий доступ =====

func (s *Store) CreateShareLink(userID, resourceType, resourceID string) (ShareLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if resourceType == "file" {
		file, ok := s.db.Files[resourceID]
		if !ok || file.UserID != userID {
			return ShareLink{}, ErrNotFound
		}
	} else if resourceType == "folder" {
		folder, ok := s.db.Folders[resourceID]
		if !ok || folder.UserID != userID {
			return ShareLink{}, ErrNotFound
		}
	} else {
		return ShareLink{}, errors.New("invalid resource type")
	}

	shareLink := ShareLink{
		ID:           randomHex(16),
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Token:        randomHex(32),
		CreatedAt:    time.Now().UTC(),
	}

	s.db.ShareLinks[shareLink.ID] = shareLink

	if err := s.saveLocked(); err != nil {
		return ShareLink{}, err
	}
	return shareLink, nil
}

func (s *Store) GetShareLink(token string) (ShareLink, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, link := range s.db.ShareLinks {
		if link.Token == token {
			return link, true
		}
	}
	return ShareLink{}, false
}

func (s *Store) DeleteShareLink(linkID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, ok := s.db.ShareLinks[linkID]
	if !ok {
		return ErrNotFound
	}
	if link.UserID != userID {
		return ErrNotFound
	}

	delete(s.db.ShareLinks, linkID)
	return s.saveLocked()
}

// ===== Handlers: auth =====

func handleRegister(store *Store) http.HandlerFunc {
	type req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if !validEmail(q.Email) {
			writeError(w, http.StatusBadRequest, "invalid email")
			return
		}
		if len(q.Password) < 6 {
			writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}
		user, err := store.CreateUser(q.Email, q.Password)
		if err != nil {
			if errors.Is(err, ErrEmailExists) {
				writeError(w, http.StatusConflict, "email already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, "cannot create user")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": user.ID, "email": user.Email, "createdAt": user.CreatedAt,
		})
	}
}

func handleLogin(store *Store) http.HandlerFunc {
	type req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var q req
		if err := readJSON(r, &q); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		user, err := store.Authenticate(q.Email, q.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		session := store.CreateSession(user.ID, 24*time.Hour)
		writeJSON(w, http.StatusOK, map[string]any{
			"token": session.Token, "expiresAt": session.ExpiresAt,
		})
	}
}

func handleLogout(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token != "" {
			store.DeleteSession(token)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// ===== Handlers: folders =====

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
		folder, err := store.CreateFolder(userID, q.Name, q.ParentID)
		if err != nil {
			if errors.Is(err, ErrInvalidParent) {
				writeError(w, http.StatusBadRequest, "invalid parent folder")
				return
			}
			writeError(w, http.StatusInternalServerError, "cannot create folder")
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
			items = append(items, toFolderResponse(f))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	}
}

func handleGetFolder(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.PathValue("id")
		folder, err := store.GetFolderForUser(folderID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}
		writeJSON(w, http.StatusOK, toFolderResponse(folder))
	}
}

func handleGetFolderItems(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.PathValue("id")
		items := store.GetFolderItems(userID, folderID)
		writeJSON(w, http.StatusOK, items)
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
		folder, err := store.RenameFolder(folderID, userID, q.Name)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}
		writeJSON(w, http.StatusOK, toFolderResponse(folder))
	}
}

func handleDeleteFolder(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.PathValue("id")
		err := store.DeleteFolder(folderID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "folder not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	}
}

// ===== Handlers: files =====

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

func handleUploadFile(store *Store, maxUploadBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		folderID := r.URL.Query().Get("folderID")

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "file too large")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid multipart form")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file is required, use field name 'file'")
			return
		}
		defer file.Close()

		fileID := randomHex(16)
		storageKey := fileID
		destPath := filepath.Join(store.filesDir(), storageKey)

		out, err := os.Create(destPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot create file")
			return
		}

		size, err := io.Copy(out, file)
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(destPath)
			writeError(w, http.StatusBadRequest, "cannot write uploaded file")
			return
		}
		if size == 0 {
			os.Remove(destPath)
			writeError(w, http.StatusBadRequest, "empty file")
			return
		}

		name := safeFilename(header.Filename)
		if name == "download" && strings.TrimSpace(header.Filename) == "" {
			name = "file-" + fileID
		}

		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		meta := FileMeta{
			ID: fileID, UserID: userID, Name: name, Size: size,
			ContentType: contentType, StorageKey: storageKey,
			FolderID: folderID, CreatedAt: time.Now().UTC(),
		}

		if err := store.AddFile(meta); err != nil {
			os.Remove(destPath)
			writeError(w, http.StatusInternalServerError, "cannot save file metadata")
			return
		}

		writeJSON(w, http.StatusCreated, toFileResponse(meta))
	}
}

func handleGetFile(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		fileID := r.PathValue("id")
		file, err := store.GetFileForUser(fileID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeJSON(w, http.StatusOK, toFileResponse(file))
	}
}

func handleDownloadFile(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		fileID := r.PathValue("id")
		file, err := store.GetFileForUser(fileID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		path := filepath.Join(store.filesDir(), file.StorageKey)
		f, err := os.Open(path)
		if err != nil {
			writeError(w, http.StatusNotFound, "file data not found")
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", file.ContentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFilename(file.Name)))
		http.ServeContent(w, r, file.Name, file.CreatedAt, f)
	}
}

func handleDeleteFile(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		fileID := r.PathValue("id")
		file, err := store.DeleteFile(fileID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		_ = os.Remove(filepath.Join(store.filesDir(), file.StorageKey))
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	}
}

func handleMoveFile(store *Store) http.HandlerFunc {
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
		file, err := store.MoveFile(fileID, userID, q.FolderID)
		if err != nil {
			if errors.Is(err, ErrInvalidParent) {
				writeError(w, http.StatusBadRequest, "invalid folder")
				return
			}
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeJSON(w, http.StatusOK, toFileResponse(file))
	}
}

// ===== Handlers: share =====

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
		token := r.PathValue("token")
		link, ok := store.GetShareLink(token)
		if !ok {
			writeError(w, http.StatusNotFound, "share link not found")
			return
		}

		if link.ResourceType == "file" {
			file, err := store.GetFile(link.ResourceID)
			if err != nil {
				writeError(w, http.StatusNotFound, "file not found")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"resourceType": "file",
				"file":         toFileResponse(file),
			})
		} else {
			folder, err := store.GetFolderForUser(link.ResourceID, link.UserID)
			if err != nil {
				writeError(w, http.StatusNotFound, "folder not found")
				return
			}
			items := store.GetFolderItems(link.UserID, link.ResourceID)
			writeJSON(w, http.StatusOK, map[string]any{
				"resourceType": "folder",
				"folder":       toFolderResponse(folder),
				"items":        items,
			})
		}
	}
}

// Публичное скачивание файла по share-ссылке (без авторизации)
func handleDownloadSharedFile(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareToken := r.PathValue("token")

		link, ok := store.GetShareLink(shareToken)
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
		} else {
			// ссылка на папку: качаем только файлы именно из этой папки
			if fileID == "" {
				writeError(w, http.StatusBadRequest, "fileID is required for folder share")
				return
			}
		}

		file, err := store.GetFile(fileID)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}

		if link.ResourceType == "folder" && file.FolderID != link.ResourceID {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}

		path := filepath.Join(store.filesDir(), file.StorageKey)
		f, err := os.Open(path)
		if err != nil {
			writeError(w, http.StatusNotFound, "file data not found")
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", file.ContentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFilename(file.Name)))
		http.ServeContent(w, r, file.Name, file.CreatedAt, f)
	}
}

func handleDeleteShareLink(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		linkID := r.PathValue("id")
		err := store.DeleteShareLink(linkID, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "share link not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	}
}

// ===== Middleware =====

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigin := os.Getenv("BANANA_FRONTEND_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
		)
	})
}

type contextKey string

const userIDContextKey contextKey = "userID"

func requireAuth(store *Store, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "authorization required")
			return
		}
		userID, ok := store.UserIDByToken(token)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next(w, r.WithContext(ctx))
	}
}

func userIDFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(userIDContextKey).(string); ok {
		return value
	}
	return ""
}

// ===== Helpers =====

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
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
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"code": code, "message": message},
	})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return decoder.Decode(v)
}

func toFileResponse(file FileMeta) fileResponse {
	return fileResponse{
		ID:          file.ID,
		Name:        file.Name,
		Size:        file.Size,
		ContentType: file.ContentType,
		FolderID:    file.FolderID,
		CreatedAt:   file.CreatedAt,
		DownloadURL: "/api/v1/files/" + file.ID + "/download",
	}
}

func toFolderResponse(folder Folder) folderResponse {
	return folderResponse{
		ID:        folder.ID,
		Name:      folder.Name,
		ParentID:  folder.ParentID,
		CreatedAt: folder.CreatedAt,
	}
}

func toShareLinkResponse(link ShareLink) shareLinkResponse {
	return shareLinkResponse{
		ID:           link.ID,
		ResourceType: link.ResourceType,
		ResourceID:   link.ResourceID,
		Token:        link.Token,
		URL:          "/api/v1/share/" + link.Token,
		CreatedAt:    link.CreatedAt,
	}
}

func randomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func hashPassword(password, salt string) string {
	data := []byte(salt + ":" + password)
	for i := 0; i < 50_000; i++ {
		h := sha256.Sum256(data)
		data = h[:]
	}
	return hex.EncodeToString(data)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	email = normalizeEmail(email)
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

func safeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\n", " ")
	name = strings.ReplaceAll(name, "\r", " ")
	if name == "" || name == "." || name == ".." {
		name = "download"
	}
	return name
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvInt64(key string, defaultValue int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// ===== Веб-страница =====

func handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("index.html")
	if err != nil {
		http.Error(w, "index.html not found next to main.go", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

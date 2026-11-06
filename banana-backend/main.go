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
	Size      int64     `json:"size"`
	FileCount int       `json:"fileCount"`
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

type TrashItem struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"userID"`
	Type               string     `json:"type"` // "file" или "folder"
	OriginalName       string     `json:"originalName"`
	OriginalFolder     string     `json:"originalFolder"`
	OriginalFolderName string     `json:"originalFolderName"`
	StorageKey         string     `json:"storageKey"`  // legacy
	ContentType        string     `json:"contentType"` // legacy
	Size               int64      `json:"size"`
	FileCount          int        `json:"fileCount"`
	FolderCount        int        `json:"folderCount"`
	SnapshotFile       *FileMeta  `json:"snapshotFile,omitempty"`
	SnapshotFolders    []Folder   `json:"snapshotFolders,omitempty"`
	SnapshotFiles      []FileMeta `json:"snapshotFiles,omitempty"`
	DeletedAt          time.Time  `json:"deletedAt"`
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

type DB struct {
	Users      map[string]User      `json:"users"`
	Files      map[string]FileMeta  `json:"files"`
	Folders    map[string]Folder    `json:"folders"`
	Trash      map[string]TrashItem `json:"trash"`
	ShareLinks map[string]ShareLink `json:"shareLinks"`
	EmailIndex map[string]string    `json:"emailIndex"`
	Sessions   map[string]Session   `json:"sessions"`
}

type Store struct {
	mu      sync.RWMutex
	db      DB
	dataDir string
	dbPath  string
}

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
	Size      int64     `json:"size"`
	FileCount int       `json:"fileCount"`
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

// ===== РУЧНОЙ РОУТЕР =====
type router struct {
	hRegister      http.HandlerFunc
	hLogin         http.HandlerFunc
	hLogout        http.HandlerFunc
	hCreateFolder  http.HandlerFunc
	hListFolders   http.HandlerFunc
	hGetFolder     http.HandlerFunc
	hFolderItems   http.HandlerFunc
	hRenameFolder  http.HandlerFunc
	hDeleteFolder  http.HandlerFunc
	hListFiles     http.HandlerFunc
	hUploadFile    http.HandlerFunc
	hGetFile       http.HandlerFunc
	hDownloadFile  http.HandlerFunc
	hDeleteFile    http.HandlerFunc
	hMoveFile      http.HandlerFunc
	hListTrash     http.HandlerFunc
	hRestoreTrash  http.HandlerFunc
	hDeleteTrash   http.HandlerFunc
	hCreateShare   http.HandlerFunc
	hAccessShare   http.HandlerFunc
	hShareDownload http.HandlerFunc
	hDeleteShare   http.HandlerFunc
}

func newRouter(s *Store, maxUpload int64) *router {
	rt := &router{}
	auth := func(h http.HandlerFunc) http.HandlerFunc { return requireAuth(s, h) }
	rt.hRegister = handleRegister(s)
	rt.hLogin = handleLogin(s)
	rt.hLogout = handleLogout(s)
	rt.hCreateFolder = auth(handleCreateFolder(s))
	rt.hListFolders = auth(handleListFolders(s))
	rt.hGetFolder = auth(handleGetFolder(s))
	rt.hFolderItems = auth(handleGetFolderItems(s))
	rt.hRenameFolder = auth(handleRenameFolder(s))
	rt.hDeleteFolder = auth(handleDeleteFolder(s))
	rt.hListFiles = auth(handleListFiles(s))
	rt.hUploadFile = auth(handleUploadFile(s, maxUpload))
	rt.hGetFile = auth(handleGetFile(s))
	rt.hDownloadFile = auth(handleDownloadFile(s))
	rt.hDeleteFile = auth(handleDeleteFile(s))
	rt.hMoveFile = auth(handleMoveFile(s))
	rt.hListTrash = auth(handleListTrash(s))
	rt.hRestoreTrash = auth(handleRestoreTrash(s))
	rt.hDeleteTrash = auth(handleDeleteTrashPermanent(s))
	rt.hCreateShare = auth(handleCreateShareLink(s))
	rt.hAccessShare = handleAccessSharedResource(s)
	rt.hShareDownload = handleDownloadSharedFile(s)
	rt.hDeleteShare = auth(handleDeleteShareLink(s))
	return rt
}

func (rt *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimSuffix(r.URL.Path, "/")
	if p == "" {
		p = "/"
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	m := r.Method

	switch {
	case p == "/" && m == "GET":
		handleIndex(w, r)
	case p == "/favicon.ico":
		w.WriteHeader(http.StatusNoContent)
	case p == "/healthz" && m == "GET":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case p == "/api/v1/auth/register" && m == "POST":
		rt.hRegister(w, r)
	case p == "/api/v1/auth/login" && m == "POST":
		rt.hLogin(w, r)
	case p == "/api/v1/auth/logout" && m == "POST":
		rt.hLogout(w, r)
	case p == "/api/v1/folders" && m == "POST":
		rt.hCreateFolder(w, r)
	case p == "/api/v1/folders" && m == "GET":
		rt.hListFolders(w, r)
	case p == "/api/v1/files" && m == "GET":
		rt.hListFiles(w, r)
	case p == "/api/v1/files" && m == "POST":
		rt.hUploadFile(w, r)
	case p == "/api/v1/trash" && m == "GET":
		rt.hListTrash(w, r)
	case p == "/api/v1/share" && m == "POST":
		rt.hCreateShare(w, r)
	case len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "folders":
		r.SetPathValue("id", parts[3])
		switch m {
		case "GET":
			rt.hGetFolder(w, r)
		case "PATCH":
			rt.hRenameFolder(w, r)
		case "DELETE":
			rt.hDeleteFolder(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case len(parts) == 5 && parts[2] == "folders" && parts[4] == "items" && m == "GET":
		r.SetPathValue("id", parts[3])
		rt.hFolderItems(w, r)
	case len(parts) == 4 && parts[2] == "files":
		r.SetPathValue("id", parts[3])
		switch m {
		case "GET":
			rt.hGetFile(w, r)
		case "DELETE":
			rt.hDeleteFile(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case len(parts) == 5 && parts[2] == "files" && parts[4] == "download" && m == "GET":
		r.SetPathValue("id", parts[3])
		rt.hDownloadFile(w, r)
	case len(parts) == 5 && parts[2] == "files" && parts[4] == "move" && m == "PATCH":
		r.SetPathValue("id", parts[3])
		rt.hMoveFile(w, r)
	case len(parts) == 4 && parts[2] == "trash" && m == "DELETE":
		r.SetPathValue("id", parts[3])
		rt.hDeleteTrash(w, r)
	case len(parts) == 5 && parts[2] == "trash" && parts[4] == "restore" && m == "POST":
		r.SetPathValue("id", parts[3])
		rt.hRestoreTrash(w, r)
	case len(parts) == 4 && parts[2] == "share" && m == "GET":
		r.SetPathValue("token", parts[3])
		rt.hAccessShare(w, r)
	case len(parts) == 4 && parts[2] == "share" && m == "DELETE":
		r.SetPathValue("id", parts[3])
		rt.hDeleteShare(w, r)
	case len(parts) == 5 && parts[2] == "share" && parts[4] == "download" && m == "GET":
		r.SetPathValue("token", parts[3])
		rt.hShareDownload(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found: "+m+" "+p)
	}
}

func main() {
	dataDir := getEnv("BANANA_DATA_DIR", "./data")
	port := getEnv("BANANA_PORT", "8080")
	maxUploadBytes := getEnvInt64("BANANA_MAX_UPLOAD_SIZE", 100<<20)

	store, err := NewStore(dataDir)
	if err != nil {
		slog.Error("failed to init store", "error", err)
		os.Exit(1)
	}

	rt := newRouter(store, maxUploadBytes)
	handler := logMiddleware(corsMiddleware(rt))

	slog.Info("Banana API started (manual router + trash)", "addr", ":"+port, "dataDir", dataDir)
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
			Trash:      make(map[string]TrashItem),
			ShareLinks: make(map[string]ShareLink),
			EmailIndex: make(map[string]string),
			Sessions:   make(map[string]Session),
		},
		dataDir: dataDir,
		dbPath:  filepath.Join(dataDir, "db.json"),
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
	if s.db.Trash == nil {
		s.db.Trash = make(map[string]TrashItem)
	}
	if s.db.ShareLinks == nil {
		s.db.ShareLinks = make(map[string]ShareLink)
	}
	if s.db.EmailIndex == nil {
		s.db.EmailIndex = make(map[string]string)
	}
	if s.db.Sessions == nil {
		s.db.Sessions = make(map[string]Session)
	}
	return s, nil
}

func (s *Store) filesDir() string { return filepath.Join(s.dataDir, "files") }

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

// ===== Пользователи и сессии (сессии в db.json) =====
func (s *Store) CreateUser(email, password string) (User, error) {
	email = normalizeEmail(email)
	salt := randomHex(16)
	passwordHash := hashPassword(password, salt)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.db.EmailIndex[email]; exists {
		return User{}, ErrEmailExists
	}
	user := User{ID: randomHex(16), Email: email, PasswordHash: passwordHash, Salt: salt, CreatedAt: time.Now().UTC()}
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
	session := Session{Token: token, UserID: userID, ExpiresAt: time.Now().UTC().Add(ttl)}
	s.mu.Lock()
	s.db.Sessions[token] = session
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		slog.Error("failed to save session", "error", err)
	}
	return session
}

func (s *Store) UserIDByToken(token string) (string, bool) {
	s.mu.RLock()
	session, ok := s.db.Sessions[token]
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
	delete(s.db.Sessions, token)
	_ = s.saveLocked()
	s.mu.Unlock()
}

// ===== Папки =====
func (s *Store) CreateFolder(userID, name, parentID string) (Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if parentID != "" {
		parent, ok := s.db.Folders[parentID]
		if !ok || parent.UserID != userID {
			return Folder{}, ErrInvalidParent
		}
	}
	folder := Folder{ID: randomHex(16), UserID: userID, Name: name, ParentID: parentID, CreatedAt: time.Now().UTC()}
	s.db.Folders[folder.ID] = folder
	if err := s.saveLocked(); err != nil {
		return Folder{}, err
	}
	return folder, nil
}

func (s *Store) ListFolders(userID, parentID string) []Folder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	folders := make([]Folder, 0)
	for _, folder := range s.db.Folders {
		if folder.UserID == userID && folder.ParentID == parentID {
			folders = append(folders, folder)
		}
	}
	sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })
	return folders
}

func (s *Store) GetFolderForUser(folderID, userID string) (Folder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	folder, ok := s.db.Folders[folderID]
	if !ok || folder.UserID != userID {
		return Folder{}, ErrNotFound
	}
	return folder, nil
}

func (s *Store) RenameFolder(folderID, userID, newName string) (Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	folder, ok := s.db.Folders[folderID]
	if !ok || folder.UserID != userID {
		return Folder{}, ErrNotFound
	}
	folder.Name = newName
	s.db.Folders[folderID] = folder
	if err := s.saveLocked(); err != nil {
		return Folder{}, err
	}
	return folder, nil
}

// DeleteFolder: папка и ВСЁ её содержимое уходят в корзину ОДНИМ слепком
func (s *Store) DeleteFolder(folderID, userID string) (TrashItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	folder, ok := s.db.Folders[folderID]
	if !ok || folder.UserID != userID {
		return TrashItem{}, ErrNotFound
	}
	var folderIDs []string
	var collect func(fid string)
	collect = func(fid string) {
		folderIDs = append(folderIDs, fid)
		for _, sub := range s.db.Folders {
			if sub.ParentID == fid {
				collect(sub.ID)
			}
		}
	}
	collect(folderID)
	inSet := map[string]bool{}
	for _, id := range folderIDs {
		inSet[id] = true
	}
	snapFolders := make([]Folder, 0, len(folderIDs))
	snapFolders = append(snapFolders, folder)
	for _, id := range folderIDs {
		if id != folderID {
			snapFolders = append(snapFolders, s.db.Folders[id])
		}
	}
	snapFiles := make([]FileMeta, 0)
	var totalSize int64
	for _, f := range s.db.Files {
		if inSet[f.FolderID] {
			snapFiles = append(snapFiles, f)
			totalSize += f.Size
		}
	}
	item := TrashItem{
		ID: "t" + randomHex(15), UserID: userID, Type: "folder",
		OriginalName: folder.Name, OriginalFolder: folder.ParentID,
		Size: totalSize, FileCount: len(snapFiles), FolderCount: len(snapFolders) - 1,
		SnapshotFolders: snapFolders, SnapshotFiles: snapFiles,
		DeletedAt: time.Now().UTC(),
	}
	if folder.ParentID != "" {
		if fo, ok2 := s.db.Folders[folder.ParentID]; ok2 {
			item.OriginalFolderName = fo.Name
		}
	}
	for _, id := range folderIDs {
		delete(s.db.Folders, id)
	}
	for _, f := range snapFiles {
		delete(s.db.Files, f.ID)
	}
	s.db.Trash[item.ID] = item
	if err := s.saveLocked(); err != nil {
		return TrashItem{}, err
	}
	return item, nil
}

func (s *Store) countRecursive(folderID string, size *int64, count *int) {
	for _, f := range s.db.Files {
		if f.FolderID == folderID {
			*size += f.Size
			*count++
		}
	}
	for _, sub := range s.db.Folders {
		if sub.ParentID == folderID {
			s.countRecursive(sub.ID, size, count)
		}
	}
}

func (s *Store) folderStats(folderID string) (int64, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sz int64
	var ct int
	s.countRecursive(folderID, &sz, &ct)
	return sz, ct
}

func (s *Store) GetFolderItems(userID, folderID string) ([]fileResponse, []folderResponse) {
	s.mu.RLock()
	files := make([]fileResponse, 0)
	var folderObjs []Folder
	for _, f := range s.db.Files {
		if f.UserID == userID && f.FolderID == folderID {
			files = append(files, toFileResponse(f))
		}
	}
	for _, fo := range s.db.Folders {
		if fo.UserID == userID && fo.ParentID == folderID {
			folderObjs = append(folderObjs, fo)
		}
	}
	s.mu.RUnlock()
	folders := make([]folderResponse, 0, len(folderObjs))
	for _, fo := range folderObjs {
		fr := toFolderResponse(fo)
		fr.Size, fr.FileCount = s.folderStats(fo.ID)
		folders = append(folders, fr)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].CreatedAt.After(files[j].CreatedAt) })
	sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })
	return files, folders
}

// ===== Дубликаты имён =====
func (s *Store) nameExists(userID, parentID, name, excludeID string) bool {
	for _, fo := range s.db.Folders {
		if fo.UserID == userID && fo.ParentID == parentID && fo.Name == name && fo.ID != excludeID {
			return true
		}
	}
	for _, f := range s.db.Files {
		if f.UserID == userID && f.FolderID == parentID && f.Name == name && f.ID != excludeID {
			return true
		}
	}
	return false
}

func (s *Store) CheckDuplicateName(userID, parentID, name, excludeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nameExists(userID, parentID, strings.TrimSpace(name), excludeID)
}

func (s *Store) uniqueNameLocked(userID, parentID, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i <= 1000; i++ {
		cand := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if !s.nameExists(userID, parentID, cand, "") {
			return cand
		}
	}
	return fmt.Sprintf("%s_%d%s", base, time.Now().Unix(), ext)
}

func (s *Store) GenerateUniqueName(userID, parentID, name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.nameExists(userID, parentID, name, "") {
		return name
	}
	return s.uniqueNameLocked(userID, parentID, name)
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
	files := make([]FileMeta, 0)
	for _, f := range s.db.Files {
		if f.UserID == userID && f.FolderID == folderID {
			files = append(files, f)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].CreatedAt.After(files[j].CreatedAt) })
	return files
}

func (s *Store) GetFileForUser(fileID, userID string) (FileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.db.Files[fileID]
	if !ok || f.UserID != userID {
		return FileMeta{}, ErrNotFound
	}
	return f, nil
}

func (s *Store) GetFile(fileID string) (FileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.db.Files[fileID]
	if !ok {
		return FileMeta{}, ErrNotFound
	}
	return f, nil
}

func (s *Store) MoveFile(fileID, userID, newFolderID string) (FileMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.db.Files[fileID]
	if !ok || f.UserID != userID {
		return FileMeta{}, ErrNotFound
	}
	if newFolderID != "" {
		fo, ok := s.db.Folders[newFolderID]
		if !ok || fo.UserID != userID {
			return FileMeta{}, ErrInvalidParent
		}
	}
	f.FolderID = newFolderID
	s.db.Files[fileID] = f
	if err := s.saveLocked(); err != nil {
		return FileMeta{}, err
	}
	return f, nil
}

// ===== КОРЗИНА =====
func (s *Store) TrashFile(fileID, userID string) (TrashItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.db.Files[fileID]
	if !ok || f.UserID != userID {
		return TrashItem{}, ErrNotFound
	}
	item := TrashItem{
		ID: "t" + randomHex(15), UserID: userID, Type: "file",
		OriginalName: f.Name, OriginalFolder: f.FolderID,
		Size: f.Size, FileCount: 1,
		SnapshotFile: &f,
		DeletedAt:    time.Now().UTC(),
	}
	if f.FolderID != "" {
		if fo, ok2 := s.db.Folders[f.FolderID]; ok2 {
			item.OriginalFolderName = fo.Name
		}
	}
	delete(s.db.Files, f.ID)
	s.db.Trash[item.ID] = item
	if err := s.saveLocked(); err != nil {
		return TrashItem{}, err
	}
	return item, nil
}

func (s *Store) ListTrash(userID string) []TrashItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]TrashItem, 0)
	for _, it := range s.db.Trash {
		if it.UserID == userID {
			items = append(items, it)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DeletedAt.After(items[j].DeletedAt) })
	return items
}

// RestoreTrash: восстанавливает слепок ЦЕЛИКОМ (папка + вложенные папки + файлы на своих местах)
func (s *Store) RestoreTrash(trashID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.db.Trash[trashID]
	if !ok || it.UserID != userID {
		return ErrNotFound
	}
	if it.Type == "folder" && len(it.SnapshotFolders) > 0 {
		root := it.SnapshotFolders[0]
		parentID := root.ParentID
		if parentID != "" {
			if fo, ok2 := s.db.Folders[parentID]; !ok2 || fo.UserID != userID {
				parentID = ""
			}
		}
		name := root.Name
		if s.nameExists(userID, parentID, name, "") {
			name = s.uniqueNameLocked(userID, parentID, name)
		}
		idMap := map[string]string{}
		for _, f := range it.SnapshotFolders {
			newID := f.ID
			if _, occupied := s.db.Folders[newID]; occupied {
				newID = randomHex(16)
			}
			idMap[f.ID] = newID
		}
		for i, f := range it.SnapshotFolders {
			nf := f
			nf.UserID = userID
			nf.ID = idMap[f.ID]
			if i == 0 {
				nf.ParentID = parentID
				nf.Name = name
			} else if mapped, ok2 := idMap[f.ParentID]; ok2 {
				nf.ParentID = mapped
			}
			s.db.Folders[nf.ID] = nf
		}
		for _, f := range it.SnapshotFiles {
			nf := f
			nf.UserID = userID
			if _, occupied := s.db.Files[nf.ID]; occupied {
				nf.ID = randomHex(16)
			}
			if mapped, ok2 := idMap[f.FolderID]; ok2 {
				nf.FolderID = mapped
			}
			s.db.Files[nf.ID] = nf
		}
	} else {
		var f FileMeta
		if it.SnapshotFile != nil {
			f = *it.SnapshotFile
		} else {
			f = FileMeta{Name: it.OriginalName, Size: it.Size, ContentType: it.ContentType, StorageKey: it.StorageKey, CreatedAt: it.DeletedAt}
		}
		folderID := it.OriginalFolder
		if folderID != "" {
			if fo, ok2 := s.db.Folders[folderID]; !ok2 || fo.UserID != userID {
				folderID = ""
			}
		}
		f.ID = randomHex(16)
		f.UserID = userID
		f.FolderID = folderID
		if s.nameExists(userID, folderID, f.Name, "") {
			f.Name = s.uniqueNameLocked(userID, folderID, f.Name)
		}
		s.db.Files[f.ID] = f
	}
	delete(s.db.Trash, trashID)
	return s.saveLocked()
}

func (s *Store) DeleteTrashPermanent(trashID, userID string) (TrashItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.db.Trash[trashID]
	if !ok || it.UserID != userID {
		return TrashItem{}, ErrNotFound
	}
	delete(s.db.Trash, trashID)
	if err := s.saveLocked(); err != nil {
		return TrashItem{}, err
	}
	return it, nil
}

// ===== Share =====
func (s *Store) CreateShareLink(userID, resourceType, resourceID string) (ShareLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if resourceType == "file" {
		f, ok := s.db.Files[resourceID]
		if !ok || f.UserID != userID {
			return ShareLink{}, ErrNotFound
		}
	} else if resourceType == "folder" {
		fo, ok := s.db.Folders[resourceID]
		if !ok || fo.UserID != userID {
			return ShareLink{}, ErrNotFound
		}
	} else {
		return ShareLink{}, errors.New("invalid resource type")
	}
	link := ShareLink{ID: randomHex(16), UserID: userID, ResourceType: resourceType, ResourceID: resourceID, Token: randomHex(32), CreatedAt: time.Now().UTC()}
	s.db.ShareLinks[link.ID] = link
	if err := s.saveLocked(); err != nil {
		return ShareLink{}, err
	}
	return link, nil
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
	if !ok || link.UserID != userID {
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
		writeJSON(w, http.StatusCreated, map[string]any{"id": user.ID, "email": user.Email, "createdAt": user.CreatedAt})
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
		writeJSON(w, http.StatusOK, map[string]any{"token": session.Token, "expiresAt": session.ExpiresAt})
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
		folder, err := store.GetFolderForUser(r.PathValue("id"), userID)
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
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "file too large")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file is required, use field name 'file'")
			return
		}
		defer file.Close()
		fileID := randomHex(16)
		destPath := filepath.Join(store.filesDir(), fileID)
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
		original := safeFilename(header.Filename)
		if original == "download" {
			original = "file-" + fileID
		}
		uniqueName := store.GenerateUniqueName(userID, folderID, original)
		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		meta := FileMeta{ID: fileID, UserID: userID, Name: uniqueName, Size: size, ContentType: contentType, StorageKey: fileID, FolderID: folderID, CreatedAt: time.Now().UTC()}
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
		path := filepath.Join(store.filesDir(), f.StorageKey)
		fh, err := os.Open(path)
		if err != nil {
			writeError(w, http.StatusNotFound, "file data not found")
			return
		}
		defer fh.Close()
		w.Header().Set("Content-Type", f.ContentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFilename(f.Name)))
		http.ServeContent(w, r, f.Name, f.CreatedAt, fh)
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

// ===== Handlers: trash =====
func handleListTrash(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		items := store.ListTrash(userID)
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	}
}

func handleRestoreTrash(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		if err := store.RestoreTrash(r.PathValue("id"), userID); err != nil {
			writeError(w, http.StatusNotFound, "trash item not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"restored": true})
	}
}

func handleDeleteTrashPermanent(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		it, err := store.DeleteTrashPermanent(r.PathValue("id"), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "trash item not found")
			return
		}
		if it.Type == "folder" {
			for _, f := range it.SnapshotFiles {
				_ = os.Remove(filepath.Join(store.filesDir(), f.StorageKey))
			}
		} else if it.SnapshotFile != nil {
			_ = os.Remove(filepath.Join(store.filesDir(), it.SnapshotFile.StorageKey))
		} else if it.StorageKey != "" {
			_ = os.Remove(filepath.Join(store.filesDir(), it.StorageKey))
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
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
		writeJSON(w, http.StatusOK, map[string]any{
			"resourceType": "folder",
			"folder":       toFolderResponse(folder),
			"items":        map[string]any{"files": files, "folders": folders},
		})
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
		path := filepath.Join(store.filesDir(), f.StorageKey)
		fh, err := os.Open(path)
		if err != nil {
			writeError(w, http.StatusNotFound, "file data not found")
			return
		}
		defer fh.Close()
		w.Header().Set("Content-Type", f.ContentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFilename(f.Name)))
		http.ServeContent(w, r, f.Name, f.CreatedAt, fh)
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
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
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
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return decoder.Decode(v)
}

func toFileResponse(f FileMeta) fileResponse {
	return fileResponse{ID: f.ID, Name: f.Name, Size: f.Size, ContentType: f.ContentType, FolderID: f.FolderID, CreatedAt: f.CreatedAt, DownloadURL: "/api/v1/files/" + f.ID + "/download"}
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

func hashPassword(password, salt string) string {
	data := []byte(salt + ":" + password)
	for i := 0; i < 50000; i++ {
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
	name = strings.ReplaceAll(name, "\\", " ")
	name = strings.ReplaceAll(name, "/", " ")
	name = strings.ReplaceAll(name, "\n", " ")
	name = strings.ReplaceAll(name, "\r", " ")
	if name == "" || name == "." || name == ".." {
		name = "download"
	}
	return name
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return defaultValue
}

// ===== Веб-страница =====
func handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("index.html")
	if err == nil && strings.Contains(string(data), "<script") && strings.Contains(string(data), "trash-btn") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(fallbackHTML))
}

const fallbackHTML = `<!DOCTYPE html>
<html lang="ru"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Banana Cloud Storage</title>
<style>
:root{--bg:#0f1115;--bg2:#161a22;--bg3:#1d222d;--bd:#2a3140;--tx:#e6e8ee;--mu:#8a93a6;--pr:#ffd23f;--er:#ef4444;--ok:#22c55e;--dh:200px}
*{box-sizing:border-box;margin:0;padding:0}
html,body{height:100%}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:var(--bg);color:var(--tx);font-size:14px;overflow:hidden}
button,input,select{font-family:inherit;font-size:14px}
a{color:var(--pr);text-decoration:none}
.hidden{display:none!important}
.wrap{min-height:100vh;display:flex;align-items:center;justify-content:center;padding:20px}
.card{background:var(--bg2);border:1px solid var(--bd);border-radius:14px;padding:28px;width:100%;max-width:420px}
.card.wide{max-width:1100px}
.logo{font-size:26px;font-weight:700;margin-bottom:6px}
.logo span{color:var(--pr)}
.mu{color:var(--mu)}
.tabs{display:flex;gap:4px;margin:18px 0;background:var(--bg3);padding:4px;border-radius:10px}
.tab{flex:1;padding:9px;text-align:center;border-radius:8px;cursor:pointer;color:var(--mu);font-weight:500}
.tab.active{background:var(--bg2);color:var(--tx)}
.field{margin-bottom:12px}
.field label{display:block;margin-bottom:5px;color:var(--mu);font-size:11px;text-transform:uppercase;letter-spacing:.5px}
.field input,.field select{width:100%;padding:10px 12px;background:var(--bg3);border:1px solid var(--bd);border-radius:8px;color:var(--tx);outline:none}
.field input:focus{border-color:var(--pr)}
.btn{display:inline-flex;align-items:center;gap:6px;padding:9px 14px;border-radius:8px;border:1px solid transparent;cursor:pointer;font-weight:500;background:var(--bg3);color:var(--tx)}
.btn:hover{background:var(--bd)}
.btn.primary{background:var(--pr);color:#1a1a1a}
.btn.danger{background:transparent;border-color:var(--er);color:var(--er)}
.btn.small{padding:5px 10px;font-size:12px}
.btn.block{width:100%;justify-content:center}
.err{color:var(--er);font-size:13px;margin-top:8px;min-height:18px}
.app{display:flex;height:100vh;flex-direction:column}
header{background:var(--bg2);border-bottom:1px solid var(--bd);padding:12px 24px;display:flex;align-items:center;gap:14px}
header .logo{margin:0;font-size:19px}
.sp{flex:1}
.toolbar{padding:14px 24px;display:flex;gap:8px;align-items:center;flex-wrap:wrap;border-bottom:1px solid var(--bd)}
.crumbs{display:flex;gap:6px;flex-wrap:wrap;align-items:center}
.crumb{color:var(--mu);cursor:pointer;padding:4px 8px;border-radius:6px}
.crumb:hover{background:var(--bg3);color:var(--tx)}
.crumb.current{color:var(--tx);cursor:default}
main{flex:1;padding:20px 24px;overflow:auto;min-height:0}
table{width:100%;border-collapse:collapse}
th,td{text-align:left;padding:11px 12px;border-bottom:1px solid var(--bd)}
th{color:var(--mu);font-weight:500;font-size:11px;text-transform:uppercase;letter-spacing:.5px;position:sticky;top:0;background:var(--bg)}
tr:hover td{background:var(--bg2)}
.click td:first-child{cursor:pointer}
.acts{display:flex;gap:5px;flex-wrap:wrap}
.empty{text-align:center;padding:50px 20px;color:var(--mu)}
.mbg{position:fixed;inset:0;background:rgba(0,0,0,.6);display:flex;align-items:center;justify-content:center;z-index:100;padding:20px}
.macts{display:flex;justify-content:flex-end;gap:8px;margin-top:18px}
.toast{position:fixed;bottom:16px;right:16px;background:var(--bg3);border:1px solid var(--bd);border-radius:10px;padding:11px 16px;z-index:300;opacity:0;transform:translateY(8px);transition:.2s;max-width:340px}
.toast.on{opacity:1;transform:translateY(0)}
.toast.error{border-color:var(--er)}
.toast.success{border-color:var(--ok)}
.linkbox{background:var(--bg3);border:1px solid var(--bd);border-radius:8px;padding:9px 10px;display:flex;gap:6px;align-items:center}
.linkbox input{flex:1;background:transparent;border:none;color:var(--tx);outline:none;font-family:monospace;font-size:12px}
.dbg{position:fixed;bottom:0;left:0;right:0;height:var(--dh);background:#0a0c0f;border-top:2px solid var(--pr);display:flex;flex-direction:column;z-index:200}
.dbg-h{padding:7px 12px;background:var(--bg2);border-bottom:1px solid var(--bd);display:flex;justify-content:space-between;align-items:center;cursor:ns-resize;user-select:none}
.dbg-h b{color:var(--pr);font-size:12px}
.dbg-b{padding:6px;overflow-y:auto;font-family:Consolas,Monaco,monospace;font-size:11px;flex:1;min-height:0}
.dl{padding:3px 7px;margin:2px 0;border-left:3px solid var(--bd);background:var(--bg2);word-break:break-word}
.dl.request{border-left-color:#3b82f6}.dl.response{border-left-color:var(--ok)}.dl.error{border-left-color:var(--er)}.dl.info{border-left-color:var(--pr)}
.dbgt{position:fixed;bottom:14px;right:14px;z-index:199;background:var(--pr);color:#1a1a1a;border:none;border-radius:50%;width:42px;height:42px;font-size:18px;cursor:pointer;display:none;align-items:center;justify-content:center}
.dbgt.on{display:flex}
</style></head><body>

<div id="auth" class="wrap">
 <div class="card">
  <div class="logo">🍌 Banana <span>Cloud</span></div>
  <div class="mu">Приватное облачное хранилище</div>
  <div class="tabs"><div class="tab active" id="tab-login">Вход</div><div class="tab" id="tab-reg">Регистрация</div></div>
  <form id="login-form">
   <div class="field"><label>Email</label><input type="email" name="email" required></div>
   <div class="field"><label>Пароль</label><input type="password" name="password" required minlength="6"></div>
   <div class="err" id="login-err"></div>
   <button class="btn primary block" type="submit">Войти</button>
  </form>
  <form id="reg-form" class="hidden">
   <div class="field"><label>Email</label><input type="email" name="email" required></div>
   <div class="field"><label>Пароль (мин. 6)</label><input type="password" name="password" required minlength="6"></div>
   <div class="err" id="reg-err"></div>
   <button class="btn primary block" type="submit">Создать аккаунт</button>
  </form>
 </div>
</div>

<div id="app" class="app hidden">
 <header>
  <div class="logo">🍌 Banana <span>Cloud</span></div>
  <div class="sp"></div>
  <span class="mu" id="who"></span>
  <button class="btn small" id="logout">Выйти</button>
 </header>
 <div class="toolbar">
  <div class="crumbs" id="crumbs"></div>
  <div class="sp"></div>
  <button class="btn small" id="refresh">🔄 Обновить</button>
  <button class="btn small" id="newfolder">📁 Новая папка</button>
  <button class="btn small primary" id="upload">⬆️ Загрузить</button>
  <button class="btn small" id="trash-btn">🗑 Корзина</button>
  <input type="file" id="file" multiple class="hidden">
 </div>
 <main>
  <table><thead><tr><th>Имя</th><th>Тип</th><th>Размер</th><th>Создано</th><th>Действия</th></tr></thead>
  <tbody id="rows"></tbody></table>
  <div id="empty" class="empty hidden">Папка пуста</div>
 </main>
</div>

<div id="shareview" class="wrap hidden">
 <div class="card wide">
  <div class="logo">🍌 Banana <span>Cloud</span> — общий доступ</div>
  <div id="sharebox" style="margin-top:14px">Загрузка...</div>
 </div>
</div>

<div class="mbg hidden" id="modal-bg"><div class="card" id="modal"></div></div>
<div class="toast" id="toast"></div>
<button class="dbgt" id="dbg-toggle">🔍</button>
<div class="dbg" id="dbg">
 <div class="dbg-h" id="dbg-head"><b>🔍 Debug Log</b>
  <span><button class="btn small" id="dbg-clear">🗑 Очистить</button> <button class="btn small danger" id="dbg-close">✕ Закрыть</button></span>
 </div>
 <div class="dbg-b" id="dbg-body"></div>
</div>

<script>
var state={token:localStorage.getItem('banana_token'),email:localStorage.getItem('banana_email'),cur:'',path:[],view:'files'};
function $(s){return document.querySelector(s);}
function $$(s){return document.querySelectorAll(s);}
function esc(s){return String(s).replace(/[&<>"']/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c];});}
function fmt(b){if(!b&&b!==0)return '—';var u=['B','KB','MB','GB','TB'],i=0,n=b;while(n>=1024&&i<4){n/=1024;i++;}return n.toFixed(n<10&&i>0?1:0)+' '+u[i];}
function fmtDate(s){if(!s)return '—';return new Date(s).toLocaleString('ru-RU',{day:'2-digit',month:'2-digit',year:'numeric',hour:'2-digit',minute:'2-digit'});}
function icon(ct){if(!ct)return '📄';if(ct.indexOf('image/')===0)return '🖼️';if(ct.indexOf('video/')===0)return '🎬';if(ct.indexOf('audio/')===0)return '🎵';if(ct.indexOf('pdf')>=0)return '📕';if(ct.indexOf('zip')>=0||ct.indexOf('rar')>=0)return '🗜️';if(ct.indexOf('text')>=0||ct.indexOf('json')>=0)return '📝';return '📄';}
var toastT;function toast(m,t){var e=$('#toast');e.textContent=m;e.className='toast on '+(t||'');clearTimeout(toastT);toastT=setTimeout(function(){e.classList.remove('on');},3000);}
var dbgState=JSON.parse(localStorage.getItem('debug_state')||'{}');
if(dbgState.hidden){$('#dbg').classList.add('hidden');$('#dbg-toggle').classList.add('on');}
if(dbgState.height)document.documentElement.style.setProperty('--dh',dbgState.height+'px');
function saveDbg(){localStorage.setItem('debug_state',JSON.stringify({hidden:$('#dbg').classList.contains('hidden'),height:$('#dbg').offsetHeight}));}
$('#dbg-close').onclick=function(){$('#dbg').classList.add('hidden');$('#dbg-toggle').classList.add('on');saveDbg();};
$('#dbg-toggle').onclick=function(){$('#dbg').classList.remove('hidden');$('#dbg-toggle').classList.remove('on');saveDbg();};
$('#dbg-clear').onclick=function(){$('#dbg-body').innerHTML='';};
var rz=false,ry=0,rh=0;
$('#dbg-head').addEventListener('mousedown',function(e){rz=true;ry=e.clientY;rh=$('#dbg').offsetHeight;e.preventDefault();});
document.addEventListener('mousemove',function(e){if(!rz)return;var h=Math.max(80,Math.min(rh+(ry-e.clientY),window.innerHeight-100));document.documentElement.style.setProperty('--dh',h+'px');});
document.addEventListener('mouseup',function(){if(rz){rz=false;saveDbg();}});
function dlog(t,m,d){var e=document.createElement('div');e.className='dl '+t;var h='<span class="mu">['+new Date().toLocaleTimeString('ru-RU')+']</span> '+esc(m);
if(d!==null&&d!==undefined){var s=JSON.stringify(d);h+='<div class="mu" style="font-size:10px;white-space:pre-wrap">'+esc(s.length>300?s.slice(0,300)+'…':s)+'</div>';}
e.innerHTML=h;var b=$('#dbg-body');b.appendChild(e);b.scrollTop=b.scrollHeight;console.log(t,m,d||'');}
function api(p,o){o=o||{};var h=o.headers||{};if(state.token)h['Authorization']='Bearer '+state.token;
var body=o.body;
if(o.body&&!(o.body instanceof FormData)){h['Content-Type']='application/json';body=JSON.stringify(o.body);}
dlog('request',(o.method||'GET')+' '+p,o.body instanceof FormData?{formData:true}:o.body);
return fetch(p,{method:o.method||'GET',headers:h,body:body}).then(function(r){return r.text().then(function(tx){var d;try{d=JSON.parse(tx);}catch(e){d=tx;}
dlog('response',r.status+' '+p,d);
if(!r.ok){var msg=(d&&d.error&&d.error.message)||('HTTP '+r.status);throw new Error(msg);}return d;});});}
$$('#tab-login,#tab-reg').forEach(function(t){t.onclick=function(){$$('.tab').forEach(function(x){x.classList.remove('active');});t.classList.add('active');
$('#login-form').classList.toggle('hidden',t.id!=='tab-login');$('#reg-form').classList.toggle('hidden',t.id!=='tab-reg');};});
$('#login-form').onsubmit=function(e){e.preventDefault();var fd=new FormData(e.target);$('#login-err').textContent='';
api('/api/v1/auth/login',{method:'POST',body:{email:fd.get('email').toString().trim(),password:fd.get('password').toString()}}).then(function(d){
state.token=d.token;state.email=fd.get('email').toString().trim();localStorage.setItem('banana_token',d.token);localStorage.setItem('banana_email',state.email);showApp();
}).catch(function(er){$('#login-err').textContent=er.message;});};
$('#reg-form').onsubmit=function(e){e.preventDefault();var fd=new FormData(e.target);$('#reg-err').textContent='';
api('/api/v1/auth/register',{method:'POST',body:{email:fd.get('email').toString().trim(),password:fd.get('password').toString()}}).then(function(){
toast('Аккаунт создан. Войдите.','success');$('#tab-login').click();}).catch(function(er){$('#reg-err').textContent=er.message;});};
$('#logout').onclick=function(){api('/api/v1/auth/logout',{method:'POST'}).catch(function(){});state.token=null;state.email=null;
localStorage.removeItem('banana_token');localStorage.removeItem('banana_email');showAuth();};
function showAuth(){$('#auth').classList.remove('hidden');$('#app').classList.add('hidden');$('#shareview').classList.add('hidden');}
function showApp(){$('#auth').classList.add('hidden');$('#app').classList.remove('hidden');$('#who').textContent=state.email||'';state.cur='';state.path=[];state.view='files';loadFolder();}
function crumbs(){var bc=$('#crumbs');bc.innerHTML='';
if(state.view==='trash'){var b=document.createElement('span');b.className='crumb';b.textContent='← Мои файлы';b.onclick=function(){backToFiles();};bc.appendChild(b);
var s1=document.createElement('span');s1.className='mu';s1.textContent='›';bc.appendChild(s1);
var c=document.createElement('span');c.className='crumb current';c.textContent='🗑 Корзина';bc.appendChild(c);return;}
var r=document.createElement('span');r.className='crumb'+(state.path.length===0?' current':'');r.textContent='🏠 Мои файлы';
r.onclick=function(){state.cur='';state.path=[];loadFolder();};bc.appendChild(r);
state.path.forEach(function(p,i){var s=document.createElement('span');s.className='mu';s.textContent='›';bc.appendChild(s);
var c=document.createElement('span');c.className='crumb'+(i===state.path.length-1?' current':'');c.textContent=p.name;
c.onclick=function(){state.path=state.path.slice(0,i+1);state.cur=p.id;loadFolder();};bc.appendChild(c);});}
function loadFolder(){state.view='files';$('#empty').textContent='Папка пуста';crumbs();
var tb=$('#rows');tb.innerHTML='<tr><td colspan="5" class="empty">Загрузка...</td></tr>';$('#empty').classList.add('hidden');tb.onclick=null;
Promise.all([api('/api/v1/folders?parentID='+encodeURIComponent(state.cur)),api('/api/v1/files?folderID='+encodeURIComponent(state.cur))])
.then(function(res){render(res[0].items||[],res[1].items||[]);})
.catch(function(er){tb.innerHTML='<tr><td colspan="5" class="empty">Ошибка: '+esc(er.message)+'</td></tr>';});}
function render(folders,files){var tb=$('#rows');tb.innerHTML='';
if(!folders.length&&!files.length){$('#empty').classList.remove('hidden');return;}
$('#empty').classList.add('hidden');
folders.forEach(function(f){var tr=document.createElement('tr');tr.className='click';
tr.innerHTML='<td>📁 '+esc(f.name)+'</td><td>Папка</td><td>'+fmt(f.size||0)+' ('+(f.fileCount||0)+' файл.)</td><td>'+fmtDate(f.createdAt)+'</td>'+
'<td class="acts"><button class="btn small" data-a="ren" data-id="'+f.id+'" data-n="'+esc(f.name)+'">✏️</button>'+
'<button class="btn small" data-a="shf" data-id="'+f.id+'">🔗</button>'+
'<button class="btn small danger" data-a="delf" data-id="'+f.id+'">🗑</button></td>';
tr.onclick=function(e){if(e.target.closest('button,a'))return;state.cur=f.id;state.path.push({id:f.id,name:f.name});loadFolder();};
tb.appendChild(tr);});
files.forEach(function(f){var tr=document.createElement('tr');
tr.innerHTML='<td>'+icon(f.contentType)+' '+esc(f.name)+'</td><td>'+esc(f.contentType||'file')+'</td><td>'+fmt(f.size)+'</td><td>'+fmtDate(f.createdAt)+'</td>'+
'<td class="acts"><a class="btn small" href="'+(f.downloadUrl||'/api/v1/files/'+f.id+'/download')+'" download>⬇️</a>'+
'<button class="btn small" data-a="mov" data-id="'+f.id+'">📦</button>'+
'<button class="btn small" data-a="shx" data-id="'+f.id+'">🔗</button>'+
'<button class="btn small danger" data-a="delx" data-id="'+f.id+'">🗑</button></td>';tb.appendChild(tr);});
tb.onclick=function(e){var b=e.target.closest('button');if(!b)return;var id=b.dataset.id,a=b.dataset.a;
if(a==='ren')renFolder(id,b.dataset.n);
if(a==='delf')delFolder(id);
if(a==='delx')delFile(id);
if(a==='mov')moveFile(id);
if(a==='shx')share('file',id);
if(a==='shf')share('folder',id);};}
$('#refresh').onclick=function(){if(state.view==='trash')loadTrash();else loadFolder();};
$('#newfolder').onclick=function(){modal('<h3>Новая папка</h3><div class="field"><label>Имя</label><input id="m-name" autofocus></div><div class="err" id="m-err"></div><div class="macts"><button class="btn" id="m-cancel">Отмена</button><button class="btn primary" id="m-ok">Создать</button></div>',function(){
var n=$('#m-name').value.trim();if(!n)return;
api('/api/v1/folders',{method:'POST',body:{name:n,parentID:state.cur}}).then(function(){closeModal();toast('Папка создана','success');loadFolder();})
.catch(function(er){if(/exists/i.test(er.message))$('#m-err').textContent='Папка с таким именем уже существует';else toast(er.message,'error');});});};
$('#upload').onclick=function(){$('#file').click();};
$('#file').onchange=function(e){var fs=Array.prototype.slice.call(e.target.files);if(!fs.length)return;var chain=Promise.resolve();
fs.forEach(function(f){chain=chain.then(function(){var fd=new FormData();fd.append('file',f);
return api('/api/v1/files?folderID='+encodeURIComponent(state.cur),{method:'POST',body:fd});});});
chain.then(function(){e.target.value='';toast('Загружено: '+fs.length,'success');loadFolder();})
.catch(function(er){e.target.value='';toast(er.message,'error');});};
function renFolder(id,old){modal('<h3>Переименовать папку</h3><div class="field"><label>Новое имя</label><input id="m-name" value="'+esc(old)+'"></div><div class="macts"><button class="btn" id="m-cancel">Отмена</button><button class="btn primary" id="m-ok">Сохранить</button></div>',function(){
var n=$('#m-name').value.trim();if(!n)return;
api('/api/v1/folders/'+encodeURIComponent(id),{method:'PATCH',body:{name:n}}).then(function(){closeModal();toast('Переименовано','success');loadFolder();}).catch(function(er){toast(er.message,'error');});});}
function delFolder(id){if(!confirm('Переместить папку и всё её содержимое в КОРЗИНУ?'))return;
api('/api/v1/folders/'+encodeURIComponent(id),{method:'DELETE'}).then(function(){toast('Папка в корзине','success');loadFolder();}).catch(function(er){toast(er.message,'error');});}
function delFile(id){if(!confirm('Переместить файл в КОРЗИНУ?'))return;
api('/api/v1/files/'+encodeURIComponent(id),{method:'DELETE'}).then(function(){toast('Файл в корзине','success');loadFolder();}).catch(function(er){toast(er.message,'error');});}
function moveFile(id){var all=[];function walk(pid,d){return api('/api/v1/folders?parentID='+encodeURIComponent(pid)).then(function(r){
var seq=Promise.resolve();(r.items||[]).forEach(function(f){all.push({id:f.id,name:f.name,d:d});seq=seq.then(function(){return walk(f.id,d+1);});});return seq;});}
walk('',0).then(function(){var opts='<option value="">🏠 Корень</option>';all.forEach(function(f){opts+='<option value="'+f.id+'">'+new Array(f.d+1).join('—')+' '+esc(f.name)+'</option>';});
modal('<h3>Куда переместить</h3><div class="field"><label>Папка</label><select id="m-sel">'+opts+'</select></div><div class="macts"><button class="btn" id="m-cancel">Отмена</button><button class="btn primary" id="m-ok">Переместить</button></div>',function(){
api('/api/v1/files/'+encodeURIComponent(id)+'/move',{method:'PATCH',body:{folderID:$('#m-sel').value}}).then(function(){closeModal();toast('Перемещено','success');loadFolder();}).catch(function(er){toast(er.message,'error');});});});}
function share(type,id){api('/api/v1/share',{method:'POST',body:{resourceType:type,resourceID:id}}).then(function(l){
var url=location.origin+'/?share='+l.token;
modal('<h3>Публичная ссылка</h3><p class="mu" style="margin-bottom:10px">Работает БЕЗ входа.</p><div class="linkbox"><input id="m-url" readonly value="'+esc(url)+'"><button class="btn small primary" id="m-copy">Копировать</button></div><div class="macts"><button class="btn danger" id="m-del">Удалить ссылку</button><button class="btn" id="m-cancel">Готово</button></div>',null);
$('#m-copy').onclick=function(){if(navigator.clipboard){navigator.clipboard.writeText(url).then(function(){toast('Скопировано','success');});}else{$('#m-url').select();document.execCommand('copy');}};
$('#m-del').onclick=function(){if(!confirm('Удалить ссылку?'))return;api('/api/v1/share/'+encodeURIComponent(l.id),{method:'DELETE'}).then(function(){closeModal();toast('Ссылка удалена','success');}).catch(function(er){toast(er.message,'error');});};
}).catch(function(er){toast(er.message,'error');});}
$('#trash-btn').onclick=function(){loadTrash();};
function backToFiles(){state.cur='';state.path=[];loadFolder();}
function loadTrash(){state.view='trash';crumbs();var tb=$('#rows');
tb.innerHTML='<tr><td colspan="5" class="empty">Загрузка...</td></tr>';$('#empty').classList.add('hidden');tb.onclick=null;
api('/api/v1/trash').then(function(d){var items=d.items||[];tb.innerHTML='';
if(!items.length){$('#empty').textContent='Корзина пуста';$('#empty').classList.remove('hidden');return;}
$('#empty').classList.add('hidden');
items.forEach(function(it){var tr=document.createElement('tr');
var ic=it.type==='folder'?'📁':'';
var meta=it.type==='folder'?('папка: '+(it.fileCount||0)+' файл., '+(it.folderCount||0)+' влож. папок'):('был в: '+esc(it.originalFolderName||'корень'));
tr.innerHTML='<td>'+ic+' '+esc(it.originalName)+'</td><td>'+meta+'</td><td>'+fmt(it.size)+'</td><td>'+fmtDate(it.deletedAt)+'</td>'+
'<td class="acts"><button class="btn small" data-a="res" data-id="'+it.id+'">♻️ Восстановить</button>'+
'<button class="btn small danger" data-a="pur" data-id="'+it.id+'">❌ Навсегда</button></td>';tb.appendChild(tr);});
tb.onclick=function(e){var b=e.target.closest('button');if(!b)return;var id=b.dataset.id;
if(b.dataset.a==='res'){api('/api/v1/trash/'+id+'/restore',{method:'POST'}).then(function(){toast('Восстановлено','success');loadTrash();}).catch(function(er){toast(er.message,'error');});}
if(b.dataset.a==='pur'){if(!confirm('Удалить НАВСЕГДА? Вернуть будет невозможно.'))return;
api('/api/v1/trash/'+id,{method:'DELETE'}).then(function(){toast('Удалено навсегда','success');loadTrash();}).catch(function(er){toast(er.message,'error');});}};
}).catch(function(er){tb.innerHTML='<tr><td colspan="5" class="empty">Ошибка: '+esc(er.message)+'</td></tr>';});}
function modal(html,onOk){$('#modal').innerHTML=html;$('#modal-bg').classList.remove('hidden');
$('#m-cancel').onclick=closeModal;if(onOk)$('#m-ok').onclick=onOk;}
function closeModal(){$('#modal-bg').classList.add('hidden');}
$('#modal-bg').onclick=function(e){if(e.target.id==='modal-bg')closeModal();};
function viewShare(t){$('#auth').classList.add('hidden');$('#app').classList.add('hidden');$('#shareview').classList.remove('hidden');
api('/api/v1/share/'+encodeURIComponent(t)).then(function(d){var box=$('#sharebox');
if(d.resourceType==='file'){var f=d.file;box.innerHTML='<h3>'+icon(f.contentType)+' '+esc(f.name)+'</h3><p class="mu" style="margin:8px 0">'+fmt(f.size)+'</p><a class="btn primary" href="/api/v1/share/'+encodeURIComponent(t)+'/download" download>⬇️ Скачать</a>';return;}
var h='<h3>📁 '+esc(d.folder.name)+'</h3><table style="margin-top:10px"><thead><tr><th>Имя</th><th>Размер</th><th></th></tr></thead><tbody>';
(d.items.folders||[]).forEach(function(x){h+='<tr><td>📁 '+esc(x.name)+'</td><td>—</td><td></td></tr>';});
(d.items.files||[]).forEach(function(x){h+='<tr><td>'+icon(x.contentType)+' '+esc(x.name)+'</td><td>'+fmt(x.size)+'</td><td><a class="btn small primary" href="/api/v1/share/'+encodeURIComponent(t)+'/download?fileID='+encodeURIComponent(x.id)+'" download>⬇️</a></td></tr>';});
box.innerHTML=h+'</tbody></table>';}).catch(function(er){$('#sharebox').innerHTML='<div class="empty">❌ '+esc(er.message)+'</div>';});}
(function(){var q=new URLSearchParams(location.search);var t=q.get('share');
dlog('info','App initialized',{hasToken:!!state.token,share:!!t});
if(t){viewShare(t);return;}
if(state.token)showApp();else showAuth();})();
</script></body></html>`

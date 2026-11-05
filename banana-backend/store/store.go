package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"banana-backend/models"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidParent = errors.New("invalid parent folder")
	ErrEmailExists   = errors.New("email already exists")
)

type DB struct {
	Users      map[string]models.User      `json:"users"`
	Files      map[string]models.FileMeta  `json:"files"`
	Folders    map[string]models.Folder    `json:"folders"`
	Trash      map[string]models.TrashItem `json:"trash"`
	ShareLinks map[string]models.ShareLink `json:"shareLinks"`
	EmailIndex map[string]string           `json:"emailIndex"`
	Sessions   map[string]models.Session   `json:"sessions"`
}

type Store struct {
	mu     sync.RWMutex
	db     DB
	dbPath string
}

func NewStore(dataDir string) (*Store, error) {
	s := &Store{
		db: DB{
			Users:      make(map[string]models.User),
			Files:      make(map[string]models.FileMeta),
			Folders:    make(map[string]models.Folder),
			Trash:      make(map[string]models.TrashItem),
			ShareLinks: make(map[string]models.ShareLink),
			EmailIndex: make(map[string]string),
			Sessions:   make(map[string]models.Session),
		},
		dbPath: filepath.Join(dataDir, "db.json"),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
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
	data, _ := json.MarshalIndent(s.db, "", "  ")
	tmpPath := s.dbPath + ".tmp"
	os.WriteFile(tmpPath, data, 0o600)
	return os.Rename(tmpPath, s.dbPath)
}

func (s *Store) CreateUser(email, password string) (models.User, error) {
	email = normalizeEmail(email)
	salt := randomHex(16)
	passwordHash := hashPassword(password, salt)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.db.EmailIndex[email]; exists {
		return models.User{}, ErrEmailExists
	}

	user := models.User{
		ID:           randomHex(16),
		Email:        email,
		PasswordHash: passwordHash,
		Salt:         salt,
		CreatedAt:    time.Now().UTC(),
	}

	s.db.Users[user.ID] = user
	s.db.EmailIndex[email] = user.ID

	if err := s.saveLocked(); err != nil {
		return models.User{}, err
	}

	return user, nil
}

func (s *Store) Authenticate(email, password string) (models.User, error) {
	email = normalizeEmail(email)

	s.mu.RLock()
	userID, ok := s.db.EmailIndex[email]
	var user models.User
	if ok {
		user = s.db.Users[userID]
	}
	s.mu.RUnlock()

	if !ok {
		return models.User{}, ErrNotFound
	}

	passwordHash := hashPassword(password, user.Salt)
	if passwordHash != user.PasswordHash {
		return models.User{}, ErrNotFound
	}

	return user, nil
}

func (s *Store) CreateSession(userID string, ttl time.Duration) models.Session {
	token := randomHex(32)
	session := models.Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}

	s.mu.Lock()
	s.db.Sessions[token] = session
	s.mu.Unlock()

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
	s.mu.Unlock()
}

func (s *Store) CreateFolder(userID, name, parentID string) (models.Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if parentID != "" {
		parent, ok := s.db.Folders[parentID]
		if !ok {
			return models.Folder{}, ErrInvalidParent
		}
		if parent.UserID != userID {
			return models.Folder{}, ErrInvalidParent
		}
	}

	folder := models.Folder{
		ID:        randomHex(16),
		UserID:    userID,
		Name:      name,
		ParentID:  parentID,
		CreatedAt: time.Now().UTC(),
	}

	s.db.Folders[folder.ID] = folder

	if err := s.saveLocked(); err != nil {
		return models.Folder{}, err
	}

	return folder, nil
}

func (s *Store) ListFolders(userID, parentID string) []models.Folder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	folders := make([]models.Folder, 0)
	for _, folder := range s.db.Folders {
		if folder.UserID == userID && folder.ParentID == parentID {
			folders = append(folders, folder)
		}
	}

	return folders
}

func (s *Store) GetFolderForUser(folderID, userID string) (models.Folder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	folder, ok := s.db.Folders[folderID]
	if !ok {
		return models.Folder{}, ErrNotFound
	}
	if folder.UserID != userID {
		return models.Folder{}, ErrNotFound
	}

	return folder, nil
}

func (s *Store) RenameFolder(folderID, userID, newName string) (models.Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	folder, ok := s.db.Folders[folderID]
	if !ok {
		return models.Folder{}, ErrNotFound
	}
	if folder.UserID != userID {
		return models.Folder{}, ErrNotFound
	}

	folder.Name = newName
	s.db.Folders[folderID] = folder

	if err := s.saveLocked(); err != nil {
		return models.Folder{}, err
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

	for id, file := range s.db.Files {
		if file.FolderID == folderID {
			delete(s.db.Files, id)
		}
	}

	for id, subfolder := range s.db.Folders {
		if subfolder.ParentID == folderID {
			delete(s.db.Folders, id)
		}
	}

	delete(s.db.Folders, folderID)

	return s.saveLocked()
}

func (s *Store) GetFolderItems(userID, folderID string) ([]models.FileMeta, []models.Folder) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var files []models.FileMeta
	var folders []models.Folder

	for _, file := range s.db.Files {
		if file.UserID == userID && file.FolderID == folderID {
			files = append(files, file)
		}
	}

	for _, folder := range s.db.Folders {
		if folder.UserID == userID && folder.ParentID == folderID {
			folders = append(folders, folder)
		}
	}

	return files, folders
}

func (s *Store) AddFile(meta models.FileMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.db.Files[meta.ID]; ok {
		newVersion := models.FileVersion{
			Version:    len(existing.Versions) + 1,
			StorageKey: meta.Versions[0].StorageKey,
			Size:       meta.Size,
			CreatedAt:  time.Now().UTC(),
		}
		meta.Versions = append(existing.Versions, newVersion)
		meta.CreatedAt = existing.CreatedAt
	} else {
		meta.Versions = []models.FileVersion{{
			Version: 1, StorageKey: meta.Versions[0].StorageKey, Size: meta.Size, CreatedAt: meta.CreatedAt,
		}}
	}

	meta.UpdatedAt = time.Now().UTC()
	s.db.Files[meta.ID] = meta

	return s.saveLocked()
}

func (s *Store) ListFiles(userID, folderID string) []models.FileMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var files []models.FileMeta
	for _, file := range s.db.Files {
		if file.UserID == userID && file.FolderID == folderID {
			files = append(files, file)
		}
	}

	return files
}

func (s *Store) GetFileForUser(fileID, userID string) (models.FileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, ok := s.db.Files[fileID]
	if !ok {
		return models.FileMeta{}, ErrNotFound
	}
	if file.UserID != userID {
		return models.FileMeta{}, ErrNotFound
	}

	return file, nil
}

func (s *Store) GetFile(fileID string) (models.FileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, ok := s.db.Files[fileID]
	if !ok {
		return models.FileMeta{}, ErrNotFound
	}

	return file, nil
}

func (s *Store) MoveFile(fileID, userID, newFolderID string) (models.FileMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, ok := s.db.Files[fileID]
	if !ok {
		return models.FileMeta{}, ErrNotFound
	}
	if file.UserID != userID {
		return models.FileMeta{}, ErrNotFound
	}

	if newFolderID != "" {
		folder, ok := s.db.Folders[newFolderID]
		if !ok {
			return models.FileMeta{}, ErrInvalidParent
		}
		if folder.UserID != userID {
			return models.FileMeta{}, ErrInvalidParent
		}
	}

	file.FolderID = newFolderID
	file.UpdatedAt = time.Now().UTC()
	s.db.Files[fileID] = file

	if err := s.saveLocked(); err != nil {
		return models.FileMeta{}, err
	}

	return file, nil
}

func (s *Store) CheckDuplicateName(userID, parentID, name, excludeID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	name = strings.TrimSpace(name)

	for _, folder := range s.db.Folders {
		if folder.UserID == userID && folder.ParentID == parentID && folder.Name == name && folder.ID != excludeID {
			return true, nil
		}
	}

	for _, file := range s.db.Files {
		if file.UserID == userID && file.FolderID == parentID && file.Name == name && file.ID != excludeID {
			return true, nil
		}
	}

	return false, nil
}

func (s *Store) GenerateUniqueName(userID, parentID, name, excludeID string) string {
	isDuplicate, _ := s.CheckDuplicateName(userID, parentID, name, excludeID)
	if !isDuplicate {
		return name
	}

	ext := filepath.Ext(name)
	baseName := strings.TrimSuffix(name, ext)

	for i := 1; i <= 1000; i++ {
		newName := fmt.Sprintf("%s (%d)%s", baseName, i, ext)
		isDuplicate, _ = s.CheckDuplicateName(userID, parentID, newName, excludeID)
		if !isDuplicate {
			return newName
		}
	}

	return fmt.Sprintf("%s_%d%s", baseName, time.Now().Unix(), ext)
}

func (s *Store) CountFilesRecursive(folderID string, totalSize *int64, fileCount *int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.countFilesRecursiveLocked(folderID, totalSize, fileCount)
}

func (s *Store) countFilesRecursiveLocked(folderID string, totalSize *int64, fileCount *int) {
	for _, file := range s.db.Files {
		if file.FolderID == folderID {
			*totalSize += file.Size
			*fileCount++
		}
	}

	for _, folder := range s.db.Folders {
		if folder.ParentID == folderID {
			s.countFilesRecursiveLocked(folder.ID, totalSize, fileCount)
		}
	}
}

func (s *Store) UpdateFolderStats(folderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var folderChain []string
	currentID := folderID

	for currentID != "" {
		folderChain = append(folderChain, currentID)
		folder, ok := s.db.Folders[currentID]
		if !ok {
			break
		}
		currentID = folder.ParentID
	}

	for _, fid := range folderChain {
		var totalSize int64
		var fileCount int

		s.countFilesRecursiveLocked(fid, &totalSize, &fileCount)

		folder, ok := s.db.Folders[fid]
		if !ok {
			continue
		}

		folder.Size = totalSize
		folder.FileCount = fileCount
		s.db.Folders[fid] = folder
	}

	return s.saveLocked()
}

func (s *Store) MoveToTrash(fileID, userID string) (models.TrashItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, ok := s.db.Files[fileID]
	if !ok || file.UserID != userID {
		return models.TrashItem{}, ErrNotFound
	}

	trashItem := models.TrashItem{
		ID:             "trash_" + fileID,
		UserID:         userID,
		OriginalName:   file.Name,
		OriginalType:   "file",
		OriginalFolder: file.FolderID,
		StorageKey:     file.Versions[len(file.Versions)-1].StorageKey,
		Size:           file.Size,
		DeletedAt:      time.Now().UTC(),
	}

	s.db.Trash[trashItem.ID] = trashItem
	delete(s.db.Files, fileID)

	if err := s.saveLocked(); err != nil {
		return models.TrashItem{}, err
	}

	return trashItem, nil
}

func (s *Store) RestoreFromTrash(trashID, userID string) (models.FileMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	trash, ok := s.db.Trash[trashID]
	if !ok || trash.UserID != userID {
		return models.FileMeta{}, ErrNotFound
	}

	file := models.FileMeta{
		ID:        trash.ID[6:],
		UserID:    userID,
		Name:      trash.OriginalName,
		Size:      trash.Size,
		FolderID:  trash.OriginalFolder,
		Versions:  []models.FileVersion{{Version: 1, StorageKey: trash.StorageKey, Size: trash.Size, CreatedAt: trash.DeletedAt}},
		CreatedAt: trash.DeletedAt,
		UpdatedAt: time.Now().UTC(),
	}

	s.db.Files[file.ID] = file
	delete(s.db.Trash, trashID)

	if err := s.saveLocked(); err != nil {
		return models.FileMeta{}, err
	}

	return file, nil
}

func (s *Store) GetTrashItem(trashID, userID string) (models.TrashItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trash, ok := s.db.Trash[trashID]
	if !ok || trash.UserID != userID {
		return models.TrashItem{}, ErrNotFound
	}

	return trash, nil
}

func (s *Store) DeleteTrashItem(trashID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	trash, ok := s.db.Trash[trashID]
	if !ok || trash.UserID != userID {
		return ErrNotFound
	}

	delete(s.db.Trash, trashID)
	return s.saveLocked()
}

func (s *Store) ListTrash(userID string) []models.TrashItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []models.TrashItem
	for _, item := range s.db.Trash {
		if item.UserID == userID {
			items = append(items, item)
		}
	}

	return items
}

func (s *Store) CreateShareLink(userID, resourceType, resourceID string) (models.ShareLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if resourceType == "file" {
		file, ok := s.db.Files[resourceID]
		if !ok || file.UserID != userID {
			return models.ShareLink{}, ErrNotFound
		}
	} else if resourceType == "folder" {
		folder, ok := s.db.Folders[resourceID]
		if !ok || folder.UserID != userID {
			return models.ShareLink{}, ErrNotFound
		}
	} else {
		return models.ShareLink{}, errors.New("invalid resource type")
	}

	shareLink := models.ShareLink{
		ID:           randomHex(16),
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Token:        randomHex(32),
		CreatedAt:    time.Now().UTC(),
	}

	s.db.ShareLinks[shareLink.ID] = shareLink

	if err := s.saveLocked(); err != nil {
		return models.ShareLink{}, err
	}

	return shareLink, nil
}

func (s *Store) GetShareLink(token string) (models.ShareLink, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, link := range s.db.ShareLinks {
		if link.Token == token {
			return link, true
		}
	}

	return models.ShareLink{}, false
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func randomHex(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func hashPassword(password, salt string) string {
	data := []byte(salt + ":" + password)
	for i := 0; i < 50000; i++ {
		h := sha256.Sum256(data)
		data = h[:]
	}
	return hex.EncodeToString(data)
}

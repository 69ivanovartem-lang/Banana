package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
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

type Store struct {
	mu      sync.RWMutex
	db      DB
	dataDir string
	dbPath  string
}

func NewStore(dataDir string) (*Store, error) {
	for _, d := range []string{filepath.Join(dataDir, "blobs"), filepath.Join(dataDir, "trash"), filepath.Join(dataDir, "files")} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return nil, err
		}
	}
	s := &Store{
		db: DB{
			Users: make(map[string]User), Files: make(map[string]FileMeta),
			Folders: make(map[string]Folder), Trash: make(map[string]TrashItem),
			ShareLinks: make(map[string]ShareLink), EmailIndex: make(map[string]string),
			Sessions: make(map[string]Session),
		},
		dataDir: dataDir, dbPath: filepath.Join(dataDir, "db.json"),
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
	tmp := s.dbPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.dbPath)
}

func (s *Store) CreateUser(email, password string) (User, error) {
	email = normalizeEmail(email)
	salt := randomHex(16)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.db.EmailIndex[email]; exists {
		return User{}, ErrEmailExists
	}
	user := User{ID: randomHex(16), Email: email, PasswordHash: hashPassword(password, salt), Salt: salt, CreatedAt: time.Now().UTC()}
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
	if subtle.ConstantTimeCompare([]byte(hashPassword(password, user.Salt)), []byte(user.PasswordHash)) == 0 {
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
		return session
	}
	return session
}

func (s *Store) UserIDByToken(token string) (string, bool) {
	s.mu.RLock()
	session, ok := s.db.Sessions[token]
	s.mu.RUnlock()
	if !ok || time.Now().UTC().After(session.ExpiresAt) {
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
	for _, f := range s.db.Folders {
		if f.UserID == userID && f.ParentID == parentID {
			folders = append(folders, f)
		}
	}
	sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })
	return folders
}

func (s *Store) GetFolderForUser(folderID, userID string) (Folder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.db.Folders[folderID]
	if !ok || f.UserID != userID {
		return Folder{}, ErrNotFound
	}
	return f, nil
}

func (s *Store) RenameFolder(folderID, userID, newName string) (Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.db.Folders[folderID]
	if !ok || f.UserID != userID {
		return Folder{}, ErrNotFound
	}
	f.Name = newName
	s.db.Folders[folderID] = f
	if err := s.saveLocked(); err != nil {
		return Folder{}, err
	}
	return f, nil
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
	var fobjs []Folder
	for _, f := range s.db.Files {
		if f.UserID == userID && f.FolderID == folderID {
			files = append(files, toFileResponse(f))
		}
	}
	for _, fo := range s.db.Folders {
		if fo.UserID == userID && fo.ParentID == folderID {
			fobjs = append(fobjs, fo)
		}
	}
	s.mu.RUnlock()
	folders := make([]folderResponse, 0, len(fobjs))
	for _, fo := range fobjs {
		fr := toFolderResponse(fo)
		fr.Size, fr.FileCount = s.folderStats(fo.ID)
		folders = append(folders, fr)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].FileType != files[j].FileType {
			return files[i].FileType < files[j].FileType
		}
		return files[i].Name < files[j].Name
	})
	sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })
	return files, folders
}

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
		c := name
		_ = c
		cand := base + " (" + itoa(i) + ")" + ext
		if !s.nameExists(userID, parentID, cand, "") {
			return cand
		}
	}
	return base + "_" + itoa(int(time.Now().Unix())) + ext
}

func (s *Store) uniqueIfTaken(userID, folderID, name string) string {
	s.mu.RLock()
	taken := s.nameExists(userID, folderID, name, "")
	s.mu.RUnlock()
	if !taken {
		return name
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.uniqueNameLocked(userID, folderID, name)
}

func (s *Store) AddFile(f FileMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Files[f.ID] = f
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
	sort.Slice(files, func(i, j int) bool {
		if files[i].FileType != files[j].FileType {
			return files[i].FileType < files[j].FileType
		}
		return files[i].Name < files[j].Name
	})
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
	f.UpdatedAt = time.Now().UTC()
	s.db.Files[fileID] = f
	if err := s.saveLocked(); err != nil {
		return FileMeta{}, err
	}
	return f, nil
}

func (s *Store) RemoveFile(fileID, userID string) (FileMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.db.Files[fileID]
	if !ok || f.UserID != userID {
		return FileMeta{}, ErrNotFound
	}
	delete(s.db.Files, fileID)
	if err := s.saveLocked(); err != nil {
		return FileMeta{}, err
	}
	return f, nil
}

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
	for _, l := range s.db.ShareLinks {
		if l.Token == token {
			return l, true
		}
	}
	return ShareLink{}, false
}

func (s *Store) DeleteShareLink(linkID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.db.ShareLinks[linkID]
	if !ok || l.UserID != userID {
		return ErrNotFound
	}
	delete(s.db.ShareLinks, linkID)
	return s.saveLocked()
}

package main

import "time"

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
	FileType    string    `json:"fileType"`
	StorageKey  string    `json:"storageKey"`
	FolderID    string    `json:"folderID"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type TrashItem struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"userID"`
	Type               string     `json:"type"`
	OriginalName       string     `json:"originalName"`
	OriginalFolder     string     `json:"originalFolder"`
	OriginalFolderName string     `json:"originalFolderName"`
	TrashKey           string     `json:"trashKey"`
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

type fileResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	FileType    string    `json:"fileType"`
	FolderID    string    `json:"folderID"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	DownloadURL string    `json:"downloadUrl"`
	PreviewURL  string    `json:"previewUrl"`
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

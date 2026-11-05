package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"passwordHash"`
	Salt         string    `json:"salt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type FileVersion struct {
	Version    int       `json:"version"`
	StorageKey string    `json:"storageKey"`
	Size       int64     `json:"size"`
	CreatedAt  time.Time `json:"createdAt"`
}

type FileMeta struct {
	ID          string        `json:"id"`
	UserID      string        `json:"userID"`
	Name        string        `json:"name"`
	Size        int64         `json:"size"`
	ContentType string        `json:"contentType"`
	FileType    string        `json:"fileType"`
	FolderID    string        `json:"folderID"`
	Versions    []FileVersion `json:"versions"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
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

type TrashItem struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userID"`
	OriginalName   string    `json:"originalName"`
	OriginalType   string    `json:"originalType"`
	OriginalFolder string    `json:"originalFolder"`
	StorageKey     string    `json:"storageKey"`
	Size           int64     `json:"size"`
	DeletedAt      time.Time `json:"deletedAt"`
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

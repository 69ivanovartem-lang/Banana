//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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

type Folder struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userID"`
	Name      string    `json:"name"`
	ParentID  string    `json:"parentID"`
	CreatedAt time.Time `json:"createdAt"`
}

type DB struct {
	Users      map[string]map[string]interface{} `json:"users"`
	Files      map[string]FileMeta               `json:"files"`
	Folders    map[string]Folder                 `json:"folders"`
	Trash      map[string]map[string]interface{} `json:"trash"`
	ShareLinks map[string]map[string]interface{} `json:"shareLinks"`
	EmailIndex map[string]string                 `json:"emailIndex"`
}

func main() {
	dbPath := filepath.Join(".", "data", "db.json")

	raw, err := os.ReadFile(dbPath)
	if err != nil {
		fmt.Println("НЕ МОГУ прочитать", dbPath, ":", err)
		return
	}

	var db DB
	if err := json.Unmarshal(raw, &db); err != nil {
		fmt.Println("db.json повреждён:", err)
		return
	}
	if db.Users == nil {
		db.Users = map[string]map[string]interface{}{}
	}
	if db.Files == nil {
		db.Files = map[string]FileMeta{}
	}
	if db.Folders == nil {
		db.Folders = map[string]Folder{}
	}
	if db.EmailIndex == nil {
		db.EmailIndex = map[string]string{}
	}

	// 1. Определяем ЦЕЛЕВОГО пользователя (под которым ты входишь)
	target := db.EmailIndex["123123@example.com"]
	if target == "" {
		for id := range db.Users {
			target = id
			break
		}
	}
	if target == "" {
		target = "recovered-user"
		db.Users[target] = map[string]interface{}{
			"id": target, "email": "recovered@local",
			"passwordHash": "x", "salt": "x", "createdAt": time.Now().UTC(),
		}
		db.EmailIndex["recovered@local"] = target
	}
	fmt.Println("Целевой userID:", target)

	// 2. Перепривязываем ВСЕ папки и файлы к целевому пользователю
	fixedF, fixedD := 0, 0
	for id, f := range db.Folders {
		if f.UserID != target {
			f.UserID = target
			db.Folders[id] = f
			fixedF++
		}
	}
	for id, f := range db.Files {
		if f.UserID != target {
			f.UserID = target
			db.Files[id] = f
			fixedD++
		}
	}
	fmt.Printf("Перепривязано папок: %d, файлов: %d\n", fixedF, fixedD)

	// 3. Чиним битые ParentID / FolderID (если родитель удалён — в корень)
	for id, f := range db.Folders {
		if f.ParentID != "" {
			if _, ok := db.Folders[f.ParentID]; !ok {
				f.ParentID = ""
				db.Folders[id] = f
			}
		}
	}
	for id, f := range db.Files {
		if f.FolderID != "" {
			if _, ok := db.Folders[f.FolderID]; !ok {
				f.FolderID = ""
				db.Files[id] = f
			}
		}
	}

	// 4. Регистрируем файлы-сироты с диска (data/files и data/blobs)
	known := map[string]bool{}
	for _, f := range db.Files {
		known[f.StorageKey] = true
	}
	added := 0
	for _, dir := range []string{"files", "blobs"} {
		entries, err := os.ReadDir(filepath.Join(".", "data", dir))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || known[e.Name()] {
				continue
			}
			ext := filepath.Ext(e.Name())
			if ext == ".gz" || ext == ".tmp" {
				continue
			}
			info, _ := e.Info()
			db.Files[e.Name()] = FileMeta{
				ID:          e.Name(),
				UserID:      target,
				Name:        "restored_" + e.Name(),
				Size:        info.Size(),
				ContentType: "application/octet-stream",
				StorageKey:  e.Name(),
				FolderID:    "",
				CreatedAt:   info.ModTime().UTC(),
			}
			known[e.Name()] = true
			added++
		}
	}
	fmt.Println("Добавлено файлов-сирот:", added)

	// 5. Бэкап и сохранение
	backup := dbPath + ".backup"
	os.WriteFile(backup, raw, 0o600)
	out, _ := json.MarshalIndent(db, "", "  ")
	tmp := dbPath + ".tmp"
	os.WriteFile(tmp, out, 0o600)
	os.Rename(tmp, dbPath)

	fmt.Println("✅ ГОТОВО. Итого файлов:", len(db.Files), "папок:", len(db.Folders))
	fmt.Println("Теперь запускай сервер и входи под 123123@example.com")
}

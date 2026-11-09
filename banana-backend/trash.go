package main

import "time"

func (s *Store) TrashFile(fileID, userID string) (TrashItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.db.Files[fileID]
	if !ok || f.UserID != userID {
		return TrashItem{}, ErrNotFound
	}
	itemID := "t" + randomHex(15)
	archive, err := s.compressToTrash(itemID, []FileMeta{f}, map[string]bool{fileID: true})
	if err != nil {
		return TrashItem{}, err
	}
	cp := f
	item := TrashItem{
		ID: itemID, UserID: userID, Type: "file", OriginalName: f.Name,
		OriginalFolder: f.FolderID, TrashKey: archive, Size: f.Size, FileCount: 1,
		SnapshotFile: &cp, DeletedAt: time.Now().UTC(),
	}
	if f.FolderID != "" {
		if fo, ok2 := s.db.Folders[f.FolderID]; ok2 {
			item.OriginalFolderName = fo.Name
		}
	}
	delete(s.db.Files, fileID)
	s.db.Trash[item.ID] = item
	if err := s.saveLocked(); err != nil {
		return TrashItem{}, err
	}
	return item, nil
}

func (s *Store) DeleteFolder(folderID, userID string) (TrashItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	folder, ok := s.db.Folders[folderID]
	if !ok || folder.UserID != userID {
		return TrashItem{}, ErrNotFound
	}
	var ids []string
	var collect func(string)
	collect = func(fid string) {
		ids = append(ids, fid)
		for _, sub := range s.db.Folders {
			if sub.ParentID == fid {
				collect(sub.ID)
			}
		}
	}
	collect(folderID)
	inSet := map[string]bool{}
	for _, id := range ids {
		inSet[id] = true
	}
	snapFolders := []Folder{folder}
	for _, id := range ids {
		if id != folderID {
			snapFolders = append(snapFolders, s.db.Folders[id])
		}
	}
	snapFiles := make([]FileMeta, 0)
	var total int64
	exclude := map[string]bool{}
	for _, f := range s.db.Files {
		if inSet[f.FolderID] {
			snapFiles = append(snapFiles, f)
			exclude[f.ID] = true
			total += f.Size
		}
	}
	itemID := "t" + randomHex(15)
	archive, err := s.compressToTrash(itemID, snapFiles, exclude)
	if err != nil {
		return TrashItem{}, err
	}
	item := TrashItem{
		ID: itemID, UserID: userID, Type: "folder", OriginalName: folder.Name,
		OriginalFolder: folder.ParentID, TrashKey: archive, Size: total,
		FileCount: len(snapFiles), FolderCount: len(snapFolders) - 1,
		SnapshotFolders: snapFolders, SnapshotFiles: snapFiles, DeletedAt: time.Now().UTC(),
	}
	if folder.ParentID != "" {
		if fo, ok2 := s.db.Folders[folder.ParentID]; ok2 {
			item.OriginalFolderName = fo.Name
		}
	}
	for _, id := range ids {
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

func (s *Store) ListTrash(userID string) []TrashItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]TrashItem, 0)
	for _, it := range s.db.Trash {
		if it.UserID == userID {
			items = append(items, it)
		}
	}
	sortTrashByDate(items)
	return items
}

func sortTrashByDate(items []TrashItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].DeletedAt.After(items[j-1].DeletedAt); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func (s *Store) RestoreTrash(trashID, userID string) error {
	s.mu.Lock()
	it, ok := s.db.Trash[trashID]
	s.mu.Unlock()
	if !ok || it.UserID != userID {
		return ErrNotFound
	}
	if err := s.restoreArchive(it); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
			nid := f.ID
			if _, occ := s.db.Folders[nid]; occ {
				nid = randomHex(16)
			}
			idMap[f.ID] = nid
		}
		for i, f := range it.SnapshotFolders {
			nf := f
			nf.UserID = userID
			nf.ID = idMap[f.ID]
			if i == 0 {
				nf.ParentID = parentID
				nf.Name = name
			} else if mp, ok2 := idMap[f.ParentID]; ok2 {
				nf.ParentID = mp
			}
			s.db.Folders[nf.ID] = nf
		}
		for _, f := range it.SnapshotFiles {
			nf := f
			nf.UserID = userID
			if _, occ := s.db.Files[nf.ID]; occ {
				nf.ID = randomHex(16)
			}
			if mp, ok2 := idMap[f.FolderID]; ok2 {
				nf.FolderID = mp
			}
			s.db.Files[nf.ID] = nf
		}
	} else {
		var f FileMeta
		if it.SnapshotFile != nil {
			f = *it.SnapshotFile
		} else {
			f = FileMeta{Name: it.OriginalName, Size: it.Size, StorageKey: it.TrashKey, CreatedAt: it.DeletedAt}
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
	it, ok := s.db.Trash[trashID]
	if !ok || it.UserID != userID {
		s.mu.Unlock()
		return TrashItem{}, ErrNotFound
	}
	delete(s.db.Trash, trashID)
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		return TrashItem{}, err
	}
	s.mu.Lock()
	s.purgeBlobs(it)
	s.mu.Unlock()
	return it, nil
}

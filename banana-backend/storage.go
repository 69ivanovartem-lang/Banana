package main

import (
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (s *Store) blobPath(h string) string   { return filepath.Join(s.dataDir, "blobs", h) }
func (s *Store) legacyPath(h string) string { return filepath.Join(s.dataDir, "files", h) }
func (s *Store) trashPath(n string) string  { return filepath.Join(s.dataDir, "trash", n) }

// blobFind ищет blob и в новом (blobs), и в старом (files) хранилище — ПОЧИНКА удалений/превью
func (s *Store) blobFind(key string) (string, bool) {
	p := s.blobPath(key)
	if _, err := os.Stat(p); err == nil {
		return p, true
	}
	p = s.legacyPath(key)
	if _, err := os.Stat(p); err == nil {
		return p, true
	}
	return "", false
}

func (s *Store) saveBlob(reader io.Reader) (string, int64, error) {
	hasher := sha256.New()
	tmp, err := os.CreateTemp(s.blobPath(""), "up-*")
	if err != nil {
		return "", 0, err
	}
	size, err := io.Copy(tmp, io.TeeReader(reader, hasher))
	tmp.Close()
	if err != nil {
		os.Remove(tmp.Name())
		return "", 0, err
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	final := s.blobPath(hash)
	if _, err := os.Stat(final); err == nil {
		os.Remove(tmp.Name())
		return hash, size, nil
	}
	return hash, size, os.Rename(tmp.Name(), final)
}

func (s *Store) blobRefs(hash string, exclude map[string]bool) int {
	n := 0
	for id, f := range s.db.Files {
		if exclude[id] {
			continue
		}
		if f.StorageKey == hash {
			n++
		}
	}
	return n
}

// compressToTrash пакует файлы в .gz (один) или .zip (папка). Если данных нет — архив пустой, удаление всё равно работает.
func (s *Store) compressToTrash(itemID string, metas []FileMeta, exclude map[string]bool) (string, error) {
	type ref struct{ hash, path string }
	var refs []ref
	seen := map[string]bool{}
	for _, f := range metas {
		if seen[f.StorageKey] {
			continue
		}
		if p, ok := s.blobFind(f.StorageKey); ok {
			seen[f.StorageKey] = true
			refs = append(refs, ref{f.StorageKey, p})
		}
	}
	if len(refs) == 0 {
		return "", nil
	}
	name := itemID + ".zip"
	if len(metas) == 1 {
		name = itemID + ".gz"
	}
	out, err := os.Create(s.trashPath(name))
	if err != nil {
		return "", err
	}
	if len(metas) == 1 {
		src, err2 := os.Open(refs[0].path)
		if err2 == nil {
			gz := gzip.NewWriter(out)
			_, err2 = io.Copy(gz, src)
			src.Close()
			gz.Close()
		}
		out.Close()
		if err2 != nil {
			os.Remove(s.trashPath(name))
			return "", err2
		}
	} else {
		zw := zip.NewWriter(out)
		var err2 error
		for _, rb := range refs {
			src, e := os.Open(rb.path)
			if e != nil {
				err2 = e
				break
			}
			fw, e := zw.Create(rb.hash)
			if e == nil {
				_, e = io.Copy(fw, src)
			}
			src.Close()
			if e != nil {
				err2 = e
				break
			}
		}
		zw.Close()
		out.Close()
		if err2 != nil {
			os.Remove(s.trashPath(name))
			return "", err2
		}
	}
	for _, rb := range refs {
		if s.blobRefs(rb.hash, exclude) == 0 {
			os.Remove(rb.path)
		}
	}
	return name, nil
}

func (s *Store) restoreArchive(it TrashItem) error {
	if it.TrashKey == "" {
		return nil
	}
	path := s.trashPath(it.TrashKey)
	if strings.HasSuffix(it.TrashKey, ".gz") {
		var want string
		if it.SnapshotFile != nil {
			want = it.SnapshotFile.StorageKey
		}
		if want == "" {
			return os.Remove(path)
		}
		if _, exists := s.blobFind(want); exists {
			return os.Remove(path)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return err
		}
		tmp, err := os.CreateTemp(s.blobPath(""), "rs-*")
		if err != nil {
			gz.Close()
			f.Close()
			return err
		}
		_, err = io.Copy(tmp, gz)
		tmp.Close()
		gz.Close()
		f.Close()
		if err != nil {
			os.Remove(tmp.Name())
			return err
		}
		os.Rename(tmp.Name(), s.blobPath(want))
		return os.Remove(path)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if _, exists := s.blobFind(zf.Name); exists {
			continue
		}
		rc, err2 := zf.Open()
		if err2 != nil {
			return err2
		}
		tmp, err2 := os.CreateTemp(s.blobPath(""), "rs-*")
		if err2 != nil {
			rc.Close()
			return err2
		}
		_, err2 = io.Copy(tmp, rc)
		tmp.Close()
		rc.Close()
		if err2 != nil {
			os.Remove(tmp.Name())
			return err2
		}
		os.Rename(tmp.Name(), s.blobPath(zf.Name))
	}
	return os.Remove(path)
}

func (s *Store) purgeBlobs(it TrashItem) {
	if it.TrashKey != "" {
		os.Remove(s.trashPath(it.TrashKey))
	}
	metas := append([]FileMeta{}, it.SnapshotFiles...)
	if it.SnapshotFile != nil {
		metas = append(metas, *it.SnapshotFile)
	}
	for _, f := range metas {
		if s.blobRefs(f.StorageKey, nil) == 0 {
			if p, ok := s.blobFind(f.StorageKey); ok {
				os.Remove(p)
			}
		}
	}
}

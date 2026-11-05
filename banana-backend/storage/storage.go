package storage

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

type Storage struct {
	blobsDir string
	trashDir string
}

func NewStorage(dataDir string) (*Storage, error) {
	blobsDir := filepath.Join(dataDir, "blobs")
	trashDir := filepath.Join(dataDir, "trash")
	if err := os.MkdirAll(blobsDir, 0o750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(trashDir, 0o750); err != nil {
		return nil, err
	}
	return &Storage{blobsDir: blobsDir, trashDir: trashDir}, nil
}

func (s *Storage) SaveFile(reader io.Reader) (string, int64, error) {
	hasher := sha256.New()
	teeReader := io.TeeReader(reader, hasher)

	tempFile, err := os.CreateTemp(s.blobsDir, "tmp-*")
	if err != nil {
		return "", 0, err
	}
	defer tempFile.Close()

	size, err := io.Copy(tempFile, teeReader)
	if err != nil {
		return "", 0, err
	}
	tempFile.Close()

	hash := hex.EncodeToString(hasher.Sum(nil))
	finalPath := filepath.Join(s.blobsDir, hash)

	if _, err := os.Stat(finalPath); err == nil {
		os.Remove(tempFile.Name())
		return hash, size, nil
	}

	return hash, size, os.Rename(tempFile.Name(), finalPath)
}

func (s *Storage) GetFile(hash string) (*os.File, error) {
	return os.Open(filepath.Join(s.blobsDir, hash))
}

func (s *Storage) CompressAndMoveToTrash(hash string) (string, int64, error) {
	srcPath := filepath.Join(s.blobsDir, hash)
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", 0, err
	}
	defer srcFile.Close()

	trashFileName := hash + ".gz"
	trashPath := filepath.Join(s.trashDir, trashFileName)

	gzFile, err := os.Create(trashPath)
	if err != nil {
		return "", 0, err
	}
	defer gzFile.Close()

	gzWriter := gzip.NewWriter(gzFile)

	size, err := io.Copy(gzWriter, srcFile)
	if err != nil {
		gzWriter.Close()
		return "", 0, err
	}
	gzWriter.Close()
	gzFile.Close()

	os.Remove(srcPath)

	return trashFileName, size, nil
}

func (s *Storage) RestoreFromTrash(trashFileName string) (string, error) {
	trashPath := filepath.Join(s.trashDir, trashFileName)
	gzFile, err := os.Open(trashPath)
	if err != nil {
		return "", err
	}
	defer gzFile.Close()

	gzReader, err := gzip.NewReader(gzFile)
	if err != nil {
		return "", err
	}
	defer gzReader.Close()

	hasher := sha256.New()
	teeReader := io.TeeReader(gzReader, hasher)

	tempFile, err := os.CreateTemp(s.blobsDir, "restore-*")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, teeReader); err != nil {
		return "", err
	}
	tempFile.Close()

	hash := hex.EncodeToString(hasher.Sum(nil))
	finalPath := filepath.Join(s.blobsDir, hash)

	os.Rename(tempFile.Name(), finalPath)
	os.Remove(trashPath)

	return hash, nil
}

func (s *Storage) DeletePermanently(key string, isTrash bool) error {
	dir := s.trashDir
	if !isTrash {
		dir = s.blobsDir
	}
	return os.Remove(filepath.Join(dir, key))
}

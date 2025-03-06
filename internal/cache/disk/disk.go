package disk

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/cirruslabs/chacha/internal/cache"
	"github.com/samber/lo"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

const (
	fileInfo = "info.json"
	fileBlob = "blob.bin"
)

type WalkFunc func(fs.File, Info, error) error

type Disk struct {
	dir        string
	limitBytes uint64
	mtx        sync.Mutex
}

func New(dir string, limitBytes uint64) (*Disk, error) {
	disk := &Disk{
		dir:        dir,
		limitBytes: limitBytes,
	}

	// Pre-create the disk's directory if not created yet
	if err := os.MkdirAll(dir, 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	return disk, nil
}

func (disk *Disk) Get(_ context.Context, key string) (io.ReadCloser, cache.Metadata, error) {
	disk.mtx.Lock()
	defer disk.mtx.Unlock()

	cacheFile, err := os.Open(disk.path(key))
	if err != nil {
		// Convert the error for consumer's convenience
		if errors.Is(err, os.ErrNotExist) {
			return nil, cache.Metadata{}, cache.ErrNotFound
		}

		return nil, cache.Metadata{}, fmt.Errorf("failed to open cache entry %q: %w", key, err)
	}

	// Update the access and modification times so that eviction would work correctly
	now := time.Now()

	if err := os.Chtimes(disk.path(key), now, now); err != nil {
		_ = cacheFile.Close()

		// Convert the error for consumer's convenience
		if errors.Is(err, os.ErrNotExist) {
			return nil, cache.Metadata{}, cache.ErrNotFound
		}

		return nil, cache.Metadata{}, fmt.Errorf("failed to set access and modification times "+
			" for the cache entry %q: %w", key, err)
	}

	blobReader, info, err := disk.getInner(cacheFile)
	if err != nil {
		_ = cacheFile.Close()

		return nil, cache.Metadata{}, fmt.Errorf("failed to read cache entry %q: %w", key, err)
	}

	return &Reader{
		cacheFile:  cacheFile,
		blobReader: blobReader,
	}, info.Metadata, nil
}

func (disk *Disk) Put(_ context.Context, key string, metadata cache.Metadata, blobReader io.Reader) error {
	tmpFile, err := os.CreateTemp("", "chacha-put-*")
	if err != nil {
		return fmt.Errorf("failed to create a temporary file for the cache entry %q: %w",
			key, err)
	}

	// Write the cache entry as a ZIP file
	zipWriter := zip.NewWriter(tmpFile)

	// Write cache entry's info
	if err := writeInfo(zipWriter, Info{
		Key:      key,
		Metadata: metadata,
	}); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())

		return fmt.Errorf("failed to write %q file to the cache entry %q: %w",
			fileInfo, key, err)
	}

	// Acquire a handle to the cache entry's underlying blob
	blobWriter, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:   fileBlob,
		Method: zip.Store,
	})
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())

		return fmt.Errorf("failed to write %q file to the cache entry %q: %w",
			fileBlob, key, err)
	}

	if _, err := io.Copy(blobWriter, blobReader); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())

		return fmt.Errorf("failed to write %q file to the cache entry %q: %w",
			fileBlob, key, err)
	}

	if err := zipWriter.Close(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())

		return fmt.Errorf("failed to finalize cache entry %q: %w", key, err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())

		return fmt.Errorf("failed to close cache entry %q: %w", key, err)
	}

	if err := disk.accept(key, tmpFile.Name()); err != nil {
		_ = os.Remove(tmpFile.Name())

		return fmt.Errorf("failed to accept cache entry %q: %w", key, err)
	}

	return nil
}

func (disk *Disk) Walk(walkFunc WalkFunc) error {
	dirEntries, err := os.ReadDir(disk.dir)
	if err != nil {
		return err
	}

	for _, dirEntry := range dirEntries {
		cacheFile, err := os.Open(filepath.Join(disk.dir, dirEntry.Name()))
		if err != nil {
			if err := walkFunc(nil, Info{}, err); err != nil {
				return err
			}

			continue
		}

		if err := walkFunc(disk.getInner(cacheFile)); err != nil {
			return err
		}
	}

	return nil
}

func (disk *Disk) Delete(key string) error {
	disk.mtx.Lock()
	defer disk.mtx.Unlock()

	if err := os.Remove(disk.path(key)); err != nil {
		// Convert the error for consumer's convenience
		if errors.Is(err, os.ErrNotExist) {
			return cache.ErrNotFound
		}

		return err
	}

	return nil
}

func (disk *Disk) path(key string) string {
	// On macOS, the maximum filename length is 255 characters (inclusive),
	// so the safest way to avoid errors is to hash the cache entry's key
	hash := sha256.Sum256([]byte(key))

	return filepath.Join(disk.dir, hex.EncodeToString(hash[:]))
}

func (disk *Disk) getInner(cacheFile *os.File) (fs.File, Info, error) {
	// Open the cache entry as a ZIP file
	fi, err := cacheFile.Stat()
	if err != nil {
		// Convert the error for consumer's convenience
		if errors.Is(err, os.ErrNotExist) {
			return nil, Info{}, cache.ErrNotFound
		}

		return nil, Info{}, fmt.Errorf("stat(2) failed: %w", err)
	}

	zipReader, err := zip.NewReader(cacheFile, fi.Size())
	if err != nil {
		return nil, Info{}, fmt.Errorf("failed to open as a ZIP file: %w", err)
	}

	// Read cache entry's info
	info, err := readInfo(zipReader)
	if err != nil {
		return nil, Info{}, fmt.Errorf("failed to read from ZIP file: %w", err)
	}

	// Acquire a handle to the cache entry's underlying blob
	blobReader, err := zipReader.Open(fileBlob)
	if err != nil {
		return nil, Info{}, fmt.Errorf("failed to read from ZIP file: %w", err)
	}

	return blobReader, *info, nil
}

func (disk *Disk) accept(key string, path string) error {
	disk.mtx.Lock()
	defer disk.mtx.Unlock()

	// Prepare for accepting the new cache entry
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	if err := disk.evict(uint64(fi.Size())); err != nil {
		return err
	}

	// Accept new cache entry
	return os.Rename(path, disk.path(key))
}

func (disk *Disk) evict(needBytes uint64) error {
	// Does it even make sense to evict anything?
	if needBytes > disk.limitBytes {
		return fmt.Errorf("cannot accept cache entry as it's size of %d bytes"+
			" is larger than the disk limit of %d bytes", needBytes, disk.limitBytes)
	}

	// Collect a slice of cache entries, sorted by modification time, ascending order
	type Entry struct {
		Name    string
		Size    uint64
		ModTime time.Time
	}

	var entries []*Entry

	dirEntries, err := os.ReadDir(disk.dir)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		fi, err := entry.Info()
		if err != nil {
			return err
		}

		entries = append(entries, &Entry{
			Name:    entry.Name(),
			Size:    uint64(fi.Size()),
			ModTime: fi.ModTime(),
		})
	}

	slices.SortFunc(entries, func(a, b *Entry) int {
		return a.ModTime.Compare(b.ModTime)
	})

	usedBytes := lo.SumBy(entries, func(entry *Entry) uint64 {
		return entry.Size
	})

	// Evict the oldest entries to fit the new entry
	for _, entry := range entries {
		if (usedBytes + needBytes) <= disk.limitBytes {
			return nil
		}

		if err := os.Remove(filepath.Join(disk.dir, entry.Name)); err != nil {
			return err
		}

		usedBytes -= entry.Size
	}

	return nil
}

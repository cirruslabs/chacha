package disk

import (
	"errors"
	"fmt"
	"github.com/cirruslabs/chacha/internal/cache"
	"github.com/cirruslabs/chacha/internal/cache/disk/percentencoding"
	"github.com/samber/lo"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

type Disk struct {
	dir      string
	maxBytes uint64
	mtx      sync.Mutex
}

func New(dir string, maxBytes uint64) (*Disk, error) {
	disk := &Disk{
		dir:      dir,
		maxBytes: maxBytes,
	}

	// Pre-create the disk's directory if not created yet
	if err := os.MkdirAll(dir, 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	return disk, nil
}

func (disk *Disk) Get(key string) (io.ReadCloser, error) {
	disk.mtx.Lock()
	defer disk.mtx.Unlock()

	file, err := os.Open(disk.path(key))
	if err != nil {
		return nil, convertErr(err)
	}

	// Update the access and modification times so that eviction would work correctly
	now := time.Now()

	if err := os.Chtimes(disk.path(key), now, now); err != nil {
		return nil, err
	}

	return file, nil
}

func (disk *Disk) Put(key string, path string) error {
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

func (disk *Disk) Delete(key string) error {
	disk.mtx.Lock()
	defer disk.mtx.Unlock()

	err := os.Remove(disk.path(key))

	return convertErr(err)
}

func (disk *Disk) path(key string) string {
	safeKey := percentencoding.Encode(key)

	return filepath.Join(disk.dir, safeKey)
}

func convertErr(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return cache.ErrNotFound
	}

	return err
}

func (disk *Disk) evict(needBytes uint64) error {
	// Does it even make sense to evict anything?
	if needBytes > disk.maxBytes {
		return fmt.Errorf("cannot accept cache entry as it's size of %d bytes"+
			" is larger than the cache size of %d bytes", needBytes, disk.maxBytes)
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
		if (usedBytes + needBytes) <= disk.maxBytes {
			return nil
		}

		if err := os.Remove(filepath.Join(disk.dir, entry.Name)); err != nil {
			return err
		}

		usedBytes -= entry.Size
	}

	return nil
}

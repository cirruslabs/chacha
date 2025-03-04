package disk

import (
	"io/fs"
	"os"
)

type Reader struct {
	cacheFile  *os.File
	blobReader fs.File
}

func (entry *Reader) Read(p []byte) (int, error) {
	return entry.blobReader.Read(p)
}

func (entry *Reader) Close() error {
	if err := entry.blobReader.Close(); err != nil {
		return err
	}

	return entry.cacheFile.Close()
}

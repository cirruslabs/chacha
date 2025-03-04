package disk

import (
	"archive/zip"
	"encoding/json"
	"github.com/cirruslabs/chacha/internal/cache"
)

type Info struct {
	Key      string         `json:"key"`
	Metadata cache.Metadata `json:"metadata"`
}

func readInfo(zipReader *zip.Reader) (*Info, error) {
	infoReader, err := zipReader.Open(fileInfo)
	if err != nil {
		return nil, err
	}

	var info Info

	if err := json.NewDecoder(infoReader).Decode(&info); err != nil {
		return nil, err
	}

	if err := infoReader.Close(); err != nil {
		return nil, err
	}

	return &info, nil
}

func writeInfo(zipWriter *zip.Writer, info Info) error {
	infoWriter, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:   fileInfo,
		Method: zip.Store,
	})
	if err != nil {
		return err
	}

	if err := json.NewEncoder(infoWriter).Encode(&info); err != nil {
		return err
	}

	return nil
}

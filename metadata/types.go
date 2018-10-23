package metadata

import "time"

type FileMetadata struct {
	ModDate time.Time
}

type Store interface {
	Get(fileAddress string) (bool, FileMetadata, error)
	Set(fileAddress string, metadata FileMetadata) error
}

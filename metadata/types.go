package metadata

import "time"

type FileMetadata struct {
	RemoteModDate time.Time
	LocalModDate  time.Time
}

type Store interface {
	Get(fileAddress string) (bool, FileMetadata, error)
	Set(fileAddress string, metadata FileMetadata) error
}

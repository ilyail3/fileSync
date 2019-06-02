package metadata

import (
	"errors"
	"fmt"
	"log"
)

type ConfigStore interface {
	ReadStringConfig(key string) (bool, string, error)
	WriteStringConfig(key, value string) error
}

func (s *SqliteMetadataStore) ReadStringConfig(key string) (bool, string, error) {
	result, err := s.readStringConfig.Query(key)

	if err != nil {
		return false, "", fmt.Errorf("failed to get string key for '%s': %v", key, err)
	}

	defer func() {
		err := result.Close()

		if err != nil {
			log.Printf("failed to close sql rows: %v", err)
		}
	}()

	if !result.Next() {
		return false, "", nil
	} else {
		var resultValue string

		err = result.Scan(&resultValue)

		if err != nil {
			return false, "", fmt.Errorf("failed to scan for config key")
		}

		return true, resultValue, nil
	}
}

func (s *SqliteMetadataStore) WriteStringConfig(key, value string) error {
	r, err := s.writeStringConfig.Exec(key, value)

	if err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	rowAffected, err := r.RowsAffected()

	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rowAffected != 1 {
		return errors.New("no rows affected")
	}

	return nil
}

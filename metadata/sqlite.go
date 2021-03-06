package metadata

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"path"
	"time"
)

type SqliteMetadataStore struct {
	db                *sql.DB
	getQuery          *sql.Stmt
	putQuery          *sql.Stmt
	readStringConfig  *sql.Stmt
	writeStringConfig *sql.Stmt
}

func (s *SqliteMetadataStore) Get(fileAddress string) (bool, FileMetadata, error) {
	result, err := s.getQuery.Query(fileAddress)

	if err != nil {
		return false, FileMetadata{}, fmt.Errorf("failed to get metadata from sqlite: %v", err)
	}

	defer func() {
		err := result.Close()

		if err != nil {
			log.Printf("failed to close sql rows: %v", err)
		}
	}()

	if !result.Next() {
		return false, FileMetadata{}, nil
	}

	var remoteModDate string
	var localModDate string

	err = result.Scan(&remoteModDate, &localModDate)

	if err != nil {
		return false, FileMetadata{}, fmt.Errorf("failed to scan get query results: %v", err)
	}

	remoteModDateTime, err := time.Parse(time.RFC3339, remoteModDate)

	if err != nil {
		return false, FileMetadata{}, fmt.Errorf("failed to parse value '%s': %v", remoteModDate, err)
	}

	localModDateTime, err := time.Parse(time.RFC3339, localModDate)

	if err != nil {
		return false, FileMetadata{}, fmt.Errorf("failed to parse value '%s': %v", localModDate, err)
	}

	return true, FileMetadata{LocalModDate: localModDateTime, RemoteModDate: remoteModDateTime}, nil
}

func (s *SqliteMetadataStore) Set(fileAddress string, metadata FileMetadata) error {
	mtStringRemote := metadata.RemoteModDate.UTC().Format(time.RFC3339)
	mtStringLocal := metadata.LocalModDate.UTC().Format(time.RFC3339)

	result, err := s.putQuery.Exec(fileAddress, mtStringRemote, mtStringLocal)

	if err != nil {
		return fmt.Errorf("failed to write metadata: %v", err)
	}

	ra, err := result.RowsAffected()

	if err != nil {
		return fmt.Errorf("failed to query affected rows: %v", err)
	}

	if ra != 1 {
		return fmt.Errorf("expecting 1 row to be affected, %d where", ra)
	}

	return nil
}

func (s *SqliteMetadataStore) Close() error {
	return s.db.Close()
}

func (s *SqliteMetadataStore) GetAllSyncedFiles() ([]string, error) {
	files := make([]string, 0)

	rows, err := s.db.Query("SELECT filename FROM sync_mt")

	if err != nil {
		return files, fmt.Errorf("failed to query for synced filenames: %v", err)
	}

	defer func() {
		err := rows.Close()

		if err != nil {
			log.Printf("failed to close rows: %v")
		}
	}()

	var filename string

	for rows.Next() {
		err = rows.Scan(&filename)

		if err != nil {
			return files, fmt.Errorf("failed to scan row: %v", err)
		}

		files = append(files, filename)
	}

	return files, nil
}

func runQuery(db *sql.DB, query string) error {
	statement, err := db.Prepare(query)

	if err != nil {
		return fmt.Errorf("failed to prepare mod_date create table query: %v", err)
	}

	_, err = statement.Exec()

	if err != nil {
		return fmt.Errorf("failed to execute mod_date create table query: %v", err)
	}

	return nil
}

func createNewDatabase(db *sql.DB) error {
	err := runQuery(db, "CREATE TABLE sync_mt(filename text primary key, remote_mod_date text, local_mod_date text);")

	if err != nil {
		return err
	}

	err = runQuery(db, "CREATE TABLE config_string(config_key text primary key, string_value text);")

	if err != nil {
		return err
	}

	return nil
}

func NewSQLite3Store(dirName string) (*SqliteMetadataStore, error) {
	fileName := path.Join(dirName, "sync.sqlite3")

	_, err := os.Stat(fileName)

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to stat database file: %v", err)
	}

	newDB := os.IsNotExist(err)

	database, err := sql.Open("sqlite3", fileName)

	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite3 database: %v", err)
	}

	defer func() {
		err := database.Close()

		if err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	if newDB {
		err = createNewDatabase(database)

		if err != nil {
			return nil, fmt.Errorf("failed to close statement: %v", err)
		}
	}

	getQuery, err := database.Prepare("SELECT remote_mod_date,local_mod_date FROM sync_mt WHERE filename = ?")

	if err != nil {
		log.Fatalf("Failed to prepare get query: %v", err)
	}

	putQuery, err := database.Prepare("INSERT OR REPLACE INTO sync_mt(filename, remote_mod_date, local_mod_date) VALUES (?, ?, ?)")

	if err != nil {
		return nil, fmt.Errorf("failed to prepare put query: %v", err)
	}

	readStringConfigQuery, err := database.Prepare("SELECT string_value FROM config_string WHERE config_key = ?")

	if err != nil {
		log.Fatalf("Failed to prepare get query: %v", err)
	}

	writeStringConfigQuery, err := database.Prepare("INSERT OR REPLACE INTO config_string(config_key, string_value) VALUES (?, ?)")

	if err != nil {
		return nil, fmt.Errorf("failed to prepare put query: %v", err)
	}

	return &SqliteMetadataStore{
		db:                database,
		getQuery:          getQuery,
		putQuery:          putQuery,
		readStringConfig:  readStringConfigQuery,
		writeStringConfig: writeStringConfigQuery}, nil
}

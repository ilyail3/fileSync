package main

import (
	"github.com/ilyail3/fileSync/gdrive"
	"github.com/ilyail3/fileSync/metadata"
	"github.com/kardianos/osext"

	"log"
	"os"
	"path"
)

const SignKey = "DCB47525"
const SyncFolderName = "sync"

func main() {
	execPath, err := osext.Executable()

	if err != nil {
		log.Fatalf("failed to get executable path")
	}

	dirName := path.Dir(execPath)
	// log.Printf("dir is:%s", dirName)

	dbDir := path.Join(os.Getenv("HOME"), ".bin")
	mtStore, err := metadata.NewSQLite3Store(dbDir)

	if err != nil {
		log.Fatalf("Failed to open metadata store: %v", err)
	}

	defer mtStore.Close()

	srv, err := gdrive.NewService(dirName)

	if err != nil {
		log.Fatalf("Failed to inialize google drive service: %v", err)
	}

	parentId, err := gdrive.GetOrCreateDirectory(srv, SyncFolderName)

	if err != nil {
		log.Fatalf("failed to get parent directory: %v", err)
	}

	// log.Printf("folder id is:%s", parentId)

	//r, err := srv.Files.List().PageSize(10).
	//	Fields("nextPageToken, files(id, name)").Do()

	// if no filename given sync everything
	if len(os.Args) == 1 {
		files, err := mtStore.GetAllSyncedFiles()

		if err != nil {
			log.Fatalf("failed to read all synced filenames: %v", err)
		}

		for _, fullAddress := range files {
			log.Printf("syncing file: %s", fullAddress)

			err = gdrive.SyncFile(fullAddress, parentId, srv, mtStore, SignKey)

			if err != nil {
				log.Fatalf("failed to sync filename %s: %v", fullAddress, err)
			}
		}
	} else {
		fullAddress := os.Args[1]
		err = gdrive.SyncFile(fullAddress, parentId, srv, mtStore, SignKey)

		if err != nil {
			log.Fatalf("failed to sync filename %s: %v", fullAddress, err)
		}
	}

}

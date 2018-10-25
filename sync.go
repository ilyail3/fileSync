package main

import (
	"flag"
	"fmt"
	"github.com/ilyail3/fileSync/gdrive"
	"github.com/ilyail3/fileSync/metadata"
	"github.com/kardianos/osext"

	"log"
	"os"
	"path"
)

const DefaultSignKey = ""
const DefaultSyncFolderName = "sync"

func getConfigOrDefault(db metadata.ConfigStore, keyName string, flag *string, defaultValue string) (string, error) {
	if *flag != "" {
		err := db.WriteStringConfig(keyName, *flag)

		if err != nil {
			return "", fmt.Errorf("failed to write")
		}

		return *flag, nil
	}

	exists, value, err := db.ReadStringConfig(keyName)

	if err != nil {
		return "", fmt.Errorf("error reading value: %v", err)
	}

	if !exists {
		return defaultValue, nil
	} else {
		return value, nil
	}
}

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

	signKeyFlag := flag.String("sign-key", "", "sign key gpg2 signature")
	folderNameFlag := flag.String("folder-name", "", "folder name for sync")

	flag.Parse()
	args := flag.Args()

	srv, err := gdrive.NewService(dirName)

	if err != nil {
		log.Fatalf("Failed to inialize google drive service: %v", err)
	}

	folderName, err := getConfigOrDefault(
		mtStore,
		"folder-name",
		folderNameFlag,
		DefaultSyncFolderName)

	if err != nil {
		log.Fatalf("failed to get or write folder name: %v", err)
	}

	parentId, err := gdrive.GetOrCreateDirectory(srv, folderName)

	if err != nil {
		log.Fatalf("failed to get parent directory: %v", err)
	}

	signKey, err := getConfigOrDefault(
		mtStore,
		"sign-key",
		signKeyFlag,
		DefaultSignKey)

	if err != nil {
		log.Fatalf("failed to get or write signing key: %v", err)
	}

	if len(args) == 0 {
		files, err := mtStore.GetAllSyncedFiles()

		if err != nil {
			log.Fatalf("failed to read all synced filenames: %v", err)
		}

		for _, fullAddress := range files {
			log.Printf("syncing file: %s", fullAddress)

			err = gdrive.SyncFile(fullAddress, parentId, srv, mtStore, signKey)

			if err != nil {
				log.Fatalf("failed to sync filename %s: %v", fullAddress, err)
			}
		}
	} else {
		fullAddress := args[0]
		err = gdrive.SyncFile(fullAddress, parentId, srv, mtStore, signKey)

		if err != nil {
			log.Fatalf("failed to sync filename %s: %v", fullAddress, err)
		}
	}

}

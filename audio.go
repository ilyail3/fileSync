package main

import (
	"github.com/ilyail3/fileSync/gdrive"
	"github.com/kardianos/osext"
	"google.golang.org/api/drive/v3"
	"io/ioutil"
	"time"

	"log"
	"os"
	"path"
)

func main() {
	execPath, err := osext.Executable()

	if err != nil {
		log.Fatalf("failed to get executable path")
	}

	dirName := path.Dir(execPath)

	audioDir := path.Join(os.Getenv("HOME"), "Music/youtube")

	srv, err := gdrive.NewService(dirName)

	if err != nil {
		log.Fatalf("Failed to inialize google drive service: %v", err)
	}

	folderId, err := gdrive.GetOrCreateDirectory(srv, "youtube")

	if err != nil {
		log.Fatalf("Failed to create destination youtube folder")
	}

	files, err := ioutil.ReadDir(audioDir)

	if err != nil {
		log.Fatalf("failed to read audio folder: %v", err)
	}

	for _, file := range files {
		fullName := path.Join(audioDir, file.Name())
		fh, err := os.Open(fullName)

		if err != nil {
			log.Fatalf("failed to open file for upload: %v", err)
		}

		log.Printf("uploading %s", file.Name())

		f := drive.File{
			Name:         file.Name(),
			Properties:   map[string]string{},
			ModifiedTime: file.ModTime().Format(time.RFC3339),
			Parents:      []string{folderId}}

		_, err = srv.Files.Create(&f).Media(fh).Do()

		if err != nil {
			log.Fatalf("failed to upload file: %v", err)
		}

		err = fh.Close()

		if err != nil {
			log.Fatalf("failed to close file: %v", err)
		}

		err = os.Remove(fullName)

		if err != nil {
			log.Fatalf("failed to remove uploaded file: %v", err)
		}
	}

}

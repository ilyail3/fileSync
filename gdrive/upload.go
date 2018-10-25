package gdrive

import (
	"fmt"
	"github.com/ilyail3/fileSync/metadata"
	"google.golang.org/api/drive/v3"
	"log"
	"os"
	"os/exec"
	"path"
	"time"
)

func signFile(srv *drive.Service, address string, parentId string, signKey string) (string, error) {
	// gpg2 --yes --sign-with DCB47525 --detach-sig $1
	cmd := exec.Command("gpg2", "--yes", "--sign-with", signKey, "--detach-sig", address)
	err := cmd.Run()

	if err != nil {
		return "", fmt.Errorf("failed to sign file: %v", err)
	}

	signFile := address + ".sig"

	defer func() {
		os.Remove(signFile)
	}()

	fh, err := os.Open(signFile)

	if err != nil {
		return "", fmt.Errorf("failed to open signature file: %v", err)
	}

	defer fh.Close()

	f := drive.File{Name: path.Base(signFile), Parents: []string{parentId}}
	resultFile, err := srv.Files.Create(&f).Media(fh).Do()

	if err != nil {
		return "", fmt.Errorf("failed to upload signature file: %v", err)
	}

	return resultFile.Id, nil
}

func UploadFile(srv *drive.Service, address string, parentId string, metadataStore metadata.Store, signKey string) error {
	stats, err := os.Stat(address)

	if err != nil {
		return fmt.Errorf("failed to stat file: %v", err)
	}

	fh, err := os.Open(address)
	// modTime := time.Now().Format(time.RFC3339)

	if err != nil {
		return fmt.Errorf("failed to open file for uploading: %v", err)
	}

	defer fh.Close()

	properties := make(map[string]string)

	properties["mode"] = fmt.Sprintf("%d", stats.Mode())

	// sign
	if signKey != "" {
		signatureFileId, err := signFile(srv, address, parentId, signKey)

		if err != nil {
			return fmt.Errorf("failed to sign file: %v", err)
		}

		properties["gpg"] = signatureFileId
	}

	log.Printf("properties: %v", properties)

	modTime := time.Now()

	f := drive.File{
		Name:         path.Base(address),
		Properties:   properties,
		ModifiedTime: modTime.Format(time.RFC3339),
		Parents:      []string{parentId}}

	_, err = srv.Files.Create(&f).Media(fh).Do()

	if err != nil {
		return fmt.Errorf("upload operation failed: %v", err)
	}

	fileInfo, err := os.Stat(address)

	if err != nil {
		return fmt.Errorf("failed to stat downloaded file: %v", err)
	}

	err = metadataStore.Set(address, metadata.FileMetadata{RemoteModDate: modTime, LocalModDate: fileInfo.ModTime()})

	if err != nil {
		return fmt.Errorf("failed to update metadata: %v", err)
	}

	return nil
}

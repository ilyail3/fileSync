package gdrive

import (
	"fmt"
	"github.com/ilyail3/fileSync/metadata"
	"google.golang.org/api/drive/v3"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"
)

func DownloadFile(srv *drive.Service, address string, file *drive.File) error {
	f, err := srv.Files.Get(file.Id).Download()

	if err != nil {
		return fmt.Errorf("failed to get newer version from cloud: %v", err)
	}

	defer f.Body.Close()

	var mode os.FileMode = 0600
	modeString, exists := file.Properties["mode"]

	if exists {
		log.Printf("mode string is: %v", modeString)
		mode64, err := strconv.ParseInt(modeString, 10, 32)

		if err != nil {
			return fmt.Errorf("failed to parse mode string:%s", modeString)
		}

		mode = os.FileMode(mode64)
	}

	fh, err := os.OpenFile(address, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, mode)

	if err != nil {
		return fmt.Errorf("failed to open target file for update from cloud: %v", err)
	}

	defer fh.Close()

	_, err = io.Copy(fh, f.Body)

	if err != nil {
		return fmt.Errorf("failed to write download content from the cloud: %v", err)
	}

	return nil
}

func TmpDownloadFile(srv *drive.Service, address string, file *drive.File, metadataStore metadata.Store) error {
	dirName, fileName := path.Split(address)

	var renamed = false
	tmpAddress := path.Join(dirName, "_"+fileName)

	err := DownloadFile(srv, tmpAddress, file)

	if err != nil {
		return err
	}

	defer func() {
		if !renamed {
			os.Remove(tmpAddress)
		}
	}()

	gpgFileId, gpgExists := file.Properties["gpg"]

	if gpgExists {
		signatureFile := path.Join(dirName, "_"+fileName+".sig")

		err = DownloadFile(srv, signatureFile, &drive.File{Id: gpgFileId, Properties: make(map[string]string)})

		if err != nil {
			return fmt.Errorf("failed to download gpg signature: %v", err)
		}

		defer func() {
			os.Remove(signatureFile)
		}()

		cmd := exec.Command("gpg2", "--verify", signatureFile, tmpAddress)

		err = cmd.Run()

		if err != nil {
			return fmt.Errorf("failed to verify file")
		}
	}

	_, err = os.Stat(address)

	if err == nil {
		if err != nil {
			return fmt.Errorf("failed to delete original file: %v", err)
		}
	} else if os.IsExist(err) {
		return fmt.Errorf("failed to stat original file: %v", err)
	}

	err = os.Rename(tmpAddress, address)

	if err != nil {
		return fmt.Errorf("failed to rename temp file to original: %v", err)
	}

	// Mark the file as renamed, this will prevent delete attempt
	renamed = true

	modTime, err := time.Parse(time.RFC3339, file.ModifiedTime)

	if err != nil {
		return fmt.Errorf("failed to parse mod date '%s': %v", file.ModifiedTime, err)
	}

	fileInfo, err := os.Stat(address)

	if err != nil {
		return fmt.Errorf("failed to stat downloaded file: %v", address)
	}

	err = metadataStore.Set(address, metadata.FileMetadata{RemoteModDate: modTime, LocalModDate: fileInfo.ModTime()})

	if err != nil {
		return fmt.Errorf("failed to write file metadata: %v", err)
	}

	return nil
}

package gdrive

import (
	"fmt"
	"google.golang.org/api/drive/v3"
	"net/url"
)

const FolderMimeType = "application/vnd.google-apps.folder"

func GetOrCreateDirectory(srv *drive.Service, directoryName string) (string, error) {
	// Get parent directory
	fList, err := srv.Files.List().Q(fmt.Sprintf(
		"name='%s' and mimeType='%s'",
		url.QueryEscape(directoryName),
		FolderMimeType)).Fields("files(id, mimeType)").Do()

	if err != nil {
		return "", fmt.Errorf("failed to lookup sync directory: %v", err)
	}

	if len(fList.Files) == 0 {
		var folderFile *drive.File
		folderFile, err = srv.Files.Create(&drive.File{Name: directoryName, MimeType: FolderMimeType}).Do()

		if err != nil {
			return "", fmt.Errorf("failed to create sync folder: %v", err)
		}

		return folderFile.Id, nil
	} else {
		return fList.Files[0].Id, nil
	}
}

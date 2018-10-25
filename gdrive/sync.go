package gdrive

import (
	"fmt"
	"github.com/ilyail3/fileSync/cleanup"
	"github.com/ilyail3/fileSync/metadata"
	"google.golang.org/api/drive/v3"
	"log"
	"os"
	"path"
	"time"
)

func SyncFile(fullAddress string, parentId string, srv *drive.Service, mtStore metadata.Store, signKey string) error {
	fileName := path.Base(fullAddress)

	log.Printf("querying gdrive for file name:%s", fileName)

	queryFunction := ListFilesQuery(parentId, fileName)
	gpgQueryFunction := ListFilesQuery(parentId, fileName+".sig")

	r, err := queryFunction(srv, "").Do()

	if err != nil {
		return fmt.Errorf("unable to retrieve files: %v", err)
	}

	if len(r.Files) == 0 {
		log.Print("no files found, uploading")

		err = UploadFile(srv, fullAddress, parentId, mtStore, signKey)

		if err != nil {
			return fmt.Errorf("failed to upload file: %v", err)
		}
	} else {
		maxMTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		var maxFile *drive.File

		for _, i := range r.Files {
			//fmt.Printf("%s (%s) %s\n", i.Name, i.Id, i.ModifiedTime)

			mTime, err := time.Parse(time.RFC3339, i.ModifiedTime)

			if err != nil {
				return fmt.Errorf("failed to parse modified time from google '%s': %v", i.ModifiedTime, err)
			}

			if maxMTime.Before(mTime) {
				maxMTime = mTime
				maxFile = i
			}
		}

		exists, mt, err := mtStore.Get(fullAddress)

		if err != nil {
			return fmt.Errorf("failed to get mtstore metadata: %v", err)
		}

		fStat, statErr := os.Stat(fullAddress)

		if statErr != nil && os.IsExist(statErr) {
			return fmt.Errorf("failed to get stats for file: %v", err)
		}

		var download = false

		if os.IsNotExist(statErr) {
			log.Printf("local file missing, download")
			download = true
		} else {
			var modDate = mt.RemoteModDate

			if !exists {
				modDate = fStat.ModTime()
			}

			if modDate.Before(maxMTime) {
				log.Printf("local file(%s) is older than cloud, download cloud version(%s)",
					fStat.ModTime().UTC().Format(time.RFC3339),
					maxMTime.UTC().Format(time.RFC3339))

				download = true
			}

		}

		if download {
			err = TmpDownloadFile(srv, fullAddress, maxFile, mtStore)

			if err != nil {
				return fmt.Errorf("failed to download cloud version: %v", err)
			}
		} else {
			if !exists {
				log.Printf("no modified date for %s", fullAddress)
				blankDate, _ := time.Parse(time.RFC3339, "2000-01-01T00:00:00Z")
				mt.LocalModDate = blankDate
			}

			if fStat.ModTime().After(mt.LocalModDate.Add(time.Second)) {
				log.Printf(
					"local file %s is newer version %s",
					fStat.ModTime().UTC().Format(time.RFC3339),
					maxMTime.UTC().Format(time.RFC3339))

				err = UploadFile(srv, fullAddress, parentId, mtStore, signKey)

				if err != nil {
					return fmt.Errorf("failed to upload file: %v", err)
				}
			}
		}

		err = cleanup.PurgeOldFiles(srv, r, maxMTime, queryFunction, gpgQueryFunction)

		if err != nil {
			return fmt.Errorf("failed to purge old files: %v", err)
		}
	}

	return nil
}

package cleanup

import (
	"fmt"
	"github.com/emirpasic/gods/sets"
	"google.golang.org/api/drive/v3"
	"log"
	"time"
)

type FilesQuery func(srv *drive.Service, nextToken string) *drive.FilesListCall

func purgeOldGpgSignatures(srv *drive.Service, gpgSignatures sets.Set, gpgQueryFunction FilesQuery) error {

	var nextToken = ""

	for {
		r, err := gpgQueryFunction(srv, nextToken).Do()

		if err != nil {
			return fmt.Errorf("failed to query gpg files: %v", err)
		}

		for _, i := range r.Files {
			if !gpgSignatures.Contains(i.Id) {
				log.Printf("purging gpg signate %s from %s", i.Id, i.ModifiedTime)

				err = srv.Files.Delete(i.Id).Do()
				//log.Printf("delete signature:%s", i.Id)

				if err != nil {
					return fmt.Errorf("failed to delete file %s: %v", i.Id, err)
				}
			}
		}

		if r.NextPageToken == "" {
			return nil
		} else {
			nextToken = r.NextPageToken
		}
	}
}

func PurgeOldFiles(srv *drive.Service, r *drive.FileList, maxMTime time.Time, queryFunction FilesQuery, gpgQueryFunction FilesQuery, gpgSignatures sets.Set) error {
	if len(r.Files) > 0 {
		for {
			for _, i := range r.Files {
				mTime, err := time.Parse(time.RFC3339, i.ModifiedTime)

				if err != nil {
					return fmt.Errorf("failed to parse modified time from google '%s': %v", i.ModifiedTime, err)
				}

				if mTime.Before(maxMTime) {
					delta := time.Since(mTime)
					hours := int(delta.Hours())
					log.Printf("file %s is %d hours old", i.Id, hours)

					if hours > 24*10 {
						err := srv.Files.Delete(i.Id).Do()

						if err != nil {
							return fmt.Errorf("failed to cleanup old file: %v", err)
						}
					} else {
						gpg, exists := i.Properties["gpg"]

						if exists {
							gpgSignatures.Add(gpg)
						}
					}
				} else {
					gpg, exists := i.Properties["gpg"]

					if exists {
						gpgSignatures.Add(gpg)
					}
				}
			}

			if r.NextPageToken == "" {
				// log.Printf("gpg singatures: %d", gpgSignatures.Size())
				return purgeOldGpgSignatures(srv, gpgSignatures, gpgQueryFunction)
			} else {
				nextR, err := queryFunction(srv, r.NextPageToken).Do()

				if err != nil {
					return fmt.Errorf("failed to get next page: %v", err)
				}

				r = nextR
			}
		}
	} else {
		// log.Printf("gpg singatures: %d", gpgSignatures.Size())
		return purgeOldGpgSignatures(srv, gpgSignatures, gpgQueryFunction)
	}
}

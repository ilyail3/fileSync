package main

import (
	"encoding/json"
	"fmt"
	"github.com/ilyail3/fileSync/cleanup"
	"github.com/ilyail3/fileSync/metadata"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/kardianos/osext"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	"net/url"
	"path"
	"time"
)

const SIGN_KEY = "DCB47525"

// Retrieve a token, saves the token, then returns the generated client.
func getClient(dirName string, config *oauth2.Config) *http.Client {
	tokenFile := "token.json"
	tok, err := tokenFromFile(findPath(dirName, tokenFile))

	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok)
	}

	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	json.NewEncoder(f).Encode(token)
}

func SignFile(srv *drive.Service, address string) (bool, string, error) {
	// gpg2 --yes --sign-with DCB47525 --detach-sig $1
	cmd := exec.Command("gpg2", "--yes", "--sign-with", SIGN_KEY, "--detach-sig", address)
	err := cmd.Run()

	if err != nil {
		return false, "", fmt.Errorf("failed to sign file: %v", err)
	}

	signFile := address + ".sig"

	defer func() {
		os.Remove(signFile)
	}()

	fh, err := os.Open(signFile)

	if err != nil {
		return false, "", fmt.Errorf("failed to open signature file: %v", err)
	}

	defer fh.Close()

	f := drive.File{Name: path.Base(signFile)}
	resultFile, err := srv.Files.Create(&f).Media(fh).Do()

	if err != nil {
		return false, "", fmt.Errorf("failed to upload signature file: %v", err)
	}

	return true, resultFile.Id, nil
}

func UploadFile(srv *drive.Service, address string, metadataStore metadata.Store) error {
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

	signed, signatureFileId, err := SignFile(srv, address)

	if err != nil {
		return fmt.Errorf("failed to sign file: %v", err)
	}

	if signed {
		properties["gpg"] = signatureFileId
	}

	log.Printf("properties: %v", properties)

	modTime := time.Now()

	f := drive.File{Name: path.Base(address), Properties: properties, ModifiedTime: modTime.Format(time.RFC3339)}
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

func findPath(execDirName string, fileName string) string {
	execName := path.Join(execDirName, fileName)

	if _, err := os.Stat(execName); os.IsNotExist(err) {
		return fileName
	}

	return execName
}

func main() {
	execPath, err := osext.Executable()

	if err != nil {
		log.Fatalf("failed to get executable path")
	}

	dirName := path.Dir(execPath)
	log.Printf("dir is:%s", dirName)

	dbDir := path.Join(os.Getenv("HOME"), ".bin")
	mtStore, err := metadata.NewSQLite3Store(dbDir)

	if err != nil {
		log.Fatalf("Failed to open metadata store: %v", err)
	}

	defer mtStore.Close()

	b, err := ioutil.ReadFile(findPath(dirName, "credentials.json"))
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, drive.DriveFileScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(dirName, config)

	srv, err := drive.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Drive client: %v", err)
	}

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

			err = syncFile(fullAddress, srv, mtStore)

			if err != nil {
				log.Fatalf("failed to sync filename %s: %v", fullAddress, err)
			}
		}
	} else {
		fullAddress := os.Args[1]
		err = syncFile(fullAddress, srv, mtStore)

		if err != nil {
			log.Fatalf("failed to sync filename %s: %v", fullAddress, err)
		}
	}

}

func listFilesQuery(fileName string) cleanup.FilesQuery {
	return func(srv *drive.Service, nextToken string) *drive.FilesListCall {
		r := srv.Files.List().PageSize(10).
			Fields("nextPageToken, files(id, name, modifiedTime, properties)").
			Q(fmt.Sprintf("name='%s'", url.QueryEscape(fileName)))

		if nextToken != "" {
			r = r.PageToken(nextToken)
		}

		return r
	}
}

func syncFile(fullAddress string, srv *drive.Service, mtStore metadata.Store) error {
	fileName := path.Base(fullAddress)

	log.Printf("querying gdrive for file name:%s", fileName)

	queryFunction := listFilesQuery(fileName)
	gpgQueryFunction := listFilesQuery(fileName + ".sig")

	r, err := queryFunction(srv, "").Do()

	if err != nil {
		return fmt.Errorf("unable to retrieve files: %v", err)
	}

	if len(r.Files) == 0 {
		log.Print("no files found, uploading")

		err = UploadFile(srv, fullAddress, mtStore)

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

				err = UploadFile(srv, fullAddress, mtStore)

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

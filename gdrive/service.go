package gdrive

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(dirName string, config *oauth2.Config) (*http.Client, error) {
	tokenFile := "token.json"
	tok, err := tokenFromFile(findPath(dirName, tokenFile))

	if err != nil {
		tok = getTokenFromWeb(config)

		err = saveToken(tokenFile, tok)

		if err != nil {
			return nil, err
		}
	}

	return config.Client(context.Background(), tok), nil
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

	if err != nil {
		return nil, err
	}

	defer func() {
		err := f.Close()

		if err != nil {
			log.Printf("error closing token file: %v", err)
		}
	}()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)

	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %v", err)
	}

	defer func() {
		err := f.Close()

		if err != nil {
			log.Printf("failed to save token: %v", err)
		}
	}()

	err = json.NewEncoder(f).Encode(token)

	if err != nil {
		return fmt.Errorf("failed to encode json: %v", err)
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

func NewService(dirName string) (*drive.Service, error) {
	b, err := ioutil.ReadFile(findPath(dirName, "credentials.json"))
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, drive.DriveFileScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	client, err := getClient(dirName, config)

	if err != nil {
		return nil, fmt.Errorf("failed to get client: %v", err)
	}

	srv, err := drive.New(client)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve drive client: %v", err)
	}

	return srv, nil
}

package gdrive

import (
	"fmt"
	"github.com/ilyail3/fileSync/cleanup"
	"google.golang.org/api/drive/v3"
	"net/url"
)

func ListFilesQuery(parentId string, fileName string) cleanup.FilesQuery {
	return func(srv *drive.Service, nextToken string) *drive.FilesListCall {
		query := fmt.Sprintf(
			"name='%s' and parents in '%s'",
			url.QueryEscape(fileName),
			url.QueryEscape(parentId))

		r := srv.Files.List().PageSize(10).
			Fields("nextPageToken, files(id, name, modifiedTime, properties)").
			Q(query)

		if nextToken != "" {
			r = r.PageToken(nextToken)
		}

		return r
	}
}

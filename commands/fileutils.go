package commands

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

func FindFiles(root string, suffix string, recursive bool) ([]string, error) {
	rootInfo, statErr := os.Stat(root)
	if statErr != nil {
		return nil, statErr
	}

	if rootInfo.IsDir() {
		if recursive {
			var files []string
			walkErr := filepath.Walk(root, func(path string, entry os.FileInfo, err error) error {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
					files = append(files, path)
				}
				return nil
			})
			return files, walkErr
		} else {
			entries, readErr := ioutil.ReadDir(root)
			if readErr != nil {
				return nil, readErr
			}

			files := []string{}
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
					files = append(files, root+"/"+entry.Name())
				}
			}
			return files, nil
		}
	} else {
		return []string{root}, nil
	}
}

func ParseDeleteParams(path string) ([]services.DeleteParams, error) {
	reader, fileErr := os.Open(path)
	if fileErr != nil {
		return nil, fileErr
	}

	var specFiles spec.SpecFiles
	decodeErr := json.NewDecoder(reader).Decode(&specFiles)
	if fileErr != nil {
		return nil, decodeErr
	}

	deleteParams := []services.DeleteParams{}
	for _, file := range specFiles.Files {
		var (
			dp      services.DeleteParams
			castErr error
		)
		dp.CommonParams, castErr = file.ToCommonParams()
		if castErr != nil {
			return nil, castErr
		}

		deleteParams = append(deleteParams, dp)
	}

	return deleteParams, nil
}

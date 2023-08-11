package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/maps"
)

type ExpandConfiguration struct {
	configFile string
	outputPath string
}

type Policy struct {
	Template     string                   `json:"template"`
	DeleteParent bool                     `json:"deleteParent"`
	NameProperty string                   `json:"nameProperty"`
	Entries      []map[string]interface{} `json:"entries"`
}

type RepoPath struct {
	Repo string
	Path string
}

const deleteParentTemplateText = `
{
	"files": [
		{
			"aql": {
				"items.find": {
					"$or": [
{{ formatRepoPaths .RepoPaths }}
					]
				}
			}
		}
	]
}
`

var deleteParentTemplate *template.Template

func InitTemplates() {
	log.Info("Initializing built-in templates") //Debug?
	deleteParentTemplate = template.Must(template.New("deleteParent").Funcs(template.FuncMap{
		"formatRepoPaths": func(repoPaths []RepoPath) string {
			result := ""
			for i, repoPath := range repoPaths {
				result += fmt.Sprintf(`						{ "repo": "%s", "path": "%s" }`, repoPath.Repo, repoPath.Path)
				if i < len(repoPaths)-1 {
					result += ",\n"
				}
			}
			return result
		},
	}).Parse(deleteParentTemplateText))
}

func GetExpandCommand() components.Command {
	return components.Command{
		Name:        "expand",
		Description: "Expands retention templates",
		Aliases:     []string{},
		Arguments:   GetExpandArguments(),
		Flags:       GetExpandFlags(),
		EnvVars:     GetExpandEnvVar(),
		Action: func(c *components.Context) error {
			return ExpandCmd(c)
		},
	}
}

func GetExpandArguments() []components.Argument {
	return []components.Argument{
		{
			Name:        "config-file",
			Description: "Path to the JSON config file",
		},
		{
			Name:        "output-path",
			Description: "Path to output the generated filespecs",
		},
	}
}

func GetExpandFlags() []components.Flag {
	return []components.Flag{}
}

func GetExpandEnvVar() []components.EnvVar {
	return []components.EnvVar{}
}

func ExpandCmd(context *components.Context) error {
	expandConfig, argErr := ParseExpandConfig(context)
	if argErr != nil {
		return argErr
	}

	InitTemplates()

	log.Debug("expandConfig:")
	log.Debug("    configFile:", expandConfig.configFile)
	log.Debug("    outputPath:", expandConfig.outputPath)

	log.Info("Reading config file")
	configFile, readErr := os.ReadFile(expandConfig.configFile)
	if readErr != nil {
		return readErr
	}

	log.Info("Parsing config JSON")
	var config map[string]Policy
	if jsonErr := json.Unmarshal(configFile, &config); jsonErr != nil {
		return jsonErr
	}

	log.Info("Expanding policies")
	for policyName, policy := range config {
		log.Info("  [", policyName, "]")

		log.Debug("    template:", policy.Template)
		log.Debug("    nameProperty:", policy.NameProperty)
		log.Debug("    deleteParent:", policy.DeleteParent)
		log.Debug("    entries:", len(policy.Entries))

		// Find the policy's template
		templatePath := path.Join(filepath.Dir(expandConfig.configFile), policy.Template)
		log.Debug("    templatePath:", templatePath)

		// Read the policy's template
		templateBytes, readErr := os.ReadFile(templatePath)
		if readErr != nil {
			return readErr
		}

		// Prep template for parsing
		template, parseErr := template.New(policyName).Option("missingkey=error").Parse(string(templateBytes))
		if parseErr != nil {
			return parseErr
		}

		// Create output dir if necessary
		if dirErr := os.MkdirAll(path.Join(expandConfig.outputPath, policyName), 0755); dirErr != nil {
			return dirErr
		}

		// Iterate over policy entries
		for index, entry := range policy.Entries {
			// Figure out the file name for the result file
			fileName := fmt.Sprint(policyName, "-", index, ".json")

			// If the policy has a NameProperty, use that for the filename
			if policy.NameProperty != "" {
				if entry[policy.NameProperty] != "" {
					fileName = fmt.Sprint(entry[policy.NameProperty], "-", index, ".json")
				}
			}

			if !policy.DeleteParent {
				// Create the result file
				resultFile, fileErr := os.Create(path.Join(expandConfig.outputPath, policyName, fileName))
				if fileErr != nil {
					return fileErr
				}

				// Expand the template, dumping it into the result file
				if templatingErr := template.Execute(resultFile, entry); templatingErr != nil {
					return templatingErr
				}

				log.Debug("    Expanded: ", resultFile.Name())
			} else {
				// For DeleteParent policies, we'll ultimately generate a FileSpec to match the parent paths of whatever matches the policy.
				// To do this, we'll expand the template into a temp file, and use it to search for matches.
				// Then, we generate the final FileSpec that matches the parent paths of whatever we found.

				// Create a temp dir to store the search FileSpec
				tmpDir, tmpErr := ioutil.TempDir(expandConfig.outputPath, "rt-retention-*")
				if tmpErr != nil {
					return tmpErr
				}
				defer os.RemoveAll(tmpDir)

				// Create and expand the template into a temp file
				tempFile, fileErr := os.Create(path.Join(tmpDir, fileName))
				if fileErr != nil {
					return fileErr
				}
				if templatingErr := template.Execute(tempFile, entry); templatingErr != nil {
					return templatingErr
				}

				searchParams, parseErr := ParseSearchParamsFromPath(path.Join(tmpDir, fileName))
				if parseErr != nil {
					return parseErr
				}

				artifactoryManager, rtfErr := GetArtifactoryManager(context, true)
				if rtfErr != nil {
					return rtfErr
				}

				// Search for and collect matches' parent paths
				var repoPaths []RepoPath
				for _, sp := range searchParams {
					reader, searchErr := artifactoryManager.SearchFiles(sp)
					if searchErr != nil {
						return searchErr
					}

					defer func() {
						if reader != nil {
							reader.Close()
						}
					}()

					pathMap := make(map[RepoPath]struct{}) // Map to avoid duplicates in the parent paths
					for currentResult := new(utils.ResultItem); reader.NextRecord(currentResult) == nil; currentResult = new(utils.ResultItem) {
						repoPath := RepoPath{Repo: currentResult.Repo, Path: currentResult.Path}
						pathMap[repoPath] = struct{}{}
					}
					repoPaths = maps.Keys(pathMap)

					log.Debug("    Parent paths for [", policyName, "] :")
					for _, repoPath := range repoPaths {
						log.Debug("      -", repoPath.Repo, "-", repoPath.Path)
					}

					if readErr := reader.GetError(); readErr != nil {
						return readErr
					}
				}

				if len(repoPaths) <= 0 {
					log.Debug("No parent paths found, skipping generating")
					continue
				}

				// Create the result file
				resultFile, fileErr := os.Create(path.Join(expandConfig.outputPath, policyName, fileName))
				if fileErr != nil {
					return fileErr
				}

				// Expand the template, dumping it into the result file
				data := struct {
					RepoPaths []RepoPath
				}{
					RepoPaths: repoPaths,
				}
				if templatingErr := deleteParentTemplate.Execute(resultFile, data); templatingErr != nil {
					return templatingErr
				}

				log.Debug("    Expanded: ", resultFile.Name())
			}
		}
	}

	log.Info("Done")
	return nil
}

func ParseExpandConfig(context *components.Context) (*ExpandConfiguration, error) {
	if len(context.Arguments) != 2 {
		return nil, errors.New("Expected 2 arguments, received " + strconv.Itoa(len(context.Arguments)))
	}

	var expandConfig = new(ExpandConfiguration)
	expandConfig.configFile = context.Arguments[0]
	expandConfig.outputPath = context.Arguments[1]

	return expandConfig, nil
}

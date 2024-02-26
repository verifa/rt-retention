package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ExpandConfiguration struct {
	configFile string
	outputPath string
	threads    int
}

type Policy struct {
	Template     string                   `json:"template"`
	DeleteParent bool                     `json:"deleteParent"`
	NameProperty string                   `json:"nameProperty"`
	Entries      []map[string]interface{} `json:"entries"`
}

const deleteParentTemplateText = `
{
	"files": [
		{
			"aql": {
				"items.find": {
					"repo": "{{ .Repo }}",
					"$or": [
{{ formatPaths .Paths }}
					]
				}
			}
		}
	]
}
`

var deleteParentTemplate *template.Template

func InitTemplates() {
	log.Info("Initializing built-in templates")
	deleteParentTemplate = template.Must(template.New("deleteParent").Funcs(template.FuncMap{
		"formatPaths": func(paths []string) string {
			result := ""
			for i, path := range paths {
				result += fmt.Sprintf(`						{ "path": "%s" }`, path)
				if i < len(paths)-1 {
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
	return []components.Flag{
		components.StringFlag{
			BaseFlag: components.BaseFlag{
				Name:        "threads",
				Description: "Number of worker threads",
			},
			DefaultValue: "3",
			Mandatory:    false,
		},
	}
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
		policyPath := path.Join(expandConfig.outputPath, policyName)
		if dirErr := os.MkdirAll(policyPath, 0755); dirErr != nil {
			return dirErr
		}

		// Iterate over policy entries
		for index, entry := range policy.Entries {
			if !policy.DeleteParent {
				// Figure out the file name for the result file
				fileName := fmt.Sprintf("%s-%d.json", policyName, index)
				if policy.NameProperty != "" && entry[policy.NameProperty] != "" {
					fileName = fmt.Sprintf("%s-%d.json", entry[policy.NameProperty], index)
				}

				// Create the result file
				resultFile, fileErr := os.Create(path.Join(policyPath, fileName))
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
				tmpDir, tmpErr := os.MkdirTemp(expandConfig.outputPath, "rt-retention-*")
				if tmpErr != nil {
					return tmpErr
				}
				defer os.RemoveAll(tmpDir)

				// Create and expand the template into a temp file
				tempFile, fileErr := os.Create(path.Join(tmpDir, policyName))
				if fileErr != nil {
					return fileErr
				}
				if templatingErr := template.Execute(tempFile, entry); templatingErr != nil {
					return templatingErr
				}

				searchParams, parseErr := ParseSearchParamsFromPath(path.Join(tmpDir, policyName))
				if parseErr != nil {
					return parseErr
				}

				artifactoryManager, rtfErr := GetArtifactoryManager(context, true, expandConfig.threads)
				if rtfErr != nil {
					return rtfErr
				}

				// Search for and collect matches' parent paths
				repoPaths := make(map[string][]string)
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

					for result := new(utils.ResultItem); reader.NextRecord(result) == nil; result = new(utils.ResultItem) {
						if _, exists := repoPaths[result.Repo]; !exists {
							repoPaths[result.Repo] = []string{}
						}
						repoPaths[result.Repo] = append(repoPaths[result.Repo], result.Path)
					}

					log.Debug("    Parent paths for [", policyName, "] :")
					for repo, paths := range repoPaths {
						log.Debug("      -", repo)
						for _, path := range paths {
							log.Debug("          -", path)
						}
					}

					if readErr := reader.GetError(); readErr != nil {
						return readErr
					}
				}

				// Write File Specs for each matched repository
				for repo, paths := range repoPaths {
					log.Debug("Generating for", repo)
					if len(paths) <= 0 {
						log.Debug("No parent paths found, skipping generating")
						continue
					}

					// Figure out the file name for the result file
					fileName := fmt.Sprintf("%s-%d.json", policyName, index)
					if policy.NameProperty != "" && entry[policy.NameProperty] != "" {
						fileName = fmt.Sprintf("%s-%d.json", entry[policy.NameProperty], index)
					}

					// Create output dir if necessary
					log.Debug("DEBUG", path.Join(policyPath, repo))
					if dirErr := os.MkdirAll(path.Join(policyPath, repo), 0755); dirErr != nil {
						return dirErr
					}

					// Create the result file
					resultFile, fileErr := os.Create(path.Join(policyPath, repo, fileName))
					if fileErr != nil {
						return fileErr
					}

					// Expand the template, dumping it into the result file
					data := struct {
						Repo  string
						Paths []string
					}{
						Repo:  repo,
						Paths: paths,
					}
					if templatingErr := deleteParentTemplate.Execute(resultFile, data); templatingErr != nil {
						return templatingErr
					}

					log.Debug("    Expanded: ", resultFile.Name())

				}
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
	threads, err := strconv.Atoi(context.GetStringFlagValue("threads"))
	if err != nil {
		return nil, err
	}
	expandConfig.threads = threads

	return expandConfig, nil
}

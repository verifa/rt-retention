package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path"
	"strconv"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ExpandConfiguration struct {
	configPath    string
	templatesPath string
	outputPath    string
	recursive     bool
	verbose       bool
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
			Name:        "config-path",
			Description: "Path to the JSON config file",
		},
		{
			Name:        "templates-path",
			Description: "Path to the templates dir",
		},
		{
			Name:        "output-path",
			Description: "Path to output the generated filespecs",
		},
	}
}

func GetExpandFlags() []components.Flag {
	return []components.Flag{
		components.BoolFlag{
			Name:         "verbose",
			Description:  "output verbose logging",
			DefaultValue: false,
		},
		components.BoolFlag{
			Name:         "recursive",
			Description:  "recursively find templates in the given dir",
			DefaultValue: false,
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

	if expandConfig.verbose {
		log.Info("expandConfig:")
		log.Info("    configPath:", expandConfig.configPath)
		log.Info("    templatesPath:", expandConfig.templatesPath)
		log.Info("    outputPath:", expandConfig.outputPath)
		log.Info("    recursive:", expandConfig.recursive)
		log.Info("    verbose:", expandConfig.verbose)
	}

	log.Info("Collecting template files")
	templateFiles, findErr := FindFiles(expandConfig.templatesPath, ".json", expandConfig.recursive)
	if findErr != nil {
		return findErr
	}

	if len(templateFiles) == 0 {
		log.Warn("Found no JSON files")
	} else {
		log.Info("Found", len(templateFiles), "JSON files")
	}

	if expandConfig.verbose {
		for _, file := range templateFiles {
			log.Info("    " + file)
		}
	}

	log.Info("Parsing config file")
	configFile, readErr := os.ReadFile(expandConfig.configPath)
	if readErr != nil {
		return readErr
	}

	var config map[string][]map[string]interface{}
	if jsonErr := json.Unmarshal(configFile, &config); jsonErr != nil {
		return jsonErr
	}

	for templateName, entries := range config {
		log.Info("Expanding", templateName)

		templateText, readErr := os.ReadFile(path.Join(expandConfig.templatesPath, templateName+".json"))
		if readErr != nil {
			return readErr
		}

		template, parseErr := template.New(templateName).Option("missingkey=error").Parse(string(templateText))
		if parseErr != nil {
			return parseErr
		}

		if dirErr := os.MkdirAll(path.Join(expandConfig.outputPath, templateName), 0755); dirErr != nil {
			return dirErr
		}

		for index, entry := range entries {
			var fileName string
			if name, hasName := entry["Name"]; hasName {
				fileName = fmt.Sprint(name, ".json")
			} else {
				fileName = fmt.Sprint(templateName, "-", index, ".json")
			}

			resultFile, fileErr := os.Create(path.Join(expandConfig.outputPath, templateName, fileName))
			if fileErr != nil {
				return fileErr
			}

			if templatingErr := template.Execute(resultFile, entry); templatingErr != nil {
				return templatingErr
			}

			if expandConfig.verbose {
				log.Info("    ", resultFile.Name())
			}
		}
	}

	log.Info("Done")
	return nil
}

func ParseExpandConfig(context *components.Context) (*ExpandConfiguration, error) {
	if len(context.Arguments) != 3 {
		return nil, errors.New("Expected 3 argument, received " + strconv.Itoa(len(context.Arguments)))
	}

	var expandConfig = new(ExpandConfiguration)
	expandConfig.configPath = context.Arguments[0]
	expandConfig.templatesPath = context.Arguments[1]
	expandConfig.outputPath = context.Arguments[2]
	expandConfig.recursive = context.GetBoolFlagValue("recursive")
	expandConfig.verbose = context.GetBoolFlagValue("verbose")

	return expandConfig, nil
}

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
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ExpandConfiguration struct {
	configFile string
	outputPath string
	verbose    bool
}

type Policy struct {
	Template     string                   `json:"template"`
	DeleteParent bool                     `json:"deleteParent"`
	NameProperty string                   `json:"nameProperty"`
	Entries      []map[string]interface{} `json:"entries"`
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
		components.BoolFlag{
			Name:         "verbose",
			Description:  "output verbose logging",
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
		log.Info("    configFile:", expandConfig.configFile)
		log.Info("    outputPath:", expandConfig.outputPath)
		log.Info("    verbose:", expandConfig.verbose)
	}

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

		if expandConfig.verbose {
			log.Info("    template:", policy.Template)
			log.Info("    nameProperty:", policy.NameProperty)
			log.Info("    entries:", len(policy.Entries))
		}

		// Find the policy's template
		templatePath := path.Join(filepath.Dir(expandConfig.configFile), policy.Template)
		if expandConfig.verbose {
			log.Info("    template:", templatePath)
		}

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

			// Create the result file
			resultFile, fileErr := os.Create(path.Join(expandConfig.outputPath, policyName, fileName))
			if fileErr != nil {
				return fileErr
			}

			// Expand the template, dumping it into the result file
			if templatingErr := template.Execute(resultFile, entry); templatingErr != nil {
				return templatingErr
			}

			if expandConfig.verbose {
				log.Info("     -", resultFile.Name())
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
	expandConfig.verbose = context.GetBoolFlagValue("verbose")

	return expandConfig, nil
}

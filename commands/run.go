package commands

import (
	"errors"
	"io/ioutil"
	"strconv"

	core_commands "github.com/jfrog/jfrog-cli-core/v2/common/commands"
	core_components "github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	core_config "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	core_utils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	client_artifactory "github.com/jfrog/jfrog-client-go/artifactory"
	client_services "github.com/jfrog/jfrog-client-go/artifactory/services"
	client_serviceutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	client_utils "github.com/jfrog/jfrog-client-go/utils"
	client_log "github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/praqma-thi/jfrog-cli-retention-plugin/config"
)

type runConfiguration struct {
	configFile string
	dryRun bool
	verbose bool
}

func GetRunCommand() core_components.Command {
	return core_components.Command{
		Name:        "run",
		Description: "Runs retention",
		Aliases:     []string{},
		Arguments:   getRunArguments(),
		Flags:       getRunFlags(),
		EnvVars:     getRunEnvVar(),
		Action: func(c *core_components.Context) error {
			return runCmd(c)
		},
	}
}

func getRunArguments() []core_components.Argument {
	return []core_components.Argument{
		{
			Name:        "config-file",
			Description: "Path to the retention configuration file",
		},
	}
}

func getRunFlags() []core_components.Flag {
	return []core_components.Flag{
		core_components.BoolFlag{
			Name:         "dry-run",
			Description:  "Set to true to disable communication with Artifactory",
			DefaultValue: false,
		},
		core_components.BoolFlag{
			Name:         "verbose",
			Description:  "Set to true to output more verbose logging",
			DefaultValue: false,
		},
	}
}

func getRunEnvVar() []core_components.EnvVar {
	return []core_components.EnvVar{
		{},
	}
}

func parseRunConfig(context *core_components.Context) (*runConfiguration, error) {
	if len(context.Arguments) != 1 {
		return nil, errors.New("Expected 1 argument, received" + strconv.Itoa(len(context.Arguments)))
	}

	var runConfig = new(runConfiguration)
	runConfig.configFile = context.Arguments[0]
	runConfig.dryRun = context.GetBoolFlagValue("dry-run")
	runConfig.verbose = context.GetBoolFlagValue("verbose")

	return runConfig, nil
}

func runCmd(context *core_components.Context) error {
	runConfig, err := parseRunConfig(context)
	if err != nil {
		return err
	}

	if (runConfig.verbose) {
		client_log.Info("runConfig:")
		client_log.Info("    configFile:", runConfig.configFile)
		client_log.Info("    dryRun:", runConfig.dryRun)
		client_log.Info("    verbose:", runConfig.verbose)
	}

	client_log.Info("Fetching Artifactory details")
	artifactoryDetails, err := getArtifactoryDetails(context)
	if err != nil {
		return err
	}

	client_log.Info("Configuring Artifactory manager")
	artifactoryManager, err := core_utils.CreateServiceManager(artifactoryDetails, -1, runConfig.dryRun)
	if err != nil {
		return err
	}

	client_log.Info("Parsing retention configuration")
	retentionConfiguration := config.ParseRetentionConfiguration(runConfig.configFile)

	if (runConfig.verbose) {
		client_log.Info("retentionConfiguration:")
		client_log.Info("    Artifact:", len(retentionConfiguration.Artifact))
		for _, artifactRetention := range retentionConfiguration.Artifact {
			client_log.Info("        - ", artifactRetention.Name)
			client_log.Info("            - Limit", artifactRetention.Limit)
			client_log.Info("            - Offset", artifactRetention.Offset)
			client_log.Info("            - SortBy", artifactRetention.SortBy)
			client_log.Info("            - SortOrder", artifactRetention.SortOrder)
		}
	}

	client_log.Info("Executing",  len(retentionConfiguration.Artifact), "artifact retention policies")
	if err = runArtifactRetention(artifactoryManager, retentionConfiguration.Artifact); err != nil {
		return err
	}

	client_log.Info("Done")
	return nil
}

func runArtifactRetention(artifactoryManager client_artifactory.ArtifactoryServicesManager, artifactRetentions []config.Artifact) error {
	for i, artifactRetention := range artifactRetentions {
		client_log.Info(i + 1, "/", len(artifactRetentions), ":", artifactRetention.Name)
		aqlQuery, err := ioutil.ReadFile(artifactRetention.AqlPath)
		if err != nil {
			return err
		}

		params := client_services.NewDeleteParams()
		params.Offset = artifactRetention.Offset
		params.Limit = artifactRetention.Limit
		params.SortOrder = artifactRetention.SortOrder
		params.SortBy = artifactRetention.SortBy
		params.Aql = client_serviceutils.Aql{
			ItemsFind: string(aqlQuery),
		}
		params.Recursive = true

		pathsToDelete, err := artifactoryManager.GetPathsToDelete(params)
		if err != nil {
			return err
		}
		defer pathsToDelete.Close()

		artifactoryManager.DeleteFiles(pathsToDelete)
	}

	return nil
}

func getArtifactoryDetails(c *core_components.Context) (*core_config.ServerDetails, error) {
	details, err := core_commands.GetConfig("", false)
	if err != nil {
		return nil, err
	}

	if details.Url == "" {
		return nil, errors.New("no server-id was found, or the server-id has no url")
	}

	details.Url = client_utils.AddTrailingSlashIfNeeded(details.Url)
	err = core_config.CreateInitialRefreshableTokensIfNeeded(details)
	if err != nil {
		return nil, err
	}

	return details, nil
}

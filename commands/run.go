package commands

import (
	"errors"
	"strconv"

	core_utils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	client_utils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type RunConfiguration struct {
	fileSpecsPath string
	dryRun        bool
	recursive     bool
	verbose       bool
}

func GetRunCommand() components.Command {
	return components.Command{
		Name:        "run",
		Description: "Runs retention",
		Aliases:     []string{},
		Arguments:   GetRunArguments(),
		Flags:       GetRunFlags(),
		EnvVars:     GetRunEnvVar(),
		Action: func(c *components.Context) error {
			return RunCmd(c)
		},
	}
}

func GetRunArguments() []components.Argument {
	return []components.Argument{
		{
			Name:        "filespecs-path",
			Description: "Path to the filespecs file/dir",
		},
	}
}

func GetRunFlags() []components.Flag {
	return []components.Flag{
		components.BoolFlag{
			Name:         "dry-run",
			Description:  "disable deletion of artifacts",
			DefaultValue: true,
		},
		components.BoolFlag{
			Name:         "verbose",
			Description:  "output verbose logging",
			DefaultValue: false,
		},
		components.BoolFlag{
			Name:         "recursive",
			Description:  "recursively find filespecs files in the given dir",
			DefaultValue: false,
		},
	}
}

func GetRunEnvVar() []components.EnvVar {
	return []components.EnvVar{}
}

func RunCmd(context *components.Context) error {
	runConfig, argErr := ParseRunConfig(context)
	if argErr != nil {
		return argErr
	}

	if runConfig.verbose {
		log.Info("runConfig:")
		log.Info("    fileSpecsPath:", runConfig.fileSpecsPath)
		log.Info("    dryRun:", runConfig.dryRun)
		log.Info("    recursive:", runConfig.recursive)
		log.Info("    verbose:", runConfig.verbose)
	}

	log.Info("Fetching Artifactory details")
	artifactoryDetails, cfgErr := GetArtifactoryDetails(context, runConfig)
	if cfgErr != nil {
		return cfgErr
	}

	log.Info("Configuring Artifactory manager")
	artifactoryManager, rtfErr := core_utils.CreateServiceManager(artifactoryDetails, 3, 5000, runConfig.dryRun)
	if rtfErr != nil {
		return rtfErr
	}

	log.Info("Collecting retention files")
	fileSpecsFiles, findErr := FindFiles(runConfig.fileSpecsPath, ".json", runConfig.recursive)
	if findErr != nil {
		return findErr
	}

	if len(fileSpecsFiles) == 0 {
		log.Warn("Found no JSON files")
	} else {
		log.Info("Found", len(fileSpecsFiles), "JSON files")
	}

	if runConfig.verbose {
		for _, file := range fileSpecsFiles {
			log.Info("    " + file)
		}
	}

	if retentionErr := RunArtifactRetention(artifactoryManager, fileSpecsFiles); retentionErr != nil {
		return retentionErr
	}

	log.Info("Done")
	return nil
}

func ParseRunConfig(context *components.Context) (*RunConfiguration, error) {
	if len(context.Arguments) != 1 {
		return nil, errors.New("Expected 1 argument, received " + strconv.Itoa(len(context.Arguments)))
	}

	var runConfig = new(RunConfiguration)
	runConfig.fileSpecsPath = context.Arguments[0]
	runConfig.dryRun = context.GetBoolFlagValue("dry-run")
	runConfig.recursive = context.GetBoolFlagValue("recursive")
	runConfig.verbose = context.GetBoolFlagValue("verbose")

	return runConfig, nil
}

func GetArtifactoryDetails(context *components.Context, runConfig *RunConfiguration) (*config.ServerDetails, error) {
	if runConfig.verbose {
		var servers = commands.GetAllServerIds()
		log.Info("Server IDs: ", len(servers))
		for _, server := range servers {
			log.Info("\t", server)
		}
	}

	details, cfgErr := config.GetDefaultServerConf()
	if cfgErr != nil {
		return nil, cfgErr
	}

	if runConfig.verbose {
		log.Info("Default server ID:")
		log.Info("\t", details.ServerId, "(", details.ArtifactoryUrl, ")")
	}

	if details.ArtifactoryUrl == "" {
		return nil, errors.New("no server-id was found, or the server-id has no url")
	}

	details.ArtifactoryUrl = client_utils.AddTrailingSlashIfNeeded(details.ArtifactoryUrl)
	if tokenErr := config.CreateInitialRefreshableTokensIfNeeded(details); tokenErr != nil {
		return nil, tokenErr
	}

	return details, nil
}

func RunArtifactRetention(artifactoryManager artifactory.ArtifactoryServicesManager, fileSpecsFiles []string) error {
	runErrors := []string{}
	totalFiles := len(fileSpecsFiles)
	for i, file := range fileSpecsFiles {
		log.Info(i+1, "/", totalFiles, ":", file)

		deleteParams, parseErr := ParseDeleteParams(file)
		if parseErr != nil {
			var err = "ParseDeleteParams failed for file: " + file + "\n" + parseErr.Error()
			runErrors = append(runErrors, err)
			log.Error(err)
			continue
		}

		for _, dp := range deleteParams {
			pathsToDelete, pathsErr := artifactoryManager.GetPathsToDelete(dp)
			if pathsErr != nil {
				var err = "GetPathsToDelete failed for file: " + file + "\n" + pathsErr.Error()
				runErrors = append(runErrors, err)
				log.Error(err)
				continue
			}
			defer pathsToDelete.Close()

			if _, delErr := artifactoryManager.DeleteFiles(pathsToDelete); delErr != nil {
				var err = "DeleteFiles failed for file: " + file + "\n" + delErr.Error()
				runErrors = append(runErrors, err)
				log.Error(err)
				continue
			}
		}
	}

	if len(runErrors) == 0 {
		return nil
	} else {
		summary := "The following errors occured during the run:\n"
		for _, err := range runErrors {
			summary += err + "\n"
		}
		return errors.New(summary)
	}
}

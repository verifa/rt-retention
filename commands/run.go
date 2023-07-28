package commands

import (
	"errors"
	"strconv"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/artifactory"
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
			Description:  "do not delete artifacts",
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

	log.Info("Configuring Artifactory manager")
	artifactoryManager, rtfErr := GetArtifactoryManager(context, runConfig.dryRun, runConfig.verbose)
	if rtfErr != nil {
		return rtfErr
	}

	log.Info("Collecting retention files")
	fileSpecsPaths, findErr := FindFiles(runConfig.fileSpecsPath, ".json", runConfig.recursive)
	if findErr != nil {
		return findErr
	}

	if len(fileSpecsPaths) == 0 {
		log.Warn("Found no JSON files")
	} else {
		log.Info("Found", len(fileSpecsPaths), "JSON files")
	}

	if runConfig.verbose {
		for _, file := range fileSpecsPaths {
			log.Info("    " + file)
		}
	}

	if retentionErr := RunArtifactRetention(artifactoryManager, fileSpecsPaths); retentionErr != nil {
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

func RunArtifactRetention(artifactoryManager artifactory.ArtifactoryServicesManager, fileSpecsPaths []string) error {
	runErrors := []string{}
	totalPaths := len(fileSpecsPaths)
	for i, path := range fileSpecsPaths {
		log.Info(i+1, "/", totalPaths, ":", path)

		deleteParams, parseErr := ParseDeleteParamsFromPath(path)
		if parseErr != nil {
			var err = "ParseDeleteParamsFromPath failed for path: " + path + "\n" + parseErr.Error()
			runErrors = append(runErrors, err)
			log.Error(err)
			continue
		}

		for _, dp := range deleteParams {
			pathsToDelete, pathsErr := artifactoryManager.GetPathsToDelete(dp)
			if pathsErr != nil {
				var err = "GetPathsToDelete failed for path: " + path + "\n" + pathsErr.Error()
				runErrors = append(runErrors, err)
				log.Error(err)
				continue
			}
			defer pathsToDelete.Close()

			if _, delErr := artifactoryManager.DeleteFiles(pathsToDelete); delErr != nil {
				var err = "DeleteFiles failed for path: " + path + "\n" + delErr.Error()
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

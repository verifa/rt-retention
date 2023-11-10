package commands

import (
	"errors"

	core_utils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	client_utils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func GetArtifactoryDetails(context *components.Context) (*config.ServerDetails, error) {
	log.Info("Fetching Artifactory details")

	var servers = commands.GetAllServerIds()
	log.Debug("Server IDs: ", len(servers))
	for _, server := range servers {
		log.Debug("\t", server)
	}

	details, cfgErr := config.GetDefaultServerConf()
	if cfgErr != nil {
		return nil, cfgErr
	}

	log.Debug("Default server ID:")
	log.Debug("\t", details.ServerId, "(", details.ArtifactoryUrl, ")")

	if details.ArtifactoryUrl == "" {
		return nil, errors.New("no server-id was found, or the server-id has no url")
	}

	details.ArtifactoryUrl = client_utils.AddTrailingSlashIfNeeded(details.ArtifactoryUrl)
	if tokenErr := config.CreateInitialRefreshableTokensIfNeeded(details); tokenErr != nil {
		return nil, tokenErr
	}

	return details, nil
}

func GetArtifactoryManager(context *components.Context, dryRun bool, threads int) (artifactory.ArtifactoryServicesManager, error) {
	artifactoryDetails, cfgErr := GetArtifactoryDetails(context)
	if cfgErr != nil {
		return nil, cfgErr
	}

	log.Info("Configuring Artifactory manager")
	artifactoryManager, rtfErr := core_utils.CreateServiceManagerWithThreads(artifactoryDetails, dryRun, threads, 3, 5000)
	if rtfErr != nil {
		return nil, rtfErr
	}

	return artifactoryManager, nil
}

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

func GetArtifactoryDetails(context *components.Context, verbose bool) (*config.ServerDetails, error) {
	log.Info("Fetching Artifactory details")

	if verbose {
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

	if verbose {
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

func GetArtifactoryManager(context *components.Context, dryRun bool, verbose bool) (artifactory.ArtifactoryServicesManager, error) {
	artifactoryDetails, cfgErr := GetArtifactoryDetails(context, verbose)
	if cfgErr != nil {
		return nil, cfgErr
	}

	log.Info("Configuring Artifactory manager")
	artifactoryManager, rtfErr := core_utils.CreateServiceManager(artifactoryDetails, 3, 5000, dryRun)
	if rtfErr != nil {
		return nil, rtfErr
	}

	return artifactoryManager, nil
}

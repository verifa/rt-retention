package main

import (
	"github.com/jfrog/jfrog-cli-core/v2/plugins"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/verifa/rt-retention/commands"
)

var version = "dev"

func main() {
	plugins.PluginMain(getApp())
}

func getApp() components.App {
	app := components.App{}
	app.Name = "rt-retention"
	app.Description = "Enforce retention policies"
	app.Version = version
	app.Commands = getCommands()
	return app
}

func getCommands() []components.Command {
	return []components.Command{
		commands.GetRunCommand(),
		commands.GetExpandCommand(),
	}
}

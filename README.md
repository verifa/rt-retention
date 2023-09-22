# rt-retention

`rt-retention` is a simple JFrog CLI plugin to facilitate enforcing retention policies in Artifactory.

## How it works

`rt-retention` deletes artifacts matching [FileSpecs](https://www.jfrog.com/confluence/display/JFROG/Using+File+Specs) in a given directory.
It also has templating capabilities to help maintain similar retention policies.

To set up your retention policies, define them as [FileSpecs](https://www.jfrog.com/confluence/display/JFROG/Using+File+Specs) (or templates thereof).
To enforce them, set up a humble cron job running the plugin.

## Installing rt-retention

### docker

- `docker pull verifa/rt-retention`

### manual

- Download the latest version from the [Releases](https://github.com/verifa/rt-retention/releases) page
- Place its contents in your `~/.jfrog/plugins` directory
  (If there’s no `plugins` directory under `.jfrog`, create it)

## Running rt-retention

### commands

#### run

- Usage: `jf rt-retention run [command options] <filespecs-path>`

- Arguments:
  - filespecs-path    _(Path to the FileSpecs file/dir)_

- Options:
  - --dry-run    _do not delete artifacts [Default: **true**]_
  - --recursive    _recursively find FileSpecs files in the given dir [Default: false]_

#### expand

- Usage: `jf rt-retention expand [command options] <config-path> <templates-path> <output-path>`

- Arguments:
  - config-path    _(Path to the JSON config file)_
  - output-path    _(Path to output the generated FileSpecs)_

### running with verbose output

For verbose output to aid in debugging, set `JFROG_CLI_LOG_LEVEL=DEBUG`.

## Templating

The [`expand`](#expand) command can generate FileSpecs from Go templates, populated with values from a JSON config file.

The JSON config file may contain one or more policy definitions.
Each specifies the template file it uses, as well as a list of entries.
The template file path should be relative to the config file.
Each entry will result in a FileSpecs file being generated.

_Example `config.json`:_

```json
{
    "my-junk-repositories": {
        "template": "templates/entire-repo.json",
        "entries": [
            { "Repo": "scratch-local" }
        ]
    },
    "my-dev-repositories": {
        "template": "templates/older-than.json",
        "entries": [
            { "Repo": "generic-dev-local", "Time": "3w" },
            { "Repo": "libs-snapshot-local", "Time": "1y" }
        ]
    }
}
```

The templates themselves are [Go text templates](https://pkg.go.dev/text/template).
Properties from the JSON config entry will be used to populate the template.

_Example `templates/delete-older-than.json`:_

```json
{
  "files": [
    {
      "aql": {
        "items.find": {
          "repo": "{{.Repo}}",
          "created": {
            "$before": "{{.Time}}"
          }
        }
      }
    }
  ]
}
```

Pass the config file, the templates directory and the output directory to the [`expand`](#expand) command to generate the retention policies.

```bash
$ jf rt-retention expand config.json templates/ policies/
[🔵Info] Reading config file
[🔵Info] Parsing config JSON
[🔵Info] Expanding policies
[🔵Info]   [ my-junk-repositories ]
[🔵Info]   [ my-dev-repositories ]
[🔵Info] Done
```

### Extra templating properties

#### deleteParent

Policies can set `deleteParent` to delete the _parent paths_ of what the FileSpecs would match, rather than the matches themselves.

This is useful for deleting entire directories if they contain an artifact matching certain conditions, or deleting Docker images based on conditions on their manifest file.

```json
{
    "template-one": {
        "template": "templates/template.json",
        "deleteParent": true,
        "entries": [
            { "Repo": "scratch-local" }
        ]
    }
}
```

#### nameProperty

Policies can optionally set a `nameProperty`, which can be used to change the generated FileSpecs' filename to the value of the given property key.
Without it, FileSpecs are generated using the name of the template, and the index of their entry.

The below example uses the `Repo` property value to use as the FileSpecs' filename.

```json
{
    "template-one": {
        "template": "templates/template.json",
        "nameProperty": "Repo",
        "entries": [
            { "Repo": "scratch-local" },
            { "Repo": "dev-local" }
        ]
    }
}
```

Expanding the templates will result in the following generated files:

```text
output/
 `- template-one/
     |- scratch-local-0.json
     `- dev-local-0.json
```

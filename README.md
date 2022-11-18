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
    - --verbose    _output verbose logging [Default: false]_
    - --recursive    _recursively find FileSpecs files in the given dir [Default: false]_

#### expand

  - Usage: `jf rt-retention expand [command options] <config-path> <templates-path> <output-path>`
  
  - Arguments:
    - config-path    _(Path to the JSON config file)_
    - templates-path    _(Path to the templates dir)_
    - output-path    _(Path to output the generated FileSpecs)_

  - Options:
    - --verbose      _output verbose logging [Default: false]_
    - --recursive    _recursively find templates in the given dir [Default: false]_

## Templating

The [`expand`](#expand) command can generate FileSpecs from Go templates, populated with values from a JSON config file.

The JSON config file contains a key for each template with an array of entries.
The keys should match the template file names (without the `.json` extension). 
Each entry will result in a FileSpecs file being generated.

_Example `config.json`:_
```json
{
  "delete-everything": [
    {
      "Repo": "generic-tmp-local"
    }
  ],
  "delete-older-than": [
    {
      "Repo": "generic-dev-local",
      "Time": "14d"
    },
    {
      "Repo": "generic-rc-local",
      "Time": "1y"
    }
  ]
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
[🔵Info] Collecting template files
[🔵Info] Found 2 JSON files
[🔵Info] Parsing config file
[🔵Info] Expanding delete-everything
[🔵Info] Expanding delete-older-than
[🔵Info] Done
```

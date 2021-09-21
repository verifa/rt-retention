# rt-retention

A JFrog CLI plugin to facilitate retention in Artifactory.

## Installation

This plugin isn't currently hosted anywhere, so you'll be building it locally.

You can use the [build.sh](scripts/build.sh) and [install.sh](scripts/install.sh) scripts.

## Usage

### Commands

- run
  - Arguments:
    - config-file - Path to the retention configuration file
  - Flags:
    - dry-run: Set to true to disable communication with Artifactory **[Default: false]**
    - verbose: Set to true to output more verbose logging **[Default: false]**
  - Example: `$ jfrog rt-retention run --dry-run examples/config.toml`

### Environment variables

N/A

## Retention configuration

Retention configuration is kept in a [TOML](https://toml.io/en/) file.
Currently, only artifact retention is supported.

### Artifact retention

Artifact retention uses [AQL](https://www.jfrog.com/confluence/display/JFROG/Artifactory+Query+Language) queries to match files you wish to delete.

Artifact retention configuration consists of the following:

- **Required**:
  - `AqlPath` (`string`): Path to an AQL query that matches artifacts you wish to delete
- **Optional**:
  - `Name` (`string`): Descriptive name for the retention policy
  - `Offset` (`int`): Amount to offset results by
  - `SortBy` (`[]string`): Fields to sort results by
  - `SortOrder` (`string`): Order to sort results by (`"asc"` or `"desc"`)

```ini
[[Artifact]]
Name="foo-local: Remove all artifacts"
AqlPath="example/retention-policies/foo-local.aql"

[[Artifact]]
Name="bar-local: Keep 5 latest artifacts"
AqlPath="example/retention-policies/bar-local.aql"
Offset=5
SortBy=["updated"]
SortOrder="desc"
```

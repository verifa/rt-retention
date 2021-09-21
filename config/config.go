package config

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/BurntSushi/toml"
)

type Retention struct {
	Artifact []Artifact
}

type Artifact struct {
	Name	string
	AqlPath string
	Offset 	int
	Limit 	int
	SortBy 	[]string
	SortOrder string
}

func ParseRetentionConfiguration(path string) Retention {
	fileExists, err := fileExists(path);
	if (!fileExists) {
		log.Error("Config file does not exist:", path)
		os.Exit(1)
	} else if err != nil {
		log.Error("Error accessing config file:", path)
		log.Error(err)
		os.Exit(1)
	}

	fileContent, err := readFile(path)
	if err != nil {
		log.Error("Error reading config file:", path)
		log.Error(err)
		os.Exit(1)
	}

	var config Retention
	if _, err := toml.Decode(fileContent, &config); err != nil {
		log.Error("Error parsing config file:", path)
		log.Error(err)
		os.Exit(1)
	}

	return config
}

func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		// Schrödinger's file ¯\_(ツ)_/¯
		return false, err
	}
}

func readFile(path string) (string, error) {
    fileContent, err := ioutil.ReadFile(path)
    if err != nil {
        return "", err
    }

	return string(fileContent), nil
}

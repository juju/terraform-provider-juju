// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/names/v4"
)

// controllerConfig is a representation of the output
// returned when running the CLI command
// `juju show-controller --show-password`
type controllerConfig struct {
	ProviderDetails struct {
		UUID                   string   `json:"uuid"`
		ApiEndpoints           []string `json:"api-endpoints"`
		Cloud                  string   `json:"cloud"`
		Region                 string   `json:"region"`
		AgentVersion           string   `json:"agent-version"`
		AgentGitCommit         string   `json:"agent-git-commit"`
		ControllerModelVersion string   `json:"controller-model-version"`
		MongoVersion           string   `json:"mongo-version"`
		CAFingerprint          string   `json:"ca-fingerprint"`
		CACert                 string   `json:"ca-cert"`
	} `json:"details"`
	CurrentModel string `json:"current-model"`
	Models       map[string]struct {
		UUID      string `json:"uuid"`
		UnitCount uint   `json:"unit-count"`
	} `json:"models"`
	Account struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Access   string `json:"access"`
	} `json:"account"`
}

// localProviderConfig is populated once and queried later
// to avoid multiple juju CLI executions
var localProviderConfig map[string]string

// singleQuery will be used to limit the number of CLI queries to ONE
var singleQuery sync.Once

// GetLocalControllerConfig runs the locally installed juju command,
// if available, to get the current controller configuration.
func GetLocalControllerConfig() (map[string]string, error) {
	// populate the controller controllerConfig information only once
	singleQuery.Do(populateControllerConfig)

	// if empty something went wrong
	if localProviderConfig == nil {
		return nil, errors.New("the Juju CLI could not be accessed")
	}

	return localProviderConfig, nil
}

// populateControllerConfig executes the local juju CLI command
// to obtain the current controller configuration
func populateControllerConfig() {
	// get the value from the juju provider
	cmd := exec.Command("juju", "show-controller", "--show-password", "--format=json")

	cmdData, err := cmd.Output()
	if err != nil {
		tflog.Error(context.TODO(), "error invoking juju CLI", map[string]interface{}{"error": err})
		return
	}

	// given that the CLI output is a map containing arbitrary keys
	// (controllers) and fixed json structures, we have to do some
	// workaround to populate the struct
	var cliOutput interface{}
	err = json.Unmarshal(cmdData, &cliOutput)
	if err != nil {
		tflog.Error(context.TODO(), "error unmarshalling Juju CLI output", map[string]interface{}{"error": err})
		return
	}

	// convert to the map and extract the only entry
	controllerConfig := controllerConfig{}
	for _, v := range cliOutput.(map[string]interface{}) {
		// now v is a map[string]interface{} type
		marshalled, err := json.Marshal(v)
		if err != nil {
			tflog.Error(context.TODO(), "error marshalling provider controllerConfig", map[string]interface{}{"error": err})
			return
		}
		// now we have a controllerConfig type
		err = json.Unmarshal(marshalled, &controllerConfig)
		if err != nil {
			tflog.Error(context.TODO(), "error unmarshalling provider configuration from Juju CLI", map[string]interface{}{"error": err})
			return
		}
		break
	}

	localProviderConfig = map[string]string{}
	localProviderConfig["JUJU_CONTROLLER_ADDRESSES"] = strings.Join(controllerConfig.ProviderDetails.ApiEndpoints, ",")
	localProviderConfig["JUJU_CA_CERT"] = controllerConfig.ProviderDetails.CACert
	localProviderConfig["JUJU_USERNAME"] = controllerConfig.Account.User
	localProviderConfig["JUJU_PASSWORD"] = controllerConfig.Account.Password

	tflog.Debug(context.TODO(), "local provider controllerConfig was set", map[string]interface{}{"localProviderConfig": fmt.Sprintf("%#v", localProviderConfig)})
}

// WaitForAppAvailable blocks the execution flow and waits until all the
// application names can be queried before the context is done. The
// tickTime param indicates the frequency used to query the API.
func WaitForAppsAvailable(ctx context.Context, client *apiapplication.Client, appsName []string, tickTime time.Duration) error {
	if len(appsName) == 0 {
		return nil
	}
	// build app tags for these apps
	tags := make([]names.ApplicationTag, len(appsName))
	for i, n := range appsName {
		tags[i] = names.NewApplicationTag(n)
	}

	tick := time.NewTicker(tickTime)
	for {
		select {
		case <-tick.C:
			returned, err := client.ApplicationsInfo(tags)
			// if there is no error and we get as many app infos as
			// requested apps, we can assume the apps are available
			if err != nil {
				return err
			}
			totalAvailable := 0
			for _, entry := range returned {
				// there's no info available yet
				if entry.Result == nil {
					continue
				}
				totalAvailable++
			}
			// All the entries were available
			if totalAvailable == len(appsName) {
				return nil
			}
		case <-ctx.Done():
			return errors.New("the context was done waiting for apps")
		}
	}
}

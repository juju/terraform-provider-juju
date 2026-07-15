// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/juju/api"
	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/names/v6"
)

var getLocalControllerConfigOnce = sync.OnceValues(loadLocalControllerConfig)

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

// GetLocalControllerConfig runs the locally installed juju command,
// if available, to get the current controller configuration.
func GetLocalControllerConfig() (map[string]string, bool) {
	return getLocalControllerConfigOnce()
}

// loadLocalControllerConfig executes the local juju CLI command
// to obtain the current controller configuration.
func loadLocalControllerConfig() (map[string]string, bool) {
	// get the value from the juju provider
	cmd := exec.Command("juju", "show-controller", "--show-password", "--format=json")

	cmdData, err := cmd.Output()
	if err != nil {
		tflog.Error(context.TODO(), "error invoking juju CLI", map[string]any{"error": err})
		return nil, true
	}

	ctrlConfig, err := parseLocalControllerConfig(cmdData)
	if err != nil {
		tflog.Error(context.TODO(), "error parsing Juju CLI output", map[string]any{"error": err})
		return nil, true
	}

	// Avoid logging the password below, but return it in the map for use by the provider.
	tflog.Debug(context.TODO(), "local provider controllerConfig was set", map[string]any{
		"JUJU_AGENT_VERSION":        ctrlConfig.ProviderDetails.AgentVersion,
		"JUJU_CONTROLLER_ADDRESSES": strings.Join(ctrlConfig.ProviderDetails.ApiEndpoints, ","),
		"JUJU_USERNAME":             ctrlConfig.Account.User,
		"JUJU_CA_CERT":              ctrlConfig.ProviderDetails.CACert,
	})

	return map[string]string{
		"JUJU_AGENT_VERSION":        ctrlConfig.ProviderDetails.AgentVersion,
		"JUJU_CONTROLLER_ADDRESSES": strings.Join(ctrlConfig.ProviderDetails.ApiEndpoints, ","),
		"JUJU_USERNAME":             ctrlConfig.Account.User,
		"JUJU_PASSWORD":             ctrlConfig.Account.Password,
		"JUJU_CA_CERT":              ctrlConfig.ProviderDetails.CACert,
	}, false
}

func parseLocalControllerConfig(cmdData []byte) (controllerConfig, error) {
	// given that the CLI output is a map containing arbitrary keys
	// (controllers) and fixed json structures, we have to do some
	// workaround to populate the struct
	var cliOutput map[string]json.RawMessage
	err := json.Unmarshal(cmdData, &cliOutput)
	if err != nil {
		return controllerConfig{}, err
	}

	// convert to the map and extract the only entry
	config := controllerConfig{}
	for _, raw := range cliOutput {
		err = json.Unmarshal(raw, &config)
		if err != nil {
			return controllerConfig{}, err
		}
		return config, nil
	}

	return controllerConfig{}, errors.New("juju CLI returned no controllers")
}

// WaitForAppsAvailable blocks the execution flow and waits until all the
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
			returned, err := client.ApplicationsInfo(ctx, tags)
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

// JaasConnShim is a shim to adapt the juju api.Connection
// Now all APICAll methods require a context.Context parameter, which
// we don't implement in jaas yet.
type JaasConnShim struct {
	api.Connection
}

func (j JaasConnShim) APICall(obj string, version int, id string, request string, params interface{}, response interface{}) error {
	return j.Connection.APICall(context.TODO(), obj, version, id, request, params, response)
}

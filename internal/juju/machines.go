// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/juju/cmd/v3"
	"github.com/juju/errors"
	apiclient "github.com/juju/juju/api/client/client"
	apimachinemanager "github.com/juju/juju/api/client/machinemanager"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/cmd/juju/common"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/series"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/environs/manual"
	"github.com/juju/juju/environs/manual/sshprovisioner"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/storage"
)

type machinesClient struct {
	SharedClient
}

type CreateMachineInput struct {
	ModelName   string
	Constraints string
	Disks       string
	Base        string
	Series      string
	InstanceId  string

	// SSHAddress is the host address of a machine for manual provisioning
	// Note that it has the user too, e.g. user@host
	SSHAddress string

	// PublicKey is the file path to read the public key from
	PublicKeyFile string

	// PrivateKey is the file path to read the private key from
	PrivateKeyFile string
}

type CreateMachineResponse struct {
	ID     string
	Base   string
	Series string
}

type ReadMachineInput struct {
	ModelName string
	ID        string
}

type ReadMachineResponse struct {
	ID          string
	Base        string
	Constraints string
	Series      string
}

type DestroyMachineInput struct {
	ModelName string
	ID        string
}

func newMachinesClient(sc SharedClient) *machinesClient {
	return &machinesClient{
		SharedClient: sc,
	}
}

func (c machinesClient) CreateMachine(input *CreateMachineInput) (*CreateMachineResponse, error) {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	machineAPIClient := apimachinemanager.NewClient(conn)

	modelConfigAPIClient := apimodelconfig.NewClient(conn)

	if input.SSHAddress != "" {
		configAttrs, err := modelConfigAPIClient.ModelGet()
		if err != nil {
			return nil, errors.Trace(err)
		}
		cfg, err := config.New(config.NoDefaults, configAttrs)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return manualProvision(machineAPIClient, cfg,
			input.SSHAddress, input.PublicKeyFile, input.PrivateKeyFile)
	}

	var machineParams params.AddMachineParams

	if input.Constraints == "" {
		modelConstraints, err := modelConfigAPIClient.GetModelConstraints()
		if err != nil {
			return nil, err
		}
		machineParams.Constraints = modelConstraints
	} else {
		userConstraints, err := constraints.Parse(input.Constraints)
		if err != nil {
			return nil, err
		}
		machineParams.Constraints = userConstraints
	}

	if input.Disks != "" {
		userDisks, err := storage.ParseConstraints(input.Disks)
		if err != nil {
			return nil, err
		}
		fmt.Println(userDisks)
		machineParams.Disks = []storage.Constraints{userDisks}
	} else {
		machineParams.Disks = nil
	}

	jobs := []model.MachineJob{model.JobHostUnits}
	machineParams.Jobs = jobs

	opSys := input.Base
	if opSys == "" {
		opSys = input.Series
	}
	machineParams.Base, err = baseFromOperatingSystem(opSys)
	if err != nil {
		return nil, err
	}
	addMachineArgs := []params.AddMachineParams{machineParams}
	machines, err := machineAPIClient.AddMachines(addMachineArgs)
	if err != nil {
		return nil, err
	}
	if machines[0].Error != nil {
		return nil, machines[0].Error
	}
	return &CreateMachineResponse{
		ID:     machines[0].Machine,
		Base:   input.Base,
		Series: input.Series,
	}, err
}

func baseFromOperatingSystem(opSys string) (*params.Base, error) {
	if opSys == "" {
		return nil, nil
	}
	// opSys is a base or a series, check base first.
	info, err := series.ParseBaseFromString(opSys)
	if err != nil {
		info, err = series.GetBaseFromSeries(opSys)
		if err != nil {
			return nil, errors.NotValidf("Base or Series %q", opSys)
		}
	}
	base := &params.Base{
		Name:    info.Name,
		Channel: info.Channel.String(),
	}
	base.Channel = series.FromLegacyCentosChannel(base.Channel)
	return base, nil
}

// manualProvision calls the sshprovisioner.ProvisionMachine on the Juju side
// to provision an existing machine using ssh_address, public_key and
// private_key in the CreateMachineInput.
func manualProvision(client manual.ProvisioningClientAPI,
	config *config.Config, sshAddress string, publicKey string,
	privateKey string) (*CreateMachineResponse, error) {
	// Read the public keys
	cmdCtx, err := cmd.DefaultContext()
	if err != nil {
		return nil, errors.Trace(err)
	}
	authKeys, err := common.ReadAuthorizedKeys(cmdCtx, publicKey)
	if err != nil {
		return nil, errors.Annotatef(err, "cannot read authorized-keys from : %v", publicKey)
	}

	// Extract the user and host in the SSHAddress
	var host, user string
	if at := strings.Index(sshAddress, "@"); at != -1 {
		user, host = sshAddress[:at], sshAddress[at+1:]
	} else {
		return nil, errors.Errorf("invalid ssh_address, expected <user@host>, "+
			"given %v", sshAddress)
	}

	// Prep args for the ProvisionMachine call
	provisionArgs := manual.ProvisionMachineArgs{
		Host:           host,
		User:           user,
		Client:         client,
		Stdin:          os.Stdin,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
		AuthorizedKeys: authKeys,
		PrivateKey:     privateKey,
		UpdateBehavior: &params.UpdateBehavior{
			EnableOSRefreshUpdate: config.EnableOSRefreshUpdate(),
			EnableOSUpgrade:       config.EnableOSUpgrade(),
		},
	}

	// Call ProvisionMachine
	machineId, err := sshprovisioner.ProvisionMachine(provisionArgs)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Find out about the series of the machine just provisioned
	// (because ProvisionMachine only returns machineId)
	_, machineSeries, err := sshprovisioner.DetectSeriesAndHardwareCharacteristics(host)
	if err != nil {
		return nil, errors.Annotatef(err, "error detecting hardware characteristics")
	}

	return &CreateMachineResponse{
		ID:     machineId,
		Series: machineSeries,
	}, nil
}

func (c machinesClient) ReadMachine(input ReadMachineInput) (ReadMachineResponse, error) {
	var response ReadMachineResponse
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return response, err
	}
	defer func() { _ = conn.Close() }()

	clientAPIClient := apiclient.NewClient(conn)

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return response, err
	}

	machineStatus, exists := status.Machines[input.ID]
	if !exists {
		return response, fmt.Errorf("no status returned for machine: %s", input.ID)
	}
	response.ID = machineStatus.Id
	channel, err := series.ParseChannel(machineStatus.Base.Channel)
	if err != nil {
		return response, err
	}
	// This might cause problems later, but today, no one except for juju internals
	// uses the channel risk. Using the risk makes the base appear to have changed
	// with terraform.
	response.Base = fmt.Sprintf("%s@%s", machineStatus.Base.Name, channel.Track)
	response.Series = machineStatus.Series
	response.Constraints = machineStatus.Constraints

	return response, nil
}

func (c machinesClient) DestroyMachine(input *DestroyMachineInput) error {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	machineAPIClient := apimachinemanager.NewClient(conn)

	_, err = machineAPIClient.DestroyMachinesWithParams(false, false, (*time.Duration)(nil), input.ID)

	if err != nil {
		return err
	}

	return nil
}

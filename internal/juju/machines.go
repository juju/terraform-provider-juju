// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/juju/cmd/v3"
	"github.com/juju/errors"
	"github.com/juju/juju/api"
	apiclient "github.com/juju/juju/api/client/client"
	apimachinemanager "github.com/juju/juju/api/client/machinemanager"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/cmd/juju/common"
	"github.com/juju/juju/core/base"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/environs/manual"
	"github.com/juju/juju/environs/manual/sshprovisioner"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/storage"
	"github.com/juju/names/v5"
)

// MachineNotFoundError is returned when a machine cannot be found.
var MachineNotFoundError = errors.ConstError("machine-not-found")

// NewMachineNotFoundError returns an error indicating that a machine with the given ID was not found.
func NewMachineNotFoundError(machineId string) error {
	return errors.WithType(errors.Errorf("machine %s not found", machineId), MachineNotFoundError)
}

type machinesClient struct {
	SharedClient

	createMu sync.Mutex
}

type CreateMachineInput struct {
	ModelName   string
	Constraints string
	Disks       string
	Base        string
	Placement   string
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
	ID string
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
	Hostname    string
	Status      string
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

type targetStatusFunc func(*params.FullStatus) (string, error)

func getMachineStatusFunc(machineID string) targetStatusFunc {
	return func(modelStatus *params.FullStatus) (string, error) {
		machineStatus, ok := modelStatus.Machines[machineID]
		if !ok {
			return "", NewRetryReadError(fmt.Sprintf("machine %q not found", machineID))
		}
		return machineStatus.InstanceStatus.Status, nil
	}
}

func getContainerStatusFunc(containerID, parentMachineID string) targetStatusFunc {
	return func(modelStatus *params.FullStatus) (string, error) {
		machineStatus, ok := modelStatus.Machines[parentMachineID]
		if !ok {
			return "", NewRetryReadError(fmt.Sprintf("machine %q not found", parentMachineID))
		}
		containerStatus, ok := machineStatus.Containers[containerID]
		if !ok {
			return "", NewRetryReadError(fmt.Sprintf("container %q not found in machine %q", containerID, parentMachineID))
		}
		return containerStatus.InstanceStatus.Status, nil
	}
}

func getTargetStatusFunc(machineID string) targetStatusFunc {
	if names.IsContainerMachine(machineID) {
		mt := names.NewMachineTag(machineID)

		return getContainerStatusFunc(machineID, mt.Parent().Id())
	}
	return getMachineStatusFunc(machineID)
}

func (c *machinesClient) CreateMachine(ctx context.Context, input *CreateMachineInput) (*CreateMachineResponse, error) {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	machineID, err := c.createMachine(conn, input)
	if err != nil {
		return nil, err
	}
	return &CreateMachineResponse{
		ID: machineID,
	}, nil
}

func (c *machinesClient) createMachine(conn api.Connection, input *CreateMachineInput) (string, error) {
	machineAPIClient := apimachinemanager.NewClient(conn)
	modelConfigAPIClient := apimodelconfig.NewClient(conn)

	if input.SSHAddress != "" {
		configAttrs, err := modelConfigAPIClient.ModelGet()
		if err != nil {
			return "", errors.Trace(err)
		}
		cfg, err := config.New(config.NoDefaults, configAttrs)
		if err != nil {
			return "", errors.Trace(err)
		}
		return manualProvision(machineAPIClient, cfg,
			input.SSHAddress, input.PublicKeyFile, input.PrivateKeyFile)
	}

	var machineParams params.AddMachineParams
	var err error

	placement := input.Placement
	if placement != "" {
		machineParams.Placement, err = instance.ParsePlacement(placement)
		if err == instance.ErrPlacementScopeMissing {
			modelUUID, err := c.ModelUUID(input.ModelName)
			if err != nil {
				return "", err
			}
			placement = modelUUID + ":" + placement
			machineParams.Placement, err = instance.ParsePlacement(placement)
			if err != nil {
				return "", err
			}
		}
	}

	if input.Constraints == "" {
		modelConstraints, err := modelConfigAPIClient.GetModelConstraints()
		if err != nil {
			return "", err
		}
		machineParams.Constraints = modelConstraints
	} else {
		userConstraints, err := constraints.Parse(input.Constraints)
		if err != nil {
			return "", err
		}
		machineParams.Constraints = userConstraints
	}

	if input.Disks != "" {
		userDisks, err := storage.ParseConstraints(input.Disks)
		if err != nil {
			return "", err
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
	paramsBase, err := baseFromOperatingSystem(opSys)
	if err != nil {
		return "", err
	}
	machineParams.Base = paramsBase

	addMachineArgs := []params.AddMachineParams{machineParams}

	// There is a bug in juju that affects concurrent creation of machines, so we make
	// all AddMachine calls sequential.
	// TODO (alesstimec): remove once this bug in Juju is fixed.
	c.createMu.Lock()
	machines, err := machineAPIClient.AddMachines(addMachineArgs)
	if err != nil {
		c.createMu.Unlock()
		return "", err
	}
	c.createMu.Unlock()

	if machines[0].Error != nil {
		return "", machines[0].Error
	}

	return machines[0].Machine, nil
}

func baseAndSeriesFromParams(machineBase *params.Base) (baseStr, seriesStr string, err error) {
	if machineBase == nil {
		return "", "", errors.NotValidf("no base from machine status")
	}
	channel, err := base.ParseChannel(machineBase.Channel)
	if err != nil {
		return "", "", err
	}
	// This might cause problems later, but today, no one except for juju internals
	// uses the channel risk. Using the risk makes the base appear to have changed
	// with terraform.
	baseStr = fmt.Sprintf("%s@%s", machineBase.Name, channel.Track)

	seriesStr, err = base.GetSeriesFromBase(base.Base{
		OS:      machineBase.Name,
		Channel: base.Channel{Track: channel.Track, Risk: channel.Risk},
	})
	if err != nil {
		return "", "", errors.NotValidf("Base or Series %q", machineBase)
	}
	return baseStr, seriesStr, err
}

func baseFromOperatingSystem(opSys string) (*params.Base, error) {
	if opSys == "" {
		return nil, nil
	}
	// opSys is a base or a series, check base first.
	info, err := base.ParseBaseFromString(opSys)
	if err != nil {
		info, err = base.GetBaseFromSeries(opSys)
		if err != nil {
			return nil, errors.NotValidf("Base or Series %q", opSys)
		}
	}
	base := &params.Base{
		Name:    info.OS,
		Channel: info.Channel.String(),
	}
	base.Channel = fromLegacyCentosChannel(base.Channel)
	return base, nil
}

func fromLegacyCentosChannel(series string) string {
	if strings.HasPrefix(series, "centos") {
		return strings.TrimLeft(series, "centos")
	}
	return series
}

// manualProvision calls the sshprovisioner.ProvisionMachine on the Juju side
// to provision an existing machine using ssh_address, public_key and
// private_key in the CreateMachineInput.
func manualProvision(client manual.ProvisioningClientAPI,
	config *config.Config, sshAddress string, publicKey string,
	privateKey string) (string, error) {
	// Read the public keys
	cmdCtx, err := cmd.DefaultContext()
	if err != nil {
		return "", errors.Trace(err)
	}
	authKeys, err := common.ReadAuthorizedKeys(cmdCtx, publicKey)
	if err != nil {
		return "", errors.Annotatef(err, "cannot read authorized-keys from : %v", publicKey)
	}

	// Extract the user and host in the SSHAddress
	var host, user string
	if at := strings.Index(sshAddress, "@"); at != -1 {
		user, host = sshAddress[:at], sshAddress[at+1:]
	} else {
		return "", errors.Errorf("invalid ssh_address, expected <user@host>, "+
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
		return "", errors.Trace(err)
	}

	return machineId, nil
}

func (c *machinesClient) ReadMachine(input *ReadMachineInput) (*ReadMachineResponse, error) {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	clientAPIClient := apiclient.NewClient(conn, c.JujuLogger())

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return nil, err
	}
	machineIDParts := strings.Split(input.ID, "/")
	machineStatus, exists := status.Machines[machineIDParts[0]]
	if !exists {
		return nil, NewMachineNotFoundError(input.ID)
	}
	c.Tracef("ReadMachine:Machine status result", map[string]interface{}{"machineStatus": machineStatus})
	if len(machineIDParts) > 1 {
		// check for containers
		machineStatus, exists = machineStatus.Containers[input.ID]
		if !exists {
			return nil, NewMachineNotFoundError(input.ID)
		}
	}

	machineStatusString, err := getTargetStatusFunc(input.ID)(status)
	if err != nil {
		return nil, err
	}

	base, series, err := baseAndSeriesFromParams(&machineStatus.Base)
	if err != nil {
		return nil, err
	}

	return &ReadMachineResponse{
		ID:          machineStatus.Id,
		Hostname:    machineStatus.Hostname,
		Constraints: machineStatus.Constraints,
		Base:        base,
		Series:      series,
		Status:      machineStatusString,
	}, nil
}

func (c *machinesClient) DestroyMachine(input *DestroyMachineInput) error {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	machineAPIClient := apimachinemanager.NewClient(conn)

	_, err = machineAPIClient.DestroyMachinesWithParams(false, false, false, (*time.Duration)(nil), input.ID)
	if err != nil {
		return err
	}

	return nil
}

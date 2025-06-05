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

	"github.com/juju/clock"
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
	"github.com/juju/juju/core/status"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/environs/manual"
	"github.com/juju/juju/environs/manual/sshprovisioner"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/storage"
	"github.com/juju/names/v5"
	"github.com/juju/retry"
)

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

type targetStatusFunc func(*params.FullStatus) (string, error)

func getMachineStatusFunc(machineID string) targetStatusFunc {
	return func(modelStatus *params.FullStatus) (string, error) {
		machineStatus, ok := modelStatus.Machines[machineID]
		if !ok {
			return "", &keepWaitingError{
				item:     names.NewMachineTag(machineID).String(),
				state:    "unknown",
				endState: status.Running.String(),
			}
		}
		return machineStatus.InstanceStatus.Status, nil
	}
}

func getContainerStatusFunc(containerID, parentMachineID string) targetStatusFunc {
	return func(modelStatus *params.FullStatus) (string, error) {
		machineStatus, ok := modelStatus.Machines[parentMachineID]
		if !ok {
			return "", &keepWaitingError{
				item:     names.NewMachineTag(containerID).String(),
				state:    "unknown",
				endState: status.Running.String(),
			}
		}
		containerStatus, ok := machineStatus.Containers[containerID]
		if !ok {
			return "", &keepWaitingError{
				item:     names.NewMachineTag(containerID).String(),
				state:    "unknown",
				endState: status.Running.String(),
			}
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

	response, err := c.createMachine(ctx, conn, input)
	if err != nil {
		return nil, err
	}

	getTargetStatusF := getTargetStatusFunc(response.ID)

	// Wait for machine to go into "running" state. This is important when using the placement directive
	// in juju_application resource - to deploy an application or validate against the operating system
	// specified for the application Juju must know the operating system to use. For actual machines that
	// information is not available until it reaches the "running" state.
	retryErr := retry.Call(retry.CallArgs{
		Func: func() error {
			if !c.WaitForResource() {
				return nil
			}
			modelStatus, err := c.ModelStatus(input.ModelName, conn)
			if err != nil {
				return errors.Annotatef(err, "failed to get model status.")
			}

			machineStatus, err := getTargetStatusF(modelStatus)
			if err != nil {
				return errors.Annotatef(err, "failed to get machine status")
			}
			if machineStatus == status.Running.String() {
				return nil
			}
			return &keepWaitingError{
				item:     names.NewMachineTag(response.ID).String(),
				state:    machineStatus,
				endState: status.Running.String(),
			}
		},
		NotifyFunc: func(err error, attempt int) {
			if attempt%4 == 0 {
				message := fmt.Sprintf("waiting for machine %q to be created", response.ID)
				c.Debugf(message, map[string]interface{}{"attempt": attempt, "err": err})
			}
		},
		IsFatalError: func(err error) bool {
			return !errors.As(err, &KeepWaitingForError)
		},
		Attempts: 720,
		Delay:    defaultModelStatusCacheRetryInterval,
		Clock:    clock.WallClock,
		Stop:     ctx.Done(),
	})
	return response, retryErr
}

func (c *machinesClient) createMachine(ctx context.Context, conn api.Connection, input *CreateMachineInput) (*CreateMachineResponse, error) {
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
	var err error

	placement := input.Placement
	if placement != "" {
		machineParams.Placement, err = instance.ParsePlacement(placement)
		if err == instance.ErrPlacementScopeMissing {
			modelUUID, err := c.ModelUUID(input.ModelName)
			if err != nil {
				return nil, err
			}
			placement = modelUUID + ":" + placement
			machineParams.Placement, err = instance.ParsePlacement(placement)
			if err != nil {
				return nil, err
			}
		}
	}

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
	paramsBase, err := baseFromOperatingSystem(opSys)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	c.createMu.Unlock()

	if machines[0].Error != nil {
		return nil, machines[0].Error
	}
	machineID := machines[0].Machine

	// Read the machine to ensure we have a base and series. It's
	// not a required field in a minimal machine config.
	readResponse, err := c.readMachineWithRetryOnNotFound(ctx,
		ReadMachineInput{ModelName: input.ModelName, ID: machineID})

	return &CreateMachineResponse{
		ID:     machineID,
		Base:   readResponse.Base,
		Series: readResponse.Series,
	}, err
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
	_, machineBase, err := sshprovisioner.DetectBaseAndHardwareCharacteristics(host)
	if err != nil {
		return nil, errors.Annotatef(err, "error detecting hardware characteristics")
	}

	machineSeries, err := base.GetSeriesFromBase(machineBase)
	if err != nil {
		return nil, err
	}

	// This might cause problems later, but today, no one except for juju internals
	// uses the channel risk. Using the risk makes the base appear to have changed
	// with terraform.
	baseStr := fmt.Sprintf("%s@%s", machineBase.OS, machineBase.Channel.Track)

	return &CreateMachineResponse{
		ID:     machineId,
		Base:   baseStr,
		Series: machineSeries,
	}, nil
}

func (c *machinesClient) ReadMachine(input ReadMachineInput) (ReadMachineResponse, error) {
	var response ReadMachineResponse
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return response, err
	}
	defer func() { _ = conn.Close() }()

	clientAPIClient := apiclient.NewClient(conn, c.JujuLogger())

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return response, err
	}

	machineIDParts := strings.Split(input.ID, "/")
	machineStatus, exists := status.Machines[machineIDParts[0]]
	if !exists {
		return response, fmt.Errorf("no status returned for machine: %s", input.ID)
	}
	c.Tracef("ReadMachine:Machine status result", map[string]interface{}{"machineStatus": machineStatus})
	if len(machineIDParts) > 1 {
		// check for containers
		machineStatus, exists = machineStatus.Containers[input.ID]
		if !exists {
			return response, fmt.Errorf("no status returned for container in machine: %s", input.ID)
		}
	}
	response.ID = machineStatus.Id
	response.Base, response.Series, err = baseAndSeriesFromParams(&machineStatus.Base)
	if err != nil {
		return response, err
	}
	response.Constraints = machineStatus.Constraints
	return response, nil
}

// readMachineWithRetryOnNotFound calls ReadMachine until
// successful, or the count is exceeded when the error is of type
// not found. Delay indicates how long to wait between attempts.
func (c *machinesClient) readMachineWithRetryOnNotFound(ctx context.Context, input ReadMachineInput) (ReadMachineResponse, error) {
	var output ReadMachineResponse
	err := retry.Call(retry.CallArgs{
		Func: func() error {
			var err error
			output, err = c.ReadMachine(input)
			typedErr := typedError(err)
			if errors.Is(typedErr, errors.NotFound) {
				return nil
			}
			return err
		},
		NotifyFunc: func(err error, attempt int) {
			if attempt%4 == 0 {
				message := fmt.Sprintf("waiting for machine %q", input.ID)
				if attempt != 4 {
					message = "still " + message
				}
				c.Debugf(message)
			}
		},
		BackoffFunc: retry.DoubleDelay,
		Attempts:    30,
		Delay:       time.Second,
		Clock:       clock.WallClock,
		Stop:        ctx.Done(),
	})
	return output, err
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

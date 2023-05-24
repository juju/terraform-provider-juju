package juju

import (
	"fmt"
	"github.com/juju/cmd/v3"
	"github.com/juju/errors"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/environs/manual/sshprovisioner"
	"strings"

	"github.com/juju/juju/cmd/juju/common"
	"github.com/juju/juju/environs/config"
	"time"

	"github.com/juju/juju/rpc/params"

	apiclient "github.com/juju/juju/api/client/client"
	apimachinemanager "github.com/juju/juju/api/client/machinemanager"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/series"
	"github.com/juju/juju/environs/manual"
	"github.com/juju/juju/storage"
)

type machinesClient struct {
	ConnectionFactory
}

type CreateMachineInput struct {
	ModelUUID   string
	Constraints string
	Disks       string
	Series      string
	InstanceId  string

	// SSHAddress is the host address of a machine for manual provisioning
	// Note that it has the user too, e.g. user@host
	SSHAddress string

	// PublicKey is the file path to read the public key from
	PublicKey string

	// PrivateKey is the file path to read the private key from
	PrivateKey string
}

type CreateMachineResponse struct {
	Machines []params.AddMachinesResult
}

type ReadMachineInput struct {
	ModelUUID string
	MachineId string
}

type ReadMachineResponse struct {
	MachineId     string
	MachineStatus params.MachineStatus
}

type DestroyMachineInput struct {
	ModelUUID string
	MachineId string
}

func newMachinesClient(cf ConnectionFactory) *machinesClient {
	return &machinesClient{
		ConnectionFactory: cf,
	}
}

// manualProvision calls the sshprovisioner.ProvisionMachine on the Juju side to provision an
// existing machine using ssh_address, public_key and private_key in the CreateMachineInput
// TODO (cderici): only the ssh scope is supported, include winrm at some point
func (i *CreateMachineInput) manualProvision(client manual.ProvisioningClientAPI, config *config.Config) error {

	// Read the public key
	cmdCtx, err := cmd.DefaultContext()
	if err != nil {
		return errors.Trace(err)
	}
	authKeys, err := common.ReadAuthorizedKeys(cmdCtx, i.PublicKey)
	if err != nil {
		return errors.Annotatef(err, "cannot reading authorized-keys")
	}

	// Extract the user and host in the SSHAddress
	var host, user string
	if at := strings.Index(i.SSHAddress, "@"); at != -1 {
		user, host = i.SSHAddress[:at], i.SSHAddress[at+1:]
	} else {
		return errors.Errorf("invalid ssh_address, expected <user@host>, given ", i.SSHAddress)
	}

	// Prep args for the ProvisionMachine call
	provisionArgs := manual.ProvisionMachineArgs{
		Host:           host,
		User:           user,
		Client:         client,
		Stdin:          cmdCtx.Stdin,
		Stdout:         cmdCtx.Stdout,
		Stderr:         cmdCtx.Stderr,
		AuthorizedKeys: authKeys,
		PrivateKey:     i.PrivateKey,
		UpdateBehavior: &params.UpdateBehavior{
			EnableOSRefreshUpdate: config.EnableOSRefreshUpdate(),
			EnableOSUpgrade:       config.EnableOSUpgrade(),
		},
	}

	// Call the ProvisionMachine
	// Note that the returned machineId is ignored
	_, err = sshprovisioner.ProvisionMachine(provisionArgs)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c machinesClient) CreateMachine(input *CreateMachineInput) (*CreateMachineResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	machineAPIClient := apimachinemanager.NewClient(conn)
	defer machineAPIClient.Close()

	modelConfigAPIClient := apimodelconfig.NewClient(conn)
	defer modelConfigAPIClient.Close()

	var machineParams params.AddMachineParams

	if input.SSHAddress != "" {
		configAttrs, err := modelConfigAPIClient.ModelGet()
		if err != nil {
			return nil, errors.Trace(err)
		}
		cfg, err := config.New(config.NoDefaults, configAttrs)
		if err != nil {
			return nil, errors.Trace(err)
		}
		err = input.manualProvision(machineAPIClient, cfg)
		if err != nil {
			return nil, errors.Trace(err)
		}
		// Set the Placement so AddMachines would know that
		// it's manually provisioned
		machineParams.Placement = &instance.Placement{
			Scope:     "ssh",
			Directive: input.SSHAddress,
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

	seriesBase, err := series.GetBaseFromSeries(input.Series)
	if err != nil {
		return nil, err
	}

	jobs := []model.MachineJob{model.JobHostUnits}
	var paramsBase params.Base
	paramsBase.Name = seriesBase.Name
	paramsBase.Channel = series.Channel.String(seriesBase.Channel)

	machineParams.Jobs = jobs

	machineParams.Base = &paramsBase

	addMachineArgs := []params.AddMachineParams{machineParams}

	machines, err := machineAPIClient.AddMachines(addMachineArgs)
	return &CreateMachineResponse{
		Machines: machines,
	}, err
}

func (c machinesClient) ReadMachine(input *ReadMachineInput) (*ReadMachineResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	clientAPIClient := apiclient.NewClient(conn)
	defer clientAPIClient.Close()

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return nil, err
	}
	var machineStatus params.MachineStatus
	var exists bool
	if machineStatus, exists = status.Machines[input.MachineId]; !exists {
		return nil, fmt.Errorf("no status returned for machine: %s", input.MachineId)
	}
	response := &ReadMachineResponse{
		MachineId:     machineStatus.Id,
		MachineStatus: machineStatus,
	}

	return response, nil
}

func (c machinesClient) DestroyMachine(input *DestroyMachineInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}

	machineAPIClient := apimachinemanager.NewClient(conn)
	defer machineAPIClient.Close()

	_, err = machineAPIClient.DestroyMachinesWithParams(false, false, (*time.Duration)(nil), input.MachineId)

	if err != nil {
		return err
	}

	return nil
}

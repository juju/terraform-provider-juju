package juju

import (
	"fmt"
	"os"

	"github.com/juju/cmd/v3"
	"github.com/juju/errors"
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
	PublicKeyFile string

	// PrivateKey is the file path to read the private key from
	PrivateKeyFile string
}

type CreateMachineResponse struct {
	Machine params.AddMachinesResult
	Series  string
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
		machine_series, machineId, err := manualProvision(machineAPIClient, cfg,
			input.SSHAddress, input.PublicKeyFile, input.PrivateKeyFile)
		if err != nil {
			return nil, errors.Trace(err)
		} else {
			return &CreateMachineResponse{
				Machine: params.AddMachinesResult{
					Machine: machineId,
					Error:   nil,
				},
				Series: machine_series,
			}, nil
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

	var paramsBase params.Base
	if input.Series != "" {
		seriesBase, err := series.GetBaseFromSeries(input.Series)
		if err != nil {
			return nil, err
		}

		paramsBase.Name = seriesBase.Name
		paramsBase.Channel = series.Channel.String(seriesBase.Channel)
	}
	machineParams.Base = &paramsBase

	addMachineArgs := []params.AddMachineParams{machineParams}
	machines, err := machineAPIClient.AddMachines(addMachineArgs)
	return &CreateMachineResponse{
		Machine: machines[0],
		Series:  input.Series,
	}, err
}

// manualProvision calls the sshprovisioner.ProvisionMachine on the Juju side to provision an
// existing machine using ssh_address, public_key and private_key in the CreateMachineInput
// TODO (cderici): only the ssh scope is supported, include winrm at some point
func manualProvision(client manual.ProvisioningClientAPI,
	config *config.Config, sshAddress string, publicKey string,
	privateKey string) (string, string, error) {
	// Read the public keys
	cmdCtx, err := cmd.DefaultContext()
	if err != nil {
		return "", "", errors.Trace(err)
	}
	authKeys, err := common.ReadAuthorizedKeys(cmdCtx, publicKey)
	if err != nil {
		return "", "", errors.Annotatef(err, "cannot read authorized-keys from : %v", publicKey)
	}

	// Extract the user and host in the SSHAddress
	var host, user string
	if at := strings.Index(sshAddress, "@"); at != -1 {
		user, host = sshAddress[:at], sshAddress[at+1:]
	} else {
		return "", "", errors.Errorf("invalid ssh_address, expected <user@host>, "+
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

	// Call the ProvisionMachine
	machineId, err := sshprovisioner.ProvisionMachine(provisionArgs)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	// Find out about the series of the machine just provisioned
	// (because ProvisionMachine only returns machineId)
	_, series, err := sshprovisioner.DetectSeriesAndHardwareCharacteristics(host)
	if err != nil {
		return "", "", errors.Annotatef(err, "error detecting linux hardware characteristics")
	}

	return series, machineId, nil
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

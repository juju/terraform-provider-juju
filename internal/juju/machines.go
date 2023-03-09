package juju

import (
	"fmt"
	"time"

	"github.com/juju/juju/rpc/params"

	apiclient "github.com/juju/juju/api/client/client"
	apimachinemanager "github.com/juju/juju/api/client/machinemanager"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/series"
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

func (c machinesClient) CreateMachine(input *CreateMachineInput) (*CreateMachineResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	machineAPIClient := apimachinemanager.NewClient(conn)
	defer machineAPIClient.Close()

	modelconfigAPIClient := apimodelconfig.NewClient(conn)
	defer modelconfigAPIClient.Close()

	var machineParams params.AddMachineParams
	var machineConstraints constraints.Value

	if input.Constraints == "" {
		modelConstraints, err := modelconfigAPIClient.GetModelConstraints()
		if err != nil {
			return nil, err
		}
		machineConstraints = modelConstraints
	} else {
		userConstraints, err := constraints.Parse(input.Constraints)
		if err != nil {
			return nil, err
		}
		machineConstraints = userConstraints
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
	machineParams.Constraints = machineConstraints

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

	machineAPIClient := apimachinemanager.NewClient(conn)
	defer machineAPIClient.Close()

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

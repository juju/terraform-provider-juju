package juju

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/juju/juju/core/model"
	"github.com/juju/juju/rpc/params"
	"github.com/rs/zerolog/log"

	jujuerrors "github.com/juju/errors"
	apiclient "github.com/juju/juju/api/client/client"
	apimachinemanager "github.com/juju/juju/api/client/machinemanager"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/series"
	"github.com/juju/juju/storage"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/version"
	"github.com/juju/names/v4"
)

type machinesClient struct {
	ConnectionFactory
}

type CreateMachineInput struct {
	ModelUUID      string
	Constraints    string
	Disks          string
	Series         string
}

type CreateMachineResponse struct {
	Machine   string
}

type ReadMachineInput struct {
	ModelUUID string
	MachineId string
}

type ReadMachineResponse struct {
	MachineId         string
	MachineStatus     params.MachineStatus
}

func (c machinesClient) CreateMachine(input*CreateMachineInput) (*CreateMachineResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	machineAPIClient := apimachinemanager.NewClient(conn)
	defer machineAPIClient.Close()

	modelconfigAPIClient := apimodelconfig.NewClient(conn)
	defer modelconfigAPIClient.Close()

	var machineConstraints constraints.Value

	if input.Constraints == ""{
	    modelConstraints, err := modelconfigAPIClient.GetModelConstraints()
		if err != nil {
			return nil, err
		}
		machineConstraints = modelConstraints
	} else {
		userConstraints, err := constraints.ParseConstraints(input.Constraints)
		if err != nil {
			return nil, err
		}
		machineConstraints = userConstraints
	}

	var diskConstraints storage.Constraints

	if input.Disks != "" {
		userDisks, err := storage.ParseConstraints(input.Disks)
		if err != nil {
			return nil, err
		}
		diskConstraints = userDisks
	}

	base, err := series.GetBaseFromSeries(input.Series)
	if err != nil {
		return nil, err
	}

	var addMachineArgs params.AddMachineParams

	addMachineArgs.Base = base
	addMachineArgs.Constraints = machineConstraints
	addMachineArgs.Disks = diskConstraints

	var addMachines = params.AddMachines
	addMachines.MachineParams = [1]params.AddMachineParams{addMachineArgs}

	machines, err := machineAPIClient.AddMachines(addMachineArgs)

	return &CreateMachineResponse {
			Machine: "",
	}, err
}

func (c machineAPIClient) ReadMachine(input *ReadMachineInput) (*ReadMachineResponse, error) {
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
		MachineId: machineStatus.Id,
		MachineStatus: machineStatus
	}

	return response, nil
}
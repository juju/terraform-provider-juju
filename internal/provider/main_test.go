// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

// Here we process the environment variables to be used
// in the testing phase.

import (
	"errors"
	"net"
	"os"
	"strings"
	"testing"
)

// Env variables to use for various testing purposes
const TestCloudEnvKey string = "TEST_CLOUD"
const TestMachineIPEnvKey string = "TEST_ADD_MACHINE_IP"
const TestSSHPublicKeyFileEnvKey string = "TEST_SSH_PUB_KEY_PATH"
const TestSSHPrivateKeyFileEnvKey string = "TEST_SSH_PRIV_KEY_PATH"

// CloudTesting is a value indicating the current cloud
// available for testing
type CloudTesting string

// LXDCloudTesting
const LXDCloudTesting CloudTesting = "lxd"

// MicroK8sTesting
const MicroK8sTesting CloudTesting = "microk8s"

func (ct CloudTesting) String() string {
	return string(ct)
}

// CloudName returns the cloud name as displayed
// when using `juju list-clouds`. For example,
// a controller can be bootstrapped with an lxd type.
// However, that's the controller type, the cloud name
// would be localhost
func (ct CloudTesting) CloudName() string {
	// Right now, we're only testing two cases and
	// a switch could be unnecessary. However,
	// this can be useful in the future.
	switch ct {
	case LXDCloudTesting:
		return "localhost"
	default:
		return ct.String()
	}
}

func TypeTestingCloudFromString(from string) (CloudTesting, error) {
	switch strings.ToLower(from) {
	case string(LXDCloudTesting):
		return LXDCloudTesting, nil
	case string(MicroK8sTesting):
		return MicroK8sTesting, nil
	default:
		return "", errors.New("unknown cloud type")
	}
}

// testingCloud stores what type of cloud are we using
// for the testing phase.
var testingCloud CloudTesting

// testAddMachineIP stores the IP address of a manually created machine outside terraform,
// communicated via the TEST_ADD_MACHINE_IP env variable.
// That IP will be used in testing the add-machine functionality on terraform via ssh_address.
var testAddMachineIP = ""
var testSSHPubKeyPath = ""
var testSSHPrivKeyPath = ""

func TestMain(m *testing.M) {
	testCloud := os.Getenv(TestCloudEnvKey)

	testMachineIP, exist := os.LookupEnv(TestMachineIPEnvKey)
	// Confirm the validity of the IP address before setting
	if exist && net.ParseIP(testMachineIP) != nil {
		testAddMachineIP = testMachineIP
	}
	testSSHPubKeyPath = os.Getenv(TestSSHPublicKeyFileEnvKey)
	testSSHPrivKeyPath = os.Getenv(TestSSHPrivateKeyFileEnvKey)

	var err error
	testingCloud, err = TypeTestingCloudFromString(testCloud)
	if err != nil {
		panic(err)
	} else {
		m.Run()
	}
}

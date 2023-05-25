package provider

// Here we process the environment variables to be used
// in the testing phase.

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// CloudTesting is a value indicating the current cloud
// available for testing
type CloudTesting string

// LXDCloudTesting
const LXDCloudTesting CloudTesting = "lxd"

// MicroK8sTesting
const MicroK8sTesting CloudTesting = "microk8s"

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

func TestMain(m *testing.M) {

	testCloud := os.Getenv("TEST_CLOUD")

	var err error
	testingCloud, err = TypeTestingCloudFromString(testCloud)
	if err != nil {
		panic(err)
	} else {
		m.Run()
	}
}

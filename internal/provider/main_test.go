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

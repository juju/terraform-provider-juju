package juju

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEndpointPath(t *testing.T) {
	application := "ausf"
	name := "sdcore-ausf-k8s"
	want := "/applications/ausf/resources/sdcore-ausf-k8s"
	got := newEndpointPath(application, name)
	assert.Equal(t, got, want)
}

func TestNewEndpointPathEmptyInputs(t *testing.T) {
	application := ""
	name := ""
	want := "/applications//resources/"
	got := newEndpointPath(application, name)
	assert.Equal(t, got, want)
}

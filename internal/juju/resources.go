// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"os"

	charmresources "github.com/juju/charm/v12/resource"
	jujuerrors "github.com/juju/errors"
	apiapplication "github.com/juju/juju/api/client/application"
	resourcecmd "github.com/juju/juju/cmd/juju/resource"
	"github.com/juju/juju/cmd/modelcmd"
)

type osFilesystem struct{}

func (osFilesystem) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (osFilesystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (osFilesystem) Open(name string) (modelcmd.ReadSeekCloser, error) {
	return os.Open(name)
}

func (osFilesystem) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (osFilesystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// UploadExistingPendingResources uploads local resources. Used
// after DeployFromRepository, where the resources have been added
// to the controller.
func uploadExistingPendingResources(
	appName string,
	pendingResources []apiapplication.PendingResourceUpload,
	filesystem modelcmd.Filesystem,
	resourceAPIClient ResourceAPIClient) error {
	if pendingResources == nil {
		return nil
	}

	for _, pendingResUpload := range pendingResources {
		t, typeParseErr := charmresources.ParseType(pendingResUpload.Type)
		if typeParseErr != nil {
			return jujuerrors.Annotatef(typeParseErr, "invalid type %v for pending resource %v",
				pendingResUpload.Type, pendingResUpload.Name)
		}

		r, openResErr := resourcecmd.OpenResource(pendingResUpload.Filename, t, filesystem.Open)
		if openResErr != nil {
			return jujuerrors.Annotatef(openResErr, "unable to open resource %v", pendingResUpload.Name)
		}
		uploadErr := resourceAPIClient.Upload(appName, pendingResUpload.Name, pendingResUpload.Filename, "", r)

		if uploadErr != nil {
			return jujuerrors.Trace(uploadErr)
		}
	}
	return nil
}

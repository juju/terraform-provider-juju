// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju_test

//go:generate go run go.uber.org/mock/mockgen -package juju -destination mock_test.go github.com/juju/terraform-provider-juju/internal/juju SharedClient,ClientAPIClient,ApplicationAPIClient,ModelConfigAPIClient
//go:generate go run go.uber.org/mock/mockgen -package juju -destination jujuapi_mock_test.go github.com/juju/juju/api Connection

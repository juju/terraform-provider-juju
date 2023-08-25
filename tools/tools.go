// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

//go:build tools
// +build tools

package tools

import (
	_ "github.com/bflad/tfproviderlint/cmd/tfproviderlint"
	_ "github.com/bflad/tfproviderlint/cmd/tfproviderlintx"
	// document generation
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)

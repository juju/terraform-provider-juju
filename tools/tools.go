//go:build tools
// +build tools

package tools

import (
	_ "github.com/bflad/tfproviderlint/cmd/tfproviderlint"
	_ "github.com/bflad/tfproviderlint/cmd/tfproviderlintx"
	// document generation
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.List = caPEMCertificateListValidator{}

// caPEMCertificateListValidator validates that each list element is a PEM-encoded X.509 certificate.
type caPEMCertificateListValidator struct{}

func (v caPEMCertificateListValidator) Description(_ context.Context) string {
	return "each element must be a PEM-encoded X.509 certificate (BEGIN CERTIFICATE)"
}

func (v caPEMCertificateListValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v caPEMCertificateListValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// Extract raw elements as []string.
	var certs []string
	diags := req.ConfigValue.ElementsAs(ctx, &certs, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	for i, s := range certs {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
				req.Path.AtListIndex(i),
				v.Description(ctx),
				"empty certificate string",
			))
			continue
		}

		if !strings.Contains(trimmed, "-----BEGIN CERTIFICATE-----") {
			resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
				req.Path.AtListIndex(i),
				v.Description(ctx),
				"certificate must be PEM-encoded and include BEGIN CERTIFICATE",
			))
			continue
		}

		// Iterate all PEM blocks and ensure each CERTIFICATE parses, and at least one exists.
		data := []byte(trimmed)
		foundCert := false
		for {
			block, rest := pem.Decode(data)
			if block == nil {
				break
			}
			data = rest
			if block.Type != "CERTIFICATE" {
				// Ignore non-certificate blocks.
				continue
			}
			foundCert = true
			if len(block.Bytes) == 0 {
				resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
					req.Path.AtListIndex(i),
					v.Description(ctx),
					"invalid PEM certificate block: empty bytes",
				))
				// keep checking remaining blocks
				continue
			}
			if _, err := x509.ParseCertificate(block.Bytes); err != nil {
				resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
					req.Path.AtListIndex(i),
					v.Description(ctx),
					fmt.Sprintf("failed to parse certificate: %v", err),
				))
			}
		}
		if !foundCert {
			resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
				req.Path.AtListIndex(i),
				v.Description(ctx),
				"no CERTIFICATE blocks found in PEM",
			))
		}
	}
}

// ValidateCACertificatesPEM returns a validator for ca_certificates list attributes that enforces PEM encoding.
func ValidateCACertificatesPEM() validator.List {
	return caPEMCertificateListValidator{}
}

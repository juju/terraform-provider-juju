// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/juju/errors"

	"github.com/juju/terraform-provider-juju/internal/juju"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceCloud(t *testing.T) {
	SkipJAAS(t)

	cloudName := acctest.RandomWithPrefix("tf-test-cloud")
	resourceName := "juju_cloud." + cloudName
	ca1 := newTestCA(t, "Team Manchester")
	ca2 := newTestCA(t, "Team Rocket")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckCloudDestroy(cloudName),
		Steps: []resource.TestStep{
			// Only required fields.
			{
				Config: testAccResourceCloud_OpenStack_Minimal(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", cloudName),
					resource.TestCheckResourceAttr(resourceName, "type", "openstack"),
					resource.TestCheckResourceAttr(resourceName, "regions.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.name", "default"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.0.endpoint"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.0.identity_endpoint"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.0.storage_endpoint"),
				),
			},
			// Update in-place every other field (they're all update in place).
			{
				Config: testAccCloudFromTemplate(internaltesting.TemplateData{
					"Name":                    cloudName,
					"Type":                    "openstack",
					"IncludeAuthTypes":        true,
					"AuthTypesList":           hclList([]string{"userpass", "access-key"}),
					"IncludeEndpoint":         true,
					"Endpoint":                "https://cloud.example.com",
					"IncludeIdentityEndpoint": true,
					"IdentityEndpoint":        "https://identity.example.com",
					"IncludeStorageEndpoint":  true,
					"StorageEndpoint":         "https://storage.example.com",
					"IncludeCACerts":          true,
					"CACertsHCL":              hclCACerts([]string{ca1, ca2}),
					"IncludeRegions":          true,
					"RegionsHCL": hclRegions([]map[string]string{
						{"name": "default", "endpoint": "https://region-default.example.com", "identity_endpoint": "https://identity-default.example.com", "storage_endpoint": "https://storage-default.example.com"},
						{"name": "us-east-1"},
						{"name": "us-east-2"},
					}),
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", cloudName),
					resource.TestCheckResourceAttr(resourceName, "type", "openstack"),
					resource.TestCheckResourceAttr(resourceName, "auth_types.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "auth_types.0", "userpass"),
					resource.TestCheckResourceAttr(resourceName, "auth_types.1", "access-key"),
					resource.TestCheckResourceAttr(resourceName, "endpoint", "https://cloud.example.com"),
					resource.TestCheckResourceAttr(resourceName, "identity_endpoint", "https://identity.example.com"),
					resource.TestCheckResourceAttr(resourceName, "storage_endpoint", "https://storage.example.com"),
					resource.TestCheckResourceAttr(resourceName, "ca_certificates.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "ca_certificates.0", ca1+"\n"),
					resource.TestCheckResourceAttr(resourceName, "ca_certificates.1", ca2+"\n"),
					resource.TestCheckResourceAttr(resourceName, "regions.#", "3"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.name", "default"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.endpoint", "https://region-default.example.com"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.identity_endpoint", "https://identity-default.example.com"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.storage_endpoint", "https://storage-default.example.com"),
					resource.TestCheckResourceAttr(resourceName, "regions.1.name", "us-east-1"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.1.endpoint"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.1.identity_endpoint"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.1.storage_endpoint"),
					resource.TestCheckResourceAttr(resourceName, "regions.2.name", "us-east-2"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.2.endpoint"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.2.identity_endpoint"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.2.storage_endpoint"),
				),
			},
			// We don't allow unsetting identity or storage endpoints, so verify our plan validator runs.
			{
				Config:   testAccResourceCloud_OpenStack_Minimal(cloudName),
				PlanOnly: true,
				ExpectError: regexp.MustCompile(
					`(?s)Unsupported change.*(identity_endpoint|storage_endpoint) cannot be unset once set \(workaround for a Juju limitation\)`,
				),
			},
		},
	})
}

func TestAcc_ResourceCloud_CACertsValidation(t *testing.T) {
	SkipJAAS(t)

	cloudName := acctest.RandomWithPrefix("tf-test-cloud-cacerts")

	// Intentionally invalid certificate content.
	invalidCert := "not-a-pem-cert"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCloudFromTemplate(internaltesting.TemplateData{
					"Name":             cloudName,
					"Type":             "openstack",
					"IncludeAuthTypes": true,
					"AuthTypesList":    hclList([]string{"userpass"}),
					"IncludeCACerts":   true,
					// Render a list with a single invalid cert string.
					"CACertsHCL":     "    \"" + invalidCert + "\"",
					"IncludeRegions": false,
				}),
				// |BEGIN CERTIFICATE| is a "best effort" / I'm paranoid as this is dependent on the error from the x509 parser.
				ExpectError: regexp.MustCompile(`(?s)ca_certificates\[0\].*(PEM-encoded X\.509 certificate|BEGIN CERTIFICATE)`),
			},
		},
	})
}

func testAccResourceCloud_OpenStack_Minimal(name string) string {
	return `
resource "juju_cloud" "` + name + `" {
  name = "` + name + `"
  type = "openstack"
  auth_types = ["userpass"]
}
`
}

// testAccCloudFromTemplate renders a juju_cloud resource using a named template and data.
// Provide fields in data to control omission vs empty and list contents.
// Supported keys in data:
// - Name (string), Type (string)
// - IncludeAuthTypes (bool), AuthTypesList (string, e.g. ["userpass", "access-key"])
// - IncludeEndpoint (bool), Endpoint (string)
// - IncludeIdentityEndpoint (bool), IdentityEndpoint (string)
// - IncludeStorageEndpoint (bool), StorageEndpoint (string)
// - IncludeCACerts (bool), CACertsHCL (string, e.g. <<-CERT blocks joined with commas)
// - IncludeRegions (bool), RegionsHCL (string, e.g. list of region objects)
func testAccCloudFromTemplate(data internaltesting.TemplateData) string {
	return internaltesting.GetStringFromTemplateWithData("testAccCloud", `
resource "juju_cloud" "{{.Name}}" {
  name = "{{.Name}}"
  type = "{{.Type}}"

  {{ if .IncludeAuthTypes }}
  auth_types = {{.AuthTypesList}}
  {{ end }}

  {{ if .IncludeEndpoint }}
  endpoint = "{{.Endpoint}}"
  {{ end }}
  {{ if .IncludeIdentityEndpoint }}
  identity_endpoint = "{{.IdentityEndpoint}}"
  {{ end }}
  {{ if .IncludeStorageEndpoint }}
  storage_endpoint  = "{{.StorageEndpoint}}"
  {{ end }}

  {{ if .IncludeCACerts }}
  ca_certificates = [
{{.CACertsHCL}}
  ]
  {{ end }}

  {{ if .IncludeRegions }}
  regions = [
{{.RegionsHCL}}
  ]
  {{ end }}
}
`, data)
}

// Helpers to build HCL fragments for lists to pass into the template.
func hclList(stringsList []string) string {
	if stringsList == nil {
		return "[]"
	}
	b := &strings.Builder{}
	b.WriteString("[")
	for i, v := range stringsList {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("\"" + v + "\"")
	}
	b.WriteString("]")
	return b.String()
}

func hclCACerts(certList []string) string {
	if len(certList) == 0 {
		return ""
	}
	b := &strings.Builder{}
	for i, c := range certList {
		b.WriteString("<<-CERT\n")
		b.WriteString(c)
		b.WriteString("\nCERT\n")
		if i < len(certList)-1 {
			b.WriteString(",\n")
		}
	}
	return b.String()
}

func hclRegions(regions []map[string]string) string {
	if len(regions) == 0 {
		return ""
	}
	b := &strings.Builder{}
	for i, r := range regions {
		b.WriteString("    {\n")
		if name := r["name"]; name != "" {
			b.WriteString("      name = \"" + name + "\"\n")
		}
		if ep := r["endpoint"]; ep != "" {
			b.WriteString("      endpoint = \"" + ep + "\"\n")
		}
		if iep := r["identity_endpoint"]; iep != "" {
			b.WriteString("      identity_endpoint = \"" + iep + "\"\n")
		}
		if sep := r["storage_endpoint"]; sep != "" {
			b.WriteString("      storage_endpoint  = \"" + sep + "\"\n")
		}
		b.WriteString("    }")
		if i < len(regions)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func newTestCA(t *testing.T, cn string) string {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"Pokemon League"},
			Locality:     []string{"Pallet Town"},
		},
		NotBefore: time.Now().Add(-time.Minute),
		NotAfter:  time.Now().Add(24 * time.Hour),

		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	pemCert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})

	return string(pemCert)
}

func testAccCheckCloudDestroy(cloudName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if TestClient == nil {
			return fmt.Errorf("TestClient is not configured")
		}

		_, err := TestClient.Clouds.ReadCloud(juju.ReadCloudInput{Name: cloudName})
		if err == nil {
			return fmt.Errorf("cloud %q still exists", cloudName)
		}

		// Juju not-found errors come back wrapped; treat any NotFound as successful destroy.
		if errors.Is(err, errors.NotFound) {
			return nil
		}

		return fmt.Errorf("error checking whether cloud %q was destroyed: %v", cloudName, err)
	}
}

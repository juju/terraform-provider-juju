// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/rpc/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internaljuju "github.com/juju/terraform-provider-juju/internal/juju"
)

var validUUID = regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)

func TestAcc_ResourceModel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	logLevelInfo := "INFO"
	logLevelDebug := "DEBUG"
	validVersion := regexp.MustCompile(`\d+\.\d+\.\d+`)

	resourceName := "juju_model.model"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), logLevelInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelInfo)),
					resource.TestMatchResourceAttr(resourceName, "uuid", validUUID),
					resource.TestMatchResourceAttr(resourceName, "agent_version", validVersion),
				),
			},
			{
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), logLevelDebug),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelDebug)),
				),
			},
			{
				Config: testAccConstraintsModel(modelName, testingCloud.CloudName(), "cores=1 mem=1024M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "constraints", "cores=1 mem=1024M"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      resourceName,
			},
		},
	})
}

func TestAcc_ResourceModel_UnsetConfig(t *testing.T) {
	if testingCloud == MicroK8sTesting {
		t.Skip(t.Name() + " skipped on microk8s: tests model config metadata, LXD is sufficient")
	}
	ctx := t.Context()

	modelName := acctest.RandomWithPrefix("tf-test-model")

	resourceName := "juju_model.this"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q

  config = {
	development = true
  }
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.development", "true"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckNoResourceAttr(resourceName, "config.development"),
					testAccCheckDevelopmentConfigIsUnset(ctx, "juju_model.this"),
					testAccCheckDevelopmentConfigIsUnset(ctx, "juju_model.this"),
				),
			},
		},
	})
}

func TestAcc_ResourceModel_UnsetConfigUsingNull(t *testing.T) {
	if testingCloud == MicroK8sTesting {
		t.Skip(t.Name() + " skipped on microk8s: tests model config metadata, LXD is sufficient")
	}
	ctx := t.Context()

	modelName := acctest.RandomWithPrefix("tf-test-model")

	resourceName := "juju_model.this"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q

  config = {
	development = true
	logging-config = "info"
  }
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.development", "true"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
  config = {
	development = null
	logging-config = "warn"
  }
}
`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckNoResourceAttr(resourceName, "config.development"),
					testAccCheckDevelopmentConfigIsUnset(ctx, resourceName),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
  config = {
	logging-config = "warn"
  }
}
`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckNoResourceAttr(resourceName, "config.development"),
					testAccCheckDevelopmentConfigIsUnset(ctx, resourceName),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
  config = {}
}
`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckNoResourceAttr(resourceName, "config.development"),
					testAccCheckDevelopmentConfigIsUnset(ctx, resourceName),
				),
			},
		},
	})
}

func TestAcc_ResourceModel_Minimal(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
				),
			},
		},
	})
}

func TestAcc_ResourceModel_TargetController(t *testing.T) {
	OnlyTestAgainstJAAS(t)

	testAccPreCheck(t)

	controllers, err := TestClient.Jaas.ListControllers(t.Context())
	if err != nil || len(controllers) == 0 {
		t.Fatalf("unable to list controllers from JAAS: %v", err)
	}
	targetController := controllers[0].Name

	modelName := acctest.RandomWithPrefix("tf-test-model")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q
  target_controller = %q
}`, modelName, targetController),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckResourceAttr("juju_model.testmodel", "target_controller", targetController),
				),
			},
		},
	})
}

func TestAcc_ResourceModel_TargetControllerValidation(t *testing.T) {
	SkipJAAS(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q
  target_controller = "some-controller"
}`, modelName),
				ExpectError: regexp.MustCompile(`(?s)The following field\(s\) can only be set when using JAAS.*\[target_controller\]`),
			},
		},
	})
}

func TestAcc_ResourceModel_UpgradeProvider(t *testing.T) {
	// This skip is temporary until we have a stable version of the provider that supports
	// Juju 4.0.0 and above, at which point we can re-enable it.
	SkipAgainstJuju4(t)
	modelName := acctest.RandomWithPrefix("tf-test-model")
	logLevelDebug := "DEBUG"

	resourceName := "juju_model.model"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: TestProviderStableVersion,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), logLevelDebug),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelDebug)),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceModel(modelName, testingCloud.CloudName(), logLevelDebug),
			},
		},
	})
}

func TestAcc_ResourceModel_Annotations_Basic(t *testing.T) {
	if testingCloud == MicroK8sTesting {
		t.Skip(t.Name() + " skipped on microk8s: tests model annotations metadata, LXD is sufficient")
	}
	modelName := acctest.RandomWithPrefix("tf-test-model")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsModel(modelName, "test", "test"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckResourceAttr("juju_model.testmodel", "annotations.test", "test"),
				),
			},
			{
				Config: testAccAnnotationsModel(modelName, "test", "test-update"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckResourceAttr("juju_model.testmodel", "annotations.test", "test-update"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckNoResourceAttr("juju_model.testmodel", "annotations.test"),
				),
			},
		},
	})
}

func TestAcc_ResourceModel_Annotations_DisjointedSet(t *testing.T) {
	if testingCloud == MicroK8sTesting {
		t.Skip(t.Name() + " skipped on microk8s: tests model annotations metadata, LXD is sufficient")
	}
	modelName := acctest.RandomWithPrefix("tf-test-model")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsModel(modelName, "test", "test"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckResourceAttr("juju_model.testmodel", "annotations.test", "test"),
				),
			},
			{
				Config: testAccAnnotationsModel(modelName, "test-another", "test-another"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckResourceAttr("juju_model.testmodel", "annotations.test-another", "test-another"),
					resource.TestCheckNoResourceAttr("juju_model.testmodel", "annotations.test"),
				),
			},
		},
	})
}

// TestAcc_ResourceModel_WaitForDelete tests that the model can be deleted and recreated successfully.
// It ensures that the model is properly cleaned up before the next creation attempt.
func TestAcc_ResourceModel_WaitForDelete(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	resourceName := "juju_model.model"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), "INFO"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestMatchResourceAttr(resourceName, "uuid", validUUID),
				),
			},
			{
				Config: " ",
			},
			{
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), "INFO"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestMatchResourceAttr(resourceName, "uuid", validUUID),
				),
			},
		},
	})
}

func TestAcc_ResourceModel_UpgradeAgentVersion(t *testing.T) {
	SkipAgainstJuju4(t)
	testAccPreCheck(t)

	targetAgentVersion := os.Getenv(TestJujuAgentVersion)
	if targetAgentVersion == "" {
		t.Skipf("%s is not set", TestJujuAgentVersion)
	}

	modelName := acctest.RandomWithPrefix("tf-test-model")
	resourceName := "juju_model.model"
	ctx := t.Context()

	modelResp, err := TestClient.Models.CreateModel(ctx, internaljuju.CreateModelInput{
		Name:        modelName,
		CloudName:   testingCloud.CloudName(),
		CloudRegion: "localhost",
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = TestClient.Models.DestroyModel(ctx, internaljuju.DestroyModelInput{UUID: modelResp.UUID})
	})

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             testAccResourceModel(modelName, testingCloud.CloudName(), "INFO"),
				ImportState:        true,
				ImportStateId:      modelResp.UUID,
				ImportStatePersist: true,
				ResourceName:       resourceName,
			},
			{
				Config: testAccResourceModelWithAgentVersion(modelName, testingCloud.CloudName(), targetAgentVersion),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestMatchResourceAttr(resourceName, "uuid", validUUID),
					resource.TestCheckResourceAttr(resourceName, "agent_version", targetAgentVersion),
				),
			},
		},
	})
}

func testAccCheckDevelopmentConfigIsUnset(ctx context.Context, resourceID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceID]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceID)
		}
		modelUUID := rs.Primary.Attributes["uuid"]
		if modelUUID == "" {
			return fmt.Errorf("uuid is empty in state")
		}
		conn, err := TestClient.Models.GetConnection(ctx, &modelUUID)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		// TODO: consider adding to client so we don't expose this layer (even in tests)
		modelconfigClient := modelconfig.NewClient(conn)

		metadata, err := modelconfigClient.ModelGetWithMetadata(context.Background())
		if err != nil {
			return err
		}

		actual, found := metadata["development"]
		if !found {
			// not set, which is what we want
			return nil
		}
		expected := params.ConfigValue{
			Value:  false,
			Source: "default",
		}
		if actual.Value != expected.Value || actual.Source != expected.Source {
			return fmt.Errorf("expecting 'development' config for model: %s, to be %#v but was: %#v",
				modelUUID, expected, actual)
		}
		return nil
	}
}

func testAccResourceModel(modelName string, cloudName string, logLevel string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q

  cloud {
   name   = %q
   region = "localhost"
  }

  config = {
    logging-config = "<root>=%s"
  }
}`, modelName, cloudName, logLevel)
}

func testAccResourceModelWithAgentVersion(modelName string, cloudName string, agentVersion string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
	name = %q

	cloud {
	 name   = %q
	 region = "localhost"
	}

	agent_version = %q
}`, modelName, cloudName, agentVersion)
}

func testAccConstraintsModel(modelName string, cloudName string, constraints string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q

  cloud {
   name   = %q
   region = "localhost"
  }

  constraints = "%s"
}`, modelName, cloudName, constraints)
}

func testAccAnnotationsModel(modelName string, annotationKey, annotationValue string) string {
	return fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q

  annotations = {
	%q = %q
  }
}`, modelName, annotationKey, annotationValue)
}

func TestAgentVersionCreateOnlyModifier(t *testing.T) {
	tests := []struct {
		name        string
		stateRaw    tftypes.Value
		configValue types.String
		wantError   bool
	}{
		{
			name:        "create with null config",
			stateRaw:    tftypes.NewValue(tftypes.String, nil),
			configValue: types.StringNull(),
			wantError:   false,
		},
		{
			name:        "create with configured value",
			stateRaw:    tftypes.NewValue(tftypes.String, nil),
			configValue: types.StringValue("4.0.0"),
			wantError:   true,
		},
		{
			name:        "update with configured value",
			stateRaw:    tftypes.NewValue(tftypes.String, "existing"),
			configValue: types.StringValue("4.0.0"),
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifier := AgentVersionCreateOnlyModifier()
			request := planmodifier.StringRequest{
				Path:        path.Root("agent_version"),
				ConfigValue: tt.configValue,
				State: tfsdk.State{
					Raw: tt.stateRaw,
				},
			}
			response := planmodifier.StringResponse{}

			modifier.PlanModifyString(t.Context(), request, &response)

			assert.Equal(t, tt.wantError, response.Diagnostics.HasError())
			if tt.wantError {
				require.Len(t, response.Diagnostics.Errors(), 1)
				assert.Equal(t, "Invalid agent_version for model creation", response.Diagnostics.Errors()[0].Summary())
			}
		})
	}
}

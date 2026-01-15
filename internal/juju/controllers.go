// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/api/connector"
	controllerapi "github.com/juju/juju/api/controller/controller"
	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/juju/juju/osenv"
	"github.com/juju/juju/jujuclient"
	"github.com/juju/version/v2"
	"gopkg.in/yaml.v2"
)

const LogJujuCommand = "juju_command"

// ControllerConnectionInformation contains the connection details for a controller.
type ControllerConnectionInformation struct {
	Addresses []string
	CACert    string
	Username  string
	Password  string
}

// CommandRunner defines the interface for executing juju commands.
type CommandRunner interface {
	// SetEnv sets an environment variable for command execution.
	SetEnv(key, value string)
	// Run executes a juju command with the configured environment and logging.
	Run(ctx context.Context, args ...string) error
	// LogFilePath returns the path to the log file.
	LogFilePath() string
}

// commandRunner manages command execution with environment variables and logging.
type commandRunner struct {
	jujuBinary  string
	logFilePath string
	envVars     map[string]string
}

// newCommandRunner creates a new command runner with a log file in a temp directory.
func newCommandRunner(jujuBinary string) (*commandRunner, error) {
	logFile, err := os.CreateTemp("", "juju-bootstrap-log-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}
	logFilePath := logFile.Name()
	logFile.Close()

	return &commandRunner{
		jujuBinary:  jujuBinary,
		logFilePath: logFilePath,
		envVars:     make(map[string]string),
	}, nil
}

// SetEnv sets an environment variable for command execution.
func (r *commandRunner) SetEnv(key, value string) {
	r.envVars[key] = value
}

// Run executes a juju command with the configured environment and logging.
func (r *commandRunner) Run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, r.jujuBinary, args...)

	// Open log file in append mode
	logFile, err := os.OpenFile(r.logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	// Write command being executed to log
	if _, err := logFile.WriteString(fmt.Sprintf("\n=== Executing: %s %s ===\n", r.jujuBinary, strings.Join(args, " "))); err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	// Redirect stdout and stderr to log file
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Build environment vars
	for k, v := range r.envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed (see log file %s): %w", r.logFilePath, err)
	}

	return nil
}

// LogFilePath returns the path to the log file.
func (r *commandRunner) LogFilePath() string {
	return r.logFilePath
}

// BootstrapConfig contains all configuration options that can be set during bootstrap.
// These are divided into controller configuration, controller model configuration,
// and bootstrap configuration.
type BootstrapConfig struct {
	// Controller configuration
	ControllerConfig map[string]string
	// Controller model config
	ControllerModelConfig map[string]string
	// BootstrapConfig contains bootstrap configuration options
	BootstrapConfig map[string]string
}

// BootstrapFlags contains CLI flags for the bootstrap command.
// The flag struct tags indicate the corresponding CLI flag names.
type BootstrapFlags struct {
	AgentVersion         string   `flag:"agent-version"`
	BootstrapBase        string   `flag:"bootstrap-base"`
	BootstrapConstraints []string `flag:"bootstrap-constraints"`
	ModelConstraints     []string `flag:"constraints"`
	ModelDefault         []string `flag:"model-default"`
	StoragePool          []string `flag:"storage-pool"`
}

// BootstrapArguments contains all the arguments needed for bootstrap.
type BootstrapArguments struct {
	Name            string
	JujuBinary      string
	Cloud           BootstrapCloudArgument
	CloudCredential BootstrapCredentialArgument
	Config          BootstrapConfig
	Flags           BootstrapFlags
}

// BootstrapCloudArgument contains cloud configuration for bootstrap.
type BootstrapCloudArgument struct {
	Name            string
	AuthTypes       []string
	CACertificates  []string
	Config          map[string]string
	Endpoint        string
	HostCloudRegion string
	Region          *BootstrapCloudRegionArgument
	Type            string
	K8sConfig       string
}

// BootstrapCloudRegionArgument contains cloud region configuration.
type BootstrapCloudRegionArgument struct {
	Name             string
	Endpoint         string
	IdentityEndpoint string
	StorageEndpoint  string
}

// BootstrapCredentialArgument contains credential configuration for bootstrap.
type BootstrapCredentialArgument struct {
	Name       string
	AuthType   string
	Attributes map[string]string
}

// DefaultJujuCommand is the default implementation of JujuCommand.
type DefaultJujuCommand struct {
	jujuBinary string
}

// NewDefaultJujuCommand creates a new DefaultJujuCommand instance.
func NewDefaultJujuCommand(jujuBinary string) (*DefaultJujuCommand, error) {
	return &DefaultJujuCommand{jujuBinary: jujuBinary}, nil
}

// Bootstrap creates a new controller and returns connection information.
func (d *DefaultJujuCommand) Bootstrap(ctx context.Context, args BootstrapArguments) (*ControllerConnectionInformation, error) {
	// Validate arguments
	if args.Name == "" {
		return nil, fmt.Errorf("controller name cannot be empty")
	}
	if args.Cloud.Name == "" {
		return nil, fmt.Errorf("cloud name cannot be empty")
	}
	if args.CloudCredential.Name == "" {
		return nil, fmt.Errorf("credential name cannot be empty")
	}
	if args.Flags.AgentVersion != "" {
		if _, err := version.Parse(args.Flags.AgentVersion); err != nil {
			return nil, fmt.Errorf("invalid agent version %q: %w", args.Flags.AgentVersion, err)
		}
	}

	// Create temporary JUJU_DATA directory
	tmpDir, err := os.MkdirTemp("", "juju-bootstrap-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary JUJU_DATA directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create command runner with log file
	runner, err := newCommandRunner(d.jujuBinary)
	if err != nil {
		return nil, err
	}

	// Set JUJU_DATA environment variable for the running process
	// so that store related commands use the temporary directory.
	oldJujuData := osenv.SetJujuXDGDataHome(tmpDir)
	defer osenv.SetJujuXDGDataHome(oldJujuData)

	// Also set it for the command runner
	runner.SetEnv("JUJU_DATA", tmpDir)

	tflog.SubsystemDebug(ctx, LogJujuCommand, fmt.Sprintf("Bootstrap log file: %s\n", runner.LogFilePath()))

	return performBootstrap(ctx, args, tmpDir, runner)
}

// performBootstrap executes the actual bootstrap logic with the provided command runner.
// This function is separated to allow for easier testing with a mock command runner.
func performBootstrap(ctx context.Context, args BootstrapArguments, tmpDir string, runner CommandRunner) (*ControllerConnectionInformation, error) {
	// Update public clouds - this command will go fetch a list of public clouds
	// from https://streams.canonical.com/juju/public-clouds.syaml and update
	// client's local store.
	if err := runner.Run(ctx, "update-public-clouds", "--client"); err != nil {
		return nil, fmt.Errorf("failed to update public clouds: %w", err)
	}

	// Setup cloud
	cloudName := args.Cloud.Name
	isPublicCloud, err := isValidPublicCloud(args)
	if err != nil {
		return nil, fmt.Errorf("failed to validate cloud: %w", err)
	}

	// If a cloud is not known to Juju i.e. clouds besides AWS, Azure, GCP, etc.,
	// then we need to create a cloud entry on disk with information on how
	// to reach the cloud, its regions, etc.
	if !isPublicCloud {
		// Create personal cloud
		cloud := buildJujuCloud(args.Cloud)
		if err := jujucloud.WritePersonalCloudMetadata(map[string]jujucloud.Cloud{
			cloudName: cloud,
		}); err != nil {
			return nil, fmt.Errorf("failed to write personal cloud metadata: %w", err)
		}
	}

	// Setup credentials
	store := jujuclient.NewFileClientStore()
	cloudCred := jujucloud.CloudCredential{
		AuthCredentials: map[string]jujucloud.Credential{
			cloudName: buildJujuCredential(args.CloudCredential),
		},
	}
	if err := store.UpdateCredential(cloudName, cloudCred); err != nil {
		return nil, fmt.Errorf("failed to update credential: %w", err)
	}

	// Write config file
	configFilePath, err := writeBootstrapConfigs(tmpDir, args.Config)
	if err != nil {
		return nil, err
	}

	// Build bootstrap command arguments
	bootstrapArgs, err := buildBootstrapArgs(ctx, args, configFilePath)
	if err != nil {
		return nil, err
	}

	// Execute bootstrap command
	if err := runner.Run(ctx, bootstrapArgs...); err != nil {
		return nil, fmt.Errorf("bootstrap failed: %w", err)
	}

	// Read controller information from the client store
	controllerDetails, err := store.ControllerByName(args.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to read controller details from client store: %w", err)
	}

	accountDetails, err := store.AccountDetails(args.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to read account details from client store: %w", err)
	}

	return &ControllerConnectionInformation{
		Addresses: controllerDetails.APIEndpoints,
		CACert:    controllerDetails.CACert,
		Username:  accountDetails.User,
		Password:  accountDetails.Password,
	}, nil
}

// UpdateConfig updates controller configuration.
func (d *DefaultJujuCommand) UpdateConfig(
	ctx context.Context,
	connInfo *ControllerConnectionInformation,
	controllerConfig, controllerModelConfig map[string]string,
	controllerModelConfigUnset []string,
) error {
	// Connect to the controller
	connr, err := connector.NewSimple(connector.SimpleConfig{
		ControllerAddresses: connInfo.Addresses,
		CACert:              connInfo.CACert,
		Username:            connInfo.Username,
		Password:            connInfo.Password,
	})
	if err != nil {
		return err
	}

	conn, err := connr.Connect()
	if err != nil {
		return err
	}

	ctrlClient := controllerapi.NewClient(conn)
	ctrlConfigVals := make(map[string]any)
	for k, v := range controllerConfig {
		ctrlConfigVals[k] = v
	}

	// Update controller config
	if len(ctrlConfigVals) > 0 {
		if err := ctrlClient.ConfigSet(ctrlConfigVals); err != nil {
			return fmt.Errorf("failed to update controller config: %w", err)
		}
	}

	modelCfgClient := modelconfig.NewClient(conn)
	modelConfigVals := make(map[string]any)
	for k, v := range controllerModelConfig {
		modelConfigVals[k] = v
	}

	// Update model config
	if len(modelConfigVals) > 0 {
		if err := modelCfgClient.ModelSet(modelConfigVals); err != nil {
			return fmt.Errorf("failed to update controller model config: %w", err)
		}
	}
	if len(controllerModelConfigUnset) > 0 {
		if err := modelCfgClient.ModelUnset(controllerModelConfigUnset...); err != nil {
			return fmt.Errorf("failed to unset controller model config keys: %w", err)
		}
	}

	return nil
}

// Config retrieves controller configuration and controller-model configuration settings.
func (d *DefaultJujuCommand) Config(ctx context.Context, connInfo *ControllerConnectionInformation) (map[string]any, map[string]any, error) {
	// Connect to the controller
	connr, err := connector.NewSimple(connector.SimpleConfig{
		ControllerAddresses: connInfo.Addresses,
		CACert:              connInfo.CACert,
		Username:            connInfo.Username,
		Password:            connInfo.Password,
	})
	if err != nil {
		return nil, nil, err
	}

	conn, err := connr.Connect()
	if err != nil {
		return nil, nil, err
	}

	// Fetch controller config
	ctrlClient := controllerapi.NewClient(conn)
	ctrlConfig, err := ctrlClient.ControllerConfig()
	if err != nil {
		return nil, nil, err
	}

	modelCfgClient := modelconfig.NewClient(conn)
	modelConfig, err := modelCfgClient.ModelGet()
	if err != nil {
		return nil, nil, err
	}

	return ctrlConfig, modelConfig, nil
}

// Destroy removes the controller.
func (d *DefaultJujuCommand) Destroy(ctx context.Context, connInfo *ControllerConnectionInformation) error {
	// TODO: Implement destroy logic
	return fmt.Errorf("not implemented")
}

// writeBootstrapConfigs writes the bootstrap configs to a single YAML file.
func writeBootstrapConfigs(workDir string, config BootstrapConfig) (string, error) {
	// Skip if config is empty
	if len(config.ControllerConfig) == 0 && len(config.ControllerModelConfig) == 0 && len(config.BootstrapConfig) == 0 {
		return "", nil
	}

	// Take all the config maps and write their values as a combined yaml
	// to the bootstrap config file.

	configFilePath := filepath.Join(workDir, "bootstrap-config.yaml")
	combinedConfig := make(map[string]any)

	for k, v := range config.ControllerConfig {
		combinedConfig[k] = v
	}
	for k, v := range config.ControllerModelConfig {
		combinedConfig[k] = v
	}
	for k, v := range config.BootstrapConfig {
		combinedConfig[k] = v
	}

	data, err := yaml.Marshal(combinedConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal bootstrap config to yaml: %w", err)
	}

	if err := os.WriteFile(configFilePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	return configFilePath, nil
}

// buildBootstrapArgs constructs the bootstrap command arguments from BootstrapArguments using reflection for flags.
func buildBootstrapArgs(ctx context.Context, args BootstrapArguments, configFilePath string) ([]string, error) {
	cmdArgs := []string{"bootstrap"}

	// Add flags using reflection
	flagsValue := reflect.ValueOf(args.Flags)
	flagsType := reflect.TypeOf(args.Flags)

	for i := 0; i < flagsType.NumField(); i++ {
		field := flagsType.Field(i)
		flagTag := field.Tag.Get("flag")
		if flagTag == "" {
			continue
		}

		fieldValue := flagsValue.Field(i)

		// Handle different types
		switch fieldValue.Kind() {
		case reflect.String:
			if str := fieldValue.String(); str != "" {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", flagTag, str))
			}
		case reflect.Slice:
			if fieldValue.Len() > 0 {
				// For slices, add multiple flags
				for j := 0; j < fieldValue.Len(); j++ {
					item := fieldValue.Index(j)
					if item.Kind() == reflect.String {
						cmdArgs = append(cmdArgs, fmt.Sprintf("--%s %s", flagTag, item.String()))
					}
				}
			}
		default:
			// Log unhandled field types for debugging
			if !fieldValue.IsZero() {
				tflog.SubsystemWarn(ctx, LogJujuCommand, fmt.Sprintf("unhandled flag field type %s for flag %s\n", fieldValue.Kind(), flagTag))
			}
		}
	}

	// Add config file if it exists
	if configFilePath != "" {
		cmdArgs = append(cmdArgs, "--config", configFilePath)
	}

	cloudRegion := args.Cloud.Name
	if args.Cloud.Region != nil {
		cloudRegion = fmt.Sprintf("%s/%s", args.Cloud.Name, args.Cloud.Region.Name)
	}

	// Add cloud name and controller name (must be at the end)
	cmdArgs = append(cmdArgs, cloudRegion, args.Name)

	return cmdArgs, nil
}

// buildJujuCloud constructs a jujucloud.Cloud from BootstrapCloudArgument.
func buildJujuCloud(cloudArg BootstrapCloudArgument) jujucloud.Cloud {
	// Convert string map to interface map for Config
	config := make(map[string]interface{})
	for k, v := range cloudArg.Config {
		config[k] = v
	}

	cloud := jujucloud.Cloud{
		Name:            cloudArg.Name,
		Type:            cloudArg.Type,
		AuthTypes:       convertToCloudAuthTypes(cloudArg.AuthTypes),
		Endpoint:        cloudArg.Endpoint,
		HostCloudRegion: cloudArg.HostCloudRegion,
		Config:          config,
		CACertificates:  cloudArg.CACertificates,
	}

	if cloudArg.Region != nil {
		cloud.Regions = []jujucloud.Region{
			{
				Name:             cloudArg.Region.Name,
				Endpoint:         cloudArg.Region.Endpoint,
				IdentityEndpoint: cloudArg.Region.IdentityEndpoint,
				StorageEndpoint:  cloudArg.Region.StorageEndpoint,
			},
		}
	}

	return cloud
}

// buildJujuCredential constructs a jujucloud.Credential from BootstrapCredentialArgument.
func buildJujuCredential(credArg BootstrapCredentialArgument) jujucloud.Credential {
	return jujucloud.NewCredential(jujucloud.AuthType(credArg.AuthType), credArg.Attributes)
}

// convertToCloudAuthTypes converts string auth types to jujucloud.AuthType.
func convertToCloudAuthTypes(authTypes []string) []jujucloud.AuthType {
	result := make([]jujucloud.AuthType, len(authTypes))
	for i, authType := range authTypes {
		result[i] = jujucloud.AuthType(authType)
	}
	return result
}

// isValidPublicCloud checks if the cloud name (and possibly region) is a valid public cloud.
func isValidPublicCloud(args BootstrapArguments) (bool, error) {
	pubClouds, _, err := jujucloud.PublicCloudMetadata(jujucloud.JujuPublicCloudsPath())
	if err != nil {
		return false, fmt.Errorf("failed to get public cloud metadata: %w", err)
	}

	for pubCloudName, cloud := range pubClouds {
		if args.Cloud.Name == pubCloudName {
			if args.Cloud.Region != nil {
				regionName := args.Cloud.Region.Name
				exists := slices.ContainsFunc(cloud.Regions, func(r jujucloud.Region) bool {
					return regionName == r.Name
				})
				if !exists {
					return false, fmt.Errorf("invalid public cloud region for cloud %s with region %s", args.Cloud.Name, regionName)
				}
			}
			return true, nil
		}
	}

	return false, nil
}

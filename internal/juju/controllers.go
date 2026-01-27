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
	Addresses      []string
	CACert         string
	Username       string
	Password       string
	AgentVersion   string
	ControllerUUID string
}

// CommandRunner defines the interface for executing juju commands.
type CommandRunner interface {
	// Run executes a juju command with the configured environment and logging.
	Run(ctx context.Context, args ...string) error
	// Version returns the juju CLI version.
	Version(ctx context.Context) (string, error)
	// LogFilePath returns the path to the log file.
	LogFilePath() string
	// WorkingDir returns the temporary directory created by the runner.
	WorkingDir() string
	// SetClientGlobal sets the Juju client global Juju data home variable.
	SetClientGlobal()
	// UnsetClientGlobal sets the Juju client global Juju data home variable to its previous value.
	UnsetClientGlobal()
	// Close cleans up working directory.
	Close() error
}

// commandRunner manages command execution with environment variables and logging.
type commandRunner struct {
	jujuBinary  string
	logFilePath string
	workingDir  string
	oldJujuData string
}

// newCommandRunner creates a new command runner with a log file in a temp directory.
func newCommandRunner(jujuBinary string) (*commandRunner, error) {
	logFile, err := os.CreateTemp("", "juju-bootstrap-log-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Create temporary JUJU_DATA directory
	tmpDir, err := os.MkdirTemp("", "juju-bootstrap-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary JUJU_DATA directory: %w", err)
	}

	return &commandRunner{
		jujuBinary:  jujuBinary,
		logFilePath: logFile.Name(),
		workingDir:  tmpDir,
	}, nil
}

// Close cleans up working directory.
func (r *commandRunner) Close() error {
	if err := os.RemoveAll(r.workingDir); err != nil {
		return err
	}
	return nil
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
	cmd.Env = append(cmd.Env, fmt.Sprintf("JUJU_DATA=%s", r.workingDir))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed (see log file %s): %w", r.logFilePath, err)
	}

	return nil
}

func (r *commandRunner) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, r.jujuBinary, "--version")

	// Build environment vars
	cmd.Env = append(cmd.Env, fmt.Sprintf("JUJU_DATA=%s", r.workingDir))

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get juju version: %w", err)
	}

	versionStr := strings.TrimSpace(string(output))
	return versionStr, nil
}

// LogFilePath returns the path to the log file.
func (r *commandRunner) LogFilePath() string {
	return r.logFilePath
}

// WorkingDir returns the working directory for the command runner.
func (r *commandRunner) WorkingDir() string {
	return r.workingDir
}

// SetClientGlobal sets the Juju client global Juju data home variable.
// The caller must UnsetClientGlobal after making filestore calls, ideally in a defer.
func (r *commandRunner) SetClientGlobal() {
	// Set Juju data home for a limited period of using the filestore.
	// This is necessary because internally, store and other methods
	// look up a global set with osenv.
	r.oldJujuData = osenv.SetJujuXDGDataHome(r.workingDir)
}

// UnsetClientGlobal sets the Juju client global Juju data home variable to its previous value.
func (r *commandRunner) UnsetClientGlobal() {
	osenv.SetJujuXDGDataHome(r.oldJujuData)
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

type DestroyFlags struct {
	DestroyAllModels bool `flag:"destroy-all-models"`
	DestroyStorage   bool `flag:"destroy-storage"`
	ReleaseStorage   bool `flag:"release-storage"`
	Force            bool `flag:"force"`
	ModelTimeout     int  `flag:"model-timeout"`
}

type DestroyArguments struct {
	Name           string
	JujuBinary     string
	CloudName      string
	CloudRegion    string
	ConnectionInfo ControllerConnectionInformation
	Flags          DestroyFlags
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

	// Create command runner with log file
	runner, err := newCommandRunner(d.jujuBinary)
	if err != nil {
		return nil, err
	}
	defer runner.Close()

	tflog.SubsystemDebug(ctx, LogJujuCommand, fmt.Sprintf("Bootstrap log file: %s\n", runner.LogFilePath()))

	return performBootstrap(ctx, args, runner)
}

// performBootstrap executes the actual bootstrap logic with the provided command runner.
// This function is separated to allow for easier testing with a mock command runner.
func performBootstrap(ctx context.Context, args BootstrapArguments, runner CommandRunner) (*ControllerConnectionInformation, error) {
	err := setupCloudWithCredentials(ctx, runner, args.Cloud, args.CloudCredential)
	if err != nil {
		return nil, fmt.Errorf("failed to setup cloud and credentials: %w", err)
	}

	// Write config file
	configFilePath, err := writeBootstrapConfigs(runner.WorkingDir(), args.Config)
	if err != nil {
		return nil, err
	}
	// Build bootstrap command arguments
	cmdArgs, err := buildBootstrapArgs(ctx, args, configFilePath)
	if err != nil {
		return nil, err
	}

	// Execute bootstrap command
	if err := runner.Run(ctx, cmdArgs...); err != nil {
		return nil, fmt.Errorf("bootstrap failed: %w", err)
	}

	// Client store to read controller information after bootstrap
	runner.SetClientGlobal()
	defer runner.UnsetClientGlobal()
	store := jujuclient.NewFileClientStore()

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
		Addresses:      controllerDetails.APIEndpoints,
		CACert:         controllerDetails.CACert,
		Username:       accountDetails.User,
		Password:       accountDetails.Password,
		AgentVersion:   controllerDetails.AgentVersion,
		ControllerUUID: controllerDetails.ControllerUUID,
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
func (d *DefaultJujuCommand) Destroy(ctx context.Context, args DestroyArguments) error {
	// Validate arguments
	if args.Name == "" {
		return fmt.Errorf("controller name cannot be empty")
	}
	if args.CloudName == "" {
		return fmt.Errorf("cloud name cannot be empty")
	}
	if args.CloudRegion == "" {
		return fmt.Errorf("cloud region cannot be empty")
	}
	if args.ConnectionInfo.AgentVersion != "" {
		if _, err := version.Parse(args.ConnectionInfo.AgentVersion); err != nil {
			return fmt.Errorf("invalid agent version %q: %w", args.ConnectionInfo.AgentVersion, err)
		}
	}

	// Create command runner with log file
	runner, err := newCommandRunner(d.jujuBinary)
	if err != nil {
		return err
	}
	defer runner.Close()

	// Check Juju CLI version matches agent version
	cliVersion, err := runner.Version(ctx)
	if err != nil {
		return fmt.Errorf("failed to get juju version: %w", err)
	}

	parsedVersion, err := version.Parse(cliVersion)
	if err != nil {
		return fmt.Errorf("invalid juju version %q: %w", cliVersion, err)
	}

	agentVersion, err := version.Parse(args.ConnectionInfo.AgentVersion)
	if err != nil {
		return fmt.Errorf("invalid agent version %q: %w", args.ConnectionInfo.AgentVersion, err)
	}

	if parsedVersion != agentVersion {
		return fmt.Errorf("Juju CLI version (%s) does not match agent version (%s)", cliVersion, args.ConnectionInfo.AgentVersion)
	}

	tflog.SubsystemDebug(ctx, LogJujuCommand, fmt.Sprintf("Destroy log file: %s\n", runner.LogFilePath()))

	return performDestroy(ctx, args, runner)
}

func performDestroy(ctx context.Context, args DestroyArguments, runner CommandRunner) error {
	err := setupControllerConnectionInfo(ctx, runner, args)
	if err != nil {
		return fmt.Errorf("failed to setup controller client store: %w", err)
	}

	cmdArgs, err := buildDestroyArgs(ctx, args)
	if err != nil {
		return err
	}

	err = runner.Run(ctx, cmdArgs...)
	if err != nil {
		return fmt.Errorf("destroy failed: %w", err)
	}

	return nil
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

	cmdArgs = append(cmdArgs, buildArgsFromFlags(ctx, args.Flags)...)

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

// buildArgsFromFlags builds command line arguments from the provided flags-like struct using reflection.
func buildArgsFromFlags(ctx context.Context, flags any) []string {
	var cmdArgs []string

	// Add flags using reflection
	flagsValue := reflect.ValueOf(flags)
	flagsType := reflect.TypeOf(flags)

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

	return cmdArgs
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
func isValidPublicCloud(arg BootstrapCloudArgument) (bool, error) {
	pubClouds, _, err := jujucloud.PublicCloudMetadata(jujucloud.JujuPublicCloudsPath())
	if err != nil {
		return false, fmt.Errorf("failed to get public cloud metadata: %w", err)
	}

	for pubCloudName, cloud := range pubClouds {
		if arg.Name == pubCloudName {
			if arg.Region != nil {
				regionName := arg.Region.Name
				exists := slices.ContainsFunc(cloud.Regions, func(r jujucloud.Region) bool {
					return regionName == r.Name
				})
				if !exists {
					return false, fmt.Errorf("invalid public cloud region for cloud %s with region %s", arg.Name, regionName)
				}
			}
			return true, nil
		}
	}

	return false, nil
}

func setupCloudWithCredentials(ctx context.Context, runner CommandRunner, cloud BootstrapCloudArgument, credential BootstrapCredentialArgument) error {
	// Update public clouds - this command will go fetch a list of public clouds
	// from https://streams.canonical.com/juju/public-clouds.syaml and update
	// client's local store.
	if err := runner.Run(ctx, "update-public-clouds", "--client"); err != nil {
		return fmt.Errorf("failed to update public clouds: %w", err)
	}

	runner.SetClientGlobal()
	defer runner.UnsetClientGlobal()
	store := jujuclient.NewFileClientStore()

	// Setup cloud
	cloudName := cloud.Name
	isPublicCloud, err := isValidPublicCloud(cloud)
	if err != nil {
		return fmt.Errorf("failed to validate cloud: %w", err)
	}

	// If a cloud is not known to Juju i.e. clouds besides AWS, Azure, GCP, etc.,
	// then we need to create a cloud entry on disk with information on how
	// to reach the cloud, its regions, etc.
	if !isPublicCloud {
		// Create personal cloud
		cloud := buildJujuCloud(cloud)
		if err := jujucloud.WritePersonalCloudMetadata(map[string]jujucloud.Cloud{
			cloudName: cloud,
		}); err != nil {
			return fmt.Errorf("failed to write personal cloud metadata: %w", err)
		}
	}

	// Setup credentials
	cloudCred := jujucloud.CloudCredential{
		AuthCredentials: map[string]jujucloud.Credential{
			cloudName: buildJujuCredential(credential),
		},
	}

	if err := store.UpdateCredential(cloudName, cloudCred); err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}

	return nil
}

// setupControllerConnectionInfo sets up the client store with controller and account details before destroy
func setupControllerConnectionInfo(_ context.Context, runner CommandRunner, args DestroyArguments) error {
	runner.SetClientGlobal()
	defer runner.UnsetClientGlobal()
	store := jujuclient.NewFileClientStore()

	err := store.AddController(args.Name, jujuclient.ControllerDetails{
		ControllerUUID: args.ConnectionInfo.ControllerUUID,
		Cloud:          args.CloudName,
		CloudRegion:    args.CloudRegion,
		APIEndpoints:   args.ConnectionInfo.Addresses,
		CACert:         args.ConnectionInfo.CACert,
	})
	if err != nil {
		return fmt.Errorf("failed to add controller to client store: %w", err)
	}

	err = store.UpdateAccount(args.Name, jujuclient.AccountDetails{
		User:     args.ConnectionInfo.Username,
		Password: args.ConnectionInfo.Password,
	})
	if err != nil {
		return fmt.Errorf("failed to add account details to client store: %w", err)
	}
	return nil
}

func buildDestroyArgs(ctx context.Context, args DestroyArguments) ([]string, error) {
	cmdArgs := []string{"destroy-controller", args.Name}

	cmdArgs = append(cmdArgs, buildArgsFromFlags(ctx, args.Flags)...)

	return cmdArgs, nil
}

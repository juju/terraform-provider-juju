package main

import (
	"context"
	"flag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-mux/tf6muxserver"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/juju/terraform-provider-juju/internal/provider"
)

// Run "go generate" to format example terraform files and generate the docs for the registry/website

// If you do not have terraform installed, you can remove the formatting command, but it's suggested to
// ensure the documentation is formatted properly.
//go:generate terraform fmt -recursive ./examples/

// Run the docs generation tool, check its repository for more information on how it works and how docs
// can be customized.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary
	version string = "dev"

	// goreleaser can also pass the specific commit if you want
	// commit  string = ""
)

func main() {
	ctx := context.Background()

	var debugMode bool

	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	upgradedSdkProvider, err := tf5to6server.UpgradeServer(
		ctx,
		provider.New(version)().GRPCProvider,
	)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	providers := []func() tfprotov6.ProviderServer{
		providerserver.NewProtocol6(provider.NewJujuProvider(version)),
		func() tfprotov6.ProviderServer {
			return upgradedSdkProvider
		},
	}

	muxServer, err := tf6muxserver.NewMuxServer(ctx, providers...)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	var serveOpts []tf6server.ServeOpt

	if debugMode {
		serveOpts = append(serveOpts, tf6server.WithManagedDebug())
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if err = tf6server.Serve(
		"registry.terraform.io/juju/juju",
		muxServer.ProviderServer,
		serveOpts...,
	); err != nil {
		log.Fatal().Msg(err.Error())
	}
}

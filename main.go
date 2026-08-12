// terraform-provider-kener is a Terraform provider for the Kener status-page
// tool (https://kener.ing). It is served over the terraform-plugin-framework
// protocol v6.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/quarks-tech/terraform-provider-kener/internal/provider"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name kener

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/quarks-tech/kener",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}

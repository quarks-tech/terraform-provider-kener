package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories are used to instantiate the provider during
// acceptance testing. The factory is called for each Terraform CLI command to
// create a provider server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"kener": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates that the environment needed for acceptance tests is
// present. Acceptance tests hit a real Kener instance; run them with:
//
//	TF_ACC=1 KENER_ENDPOINT=http://localhost:3000 KENER_API_TOKEN=kener_... go test ./internal/provider/...
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv(envEndpoint) == "" {
		t.Fatalf("%s must be set for acceptance tests", envEndpoint)
	}
	if os.Getenv(envAPIToken) == "" {
		t.Fatalf("%s must be set for acceptance tests", envAPIToken)
	}
}

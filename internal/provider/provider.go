package provider

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

// Environment variables used as fallbacks for provider configuration.
const (
	envEndpoint = "KENER_ENDPOINT"
	envAPIToken = "KENER_API_TOKEN"
)

// Ensure the provider satisfies the framework interfaces.
var _ provider.Provider = (*kenerProvider)(nil)

// kenerProvider is the provider implementation.
type kenerProvider struct {
	// version is set by the build (e.g. "dev", or a release tag).
	version string
}

// kenerProviderModel maps provider schema data to a Go type.
type kenerProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIToken types.String `tfsdk:"api_token"`
}

// New returns a function that instantiates the provider, as required by
// providerserver.Serve.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &kenerProvider{version: version}
	}
}

func (p *kenerProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "kener"
	resp.Version = p.version
}

func (p *kenerProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Kener provider manages resources on a [Kener](https://kener.ing) status-page instance via its `/api/v4` REST API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Base URL of the Kener instance, including scheme and host (e.g. `https://status.example.com`). The `/api/v4` path is appended automatically. May also be set via the `" + envEndpoint + "` environment variable.",
				Optional:            true,
			},
			"api_token": schema.StringAttribute{
				MarkdownDescription: "Kener API token (`kener_...`), created in the Kener admin UI under Settings → API Keys. May also be set via the `" + envAPIToken + "` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *kenerProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config kenerProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Unknown values mean config depends on other resources that aren't known
	// yet at configure time.
	if config.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"),
			"Unknown Kener endpoint",
			"The provider cannot be configured while the endpoint value is unknown. Set a static value or the "+envEndpoint+" environment variable.")
	}
	if config.APIToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("api_token"),
			"Unknown Kener API token",
			"The provider cannot be configured while the api_token value is unknown. Set a static value or the "+envAPIToken+" environment variable.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Precedence: explicit config value, then environment variable.
	endpoint := strings.TrimSpace(os.Getenv(envEndpoint))
	if !config.Endpoint.IsNull() {
		endpoint = strings.TrimSpace(config.Endpoint.ValueString())
	}
	token := strings.TrimSpace(os.Getenv(envAPIToken))
	if !config.APIToken.IsNull() {
		token = strings.TrimSpace(config.APIToken.ValueString())
	}

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"),
			"Missing Kener endpoint",
			"Set the provider `endpoint` attribute or the "+envEndpoint+" environment variable.")
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_token"),
			"Missing Kener API token",
			"Set the provider `api_token` attribute or the "+envAPIToken+" environment variable.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := client.New(endpoint, token, client.WithUserAgent("terraform-provider-kener/"+p.version))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Kener API client", err.Error())
		return
	}

	// Hand the client to resources and data sources.
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *kenerProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMonitorResource,
		NewPageResource,
		NewIncidentResource,
		NewIncidentCommentResource,
		NewMaintenanceResource,
		NewSiteConfigResource,
	}
}

func (p *kenerProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewMonitorDataSource,
		NewPageDataSource,
		NewIncidentDataSource,
		NewMaintenanceDataSource,
		NewSiteConfigDataSource,
	}
}

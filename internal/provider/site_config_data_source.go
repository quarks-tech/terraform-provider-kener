package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

var (
	_ datasource.DataSource              = (*siteConfigDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*siteConfigDataSource)(nil)
)

// NewSiteConfigDataSource is the data source constructor registered with the provider.
func NewSiteConfigDataSource() datasource.DataSource {
	return &siteConfigDataSource{}
}

type siteConfigDataSource struct {
	client *client.Client
}

func (d *siteConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_config"
}

func (d *siteConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single Kener global site-configuration value by key.",
		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				MarkdownDescription: "The site-configuration key to read.",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The value as JSON.",
				Computed:            true,
				CustomType:          jsontypes.NormalizedType{},
			},
			"data_type": schema.StringAttribute{
				MarkdownDescription: "The value's data type (`string` or `object`).",
				Computed:            true,
			},
		},
	}
}

func (d *siteConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	d.client = c
}

func (d *siteConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteConfigResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	entry, err := d.client.GetSiteConfig(ctx, data.Key.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading site config", fmt.Sprintf("Could not read site config %q: %s", data.Key.ValueString(), err))
		return
	}
	data.Value = jsonValue(entry.Value)
	data.DataType = types.StringValue(entry.DataType)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

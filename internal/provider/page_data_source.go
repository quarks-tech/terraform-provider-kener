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
	_ datasource.DataSource              = (*pageDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*pageDataSource)(nil)
)

// NewPageDataSource is the data source constructor registered with the provider.
func NewPageDataSource() datasource.DataSource {
	return &pageDataSource{}
}

type pageDataSource struct {
	client *client.Client
}

type pageDataSourceModel struct {
	ID            types.String         `tfsdk:"id"`
	PagePath      types.String         `tfsdk:"page_path"`
	PageTitle     types.String         `tfsdk:"page_title"`
	PageHeader    types.String         `tfsdk:"page_header"`
	PageSubheader types.String         `tfsdk:"page_subheader"`
	PageLogo      types.String         `tfsdk:"page_logo"`
	Monitors      types.List           `tfsdk:"monitors"`
	PageSettings  jsontypes.Normalized `tfsdk:"page_settings"`
}

func (d *pageDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_page"
}

func (d *pageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Kener status page by its `page_path` (use `~home` for the built-in home page).",
		Attributes: map[string]schema.Attribute{
			"page_path": schema.StringAttribute{
				MarkdownDescription: "The page slug to look up (`~home` for the home page).",
				Required:            true,
			},
			"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Internal numeric identifier."},
			"page_title":  schema.StringAttribute{Computed: true, MarkdownDescription: "Page title."},
			"page_header": schema.StringAttribute{Computed: true, MarkdownDescription: "Page header."},
			"page_subheader": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Page subheader.",
			},
			"page_logo": schema.StringAttribute{Computed: true, MarkdownDescription: "Page logo URL."},
			"monitors": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Ordered list of monitor tags shown on the page.",
			},
			"page_settings": schema.StringAttribute{
				Computed:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Page display settings as stored by Kener (JSON).",
			},
		},
	}
}

func (d *pageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *pageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data pageDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p, err := d.client.GetPage(ctx, data.PagePath.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading page", fmt.Sprintf("Could not read page %q: %s", data.PagePath.ValueString(), err))
		return
	}

	data.ID = types.StringValue(p.ID.String())
	data.PagePath = types.StringValue(p.PagePath)
	data.PageTitle = types.StringValue(p.PageTitle)
	data.PageHeader = types.StringValue(p.PageHeader)
	data.PageSubheader = strValue(p.PageSubheader)
	data.PageLogo = strValue(p.PageLogo)
	var mons []string
	if p.Monitors != nil {
		mons = []string(*p.Monitors)
	}
	list, diags := stringList(ctx, mons)
	resp.Diagnostics.Append(diags...)
	data.Monitors = list
	data.PageSettings = jsonValue(p.PageSettings)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

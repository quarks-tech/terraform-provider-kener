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
	_ datasource.DataSource              = (*monitorDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*monitorDataSource)(nil)
)

// NewMonitorDataSource is the data source constructor registered with the provider.
func NewMonitorDataSource() datasource.DataSource {
	return &monitorDataSource{}
}

type monitorDataSource struct {
	client *client.Client
}

// monitorDataSourceModel mirrors the monitor attributes for read-only lookup by
// tag. Unlike the resource, the JSON blobs reflect the server's stored value.
type monitorDataSourceModel struct {
	ID                        types.String         `tfsdk:"id"`
	Tag                       types.String         `tfsdk:"tag"`
	Name                      types.String         `tfsdk:"name"`
	Description               types.String         `tfsdk:"description"`
	Image                     types.String         `tfsdk:"image"`
	Cron                      types.String         `tfsdk:"cron"`
	CategoryName              types.String         `tfsdk:"category_name"`
	ExternalURL               types.String         `tfsdk:"external_url"`
	DefaultStatus             types.String         `tfsdk:"default_status"`
	Status                    types.String         `tfsdk:"status"`
	MonitorType               types.String         `tfsdk:"monitor_type"`
	TypeData                  jsontypes.Normalized `tfsdk:"type_data"`
	MonitorSettingsJSON       jsontypes.Normalized `tfsdk:"monitor_settings_json"`
	IsHidden                  types.Bool           `tfsdk:"is_hidden"`
	IncludeDegradedInDowntime types.Bool           `tfsdk:"include_degraded_in_downtime"`
	ConfirmationThreshold     types.Int64          `tfsdk:"confirmation_threshold"`
}

func (d *monitorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (d *monitorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Kener monitor by its `tag`.",
		Attributes: map[string]schema.Attribute{
			"tag": schema.StringAttribute{
				MarkdownDescription: "The immutable slug that identifies the monitor.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Internal numeric identifier.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable monitor name.",
				Computed:            true,
			},
			"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Description shown on the status page."},
			"image":       schema.StringAttribute{Computed: true, MarkdownDescription: "Logo/icon URL."},
			"cron":        schema.StringAttribute{Computed: true, MarkdownDescription: "Cron schedule expression."},
			"category_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Category used to group monitors.",
			},
			"external_url":   schema.StringAttribute{Computed: true, MarkdownDescription: "External link."},
			"default_status": schema.StringAttribute{Computed: true, MarkdownDescription: "Status shown when there is no realtime data."},
			"status":         schema.StringAttribute{Computed: true, MarkdownDescription: "Whether the monitor is enabled (`ACTIVE`/`INACTIVE`)."},
			"monitor_type":   schema.StringAttribute{Computed: true, MarkdownDescription: "Kind of check."},
			"type_data": schema.StringAttribute{
				Computed:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Type-specific configuration as stored by Kener (JSON).",
			},
			"monitor_settings_json": schema.StringAttribute{
				Computed:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Additional monitor settings as stored by Kener (JSON).",
			},
			"is_hidden":                    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the monitor is hidden."},
			"include_degraded_in_downtime": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether DEGRADED counts against uptime."},
			"confirmation_threshold":       schema.Int64Attribute{Computed: true, MarkdownDescription: "Consecutive checks before a status change is recorded."},
		},
	}
}

func (d *monitorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *monitorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data monitorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	m, err := d.client.GetMonitor(ctx, data.Tag.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading monitor", fmt.Sprintf("Could not read monitor %q: %s", data.Tag.ValueString(), err))
		return
	}

	data.ID = types.StringValue(m.ID.String())
	data.Tag = types.StringValue(m.Tag)
	data.Name = types.StringValue(m.Name)
	data.Description = strValue(m.Description)
	data.Image = strValue(m.Image)
	data.Cron = strValue(m.Cron)
	data.CategoryName = strValue(m.CategoryName)
	data.ExternalURL = strValue(m.ExternalURL)
	data.DefaultStatus = strValue(m.DefaultStatus)
	data.Status = strValue(m.Status)
	data.MonitorType = strValue(m.MonitorType)
	data.TypeData = jsonValue(m.TypeData)
	data.MonitorSettingsJSON = jsonValue(m.MonitorSettingsJSON)
	data.IsHidden = yesNoToBool(m.IsHidden)
	data.IncludeDegradedInDowntime = yesNoToBool(m.IncludeDegradedInDowntime)
	data.ConfirmationThreshold = int64Value(m.ConfirmationThreshold)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

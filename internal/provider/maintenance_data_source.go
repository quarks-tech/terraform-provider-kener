package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

var (
	_ datasource.DataSource              = (*maintenanceDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*maintenanceDataSource)(nil)
)

// NewMaintenanceDataSource is the data source constructor registered with the provider.
func NewMaintenanceDataSource() datasource.DataSource {
	return &maintenanceDataSource{}
}

type maintenanceDataSource struct {
	client *client.Client
}

func (d *maintenanceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_maintenance"
}

func (d *maintenanceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Kener maintenance window by its numeric id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Numeric identifier of the maintenance.",
				Required:            true,
			},
			"title":            schema.StringAttribute{Computed: true, MarkdownDescription: "Maintenance title."},
			"description":      schema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
			"start_date_time":  schema.Int64Attribute{Computed: true, MarkdownDescription: "Start time (Unix seconds)."},
			"rrule":            schema.StringAttribute{Computed: true, MarkdownDescription: "Recurrence rule (RRULE)."},
			"duration_seconds": schema.Int64Attribute{Computed: true, MarkdownDescription: "Duration of each occurrence (seconds)."},
			"status":           schema.StringAttribute{Computed: true, MarkdownDescription: "Whether the maintenance is enabled."},
			"url":              schema.StringAttribute{Computed: true, MarkdownDescription: "Absolute public URL."},
			"monitors": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Monitors affected by this maintenance.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"monitor_tag": schema.StringAttribute{Computed: true, MarkdownDescription: "Affected monitor tag."},
						"impact":      schema.StringAttribute{Computed: true, MarkdownDescription: "Impact level."},
					},
				},
			},
		},
	}
}

func (d *maintenanceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *maintenanceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data maintenanceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mm, err := d.client.GetMaintenance(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading maintenance", fmt.Sprintf("Could not read maintenance %q: %s", data.ID.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(applyMaintenance(ctx, mm, &data)...)
	data.StartDateTime = int64Value(mm.StartDateTime)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

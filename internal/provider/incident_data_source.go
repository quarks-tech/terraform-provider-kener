package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

// incidentCommentObjectType is the element type of the incident data source's
// comments list.
var incidentCommentObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":        types.StringType,
	"comment":   types.StringType,
	"state":     types.StringType,
	"timestamp": types.Int64Type,
}}

var (
	_ datasource.DataSource              = (*incidentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*incidentDataSource)(nil)
)

// NewIncidentDataSource is the data source constructor registered with the provider.
func NewIncidentDataSource() datasource.DataSource {
	return &incidentDataSource{}
}

type incidentDataSource struct {
	client *client.Client
}

type incidentDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Title          types.String `tfsdk:"title"`
	StartDateTime  types.Int64  `tfsdk:"start_date_time"`
	EndDateTime    types.Int64  `tfsdk:"end_date_time"`
	State          types.String `tfsdk:"state"`
	IncidentType   types.String `tfsdk:"incident_type"`
	IncidentSource types.String `tfsdk:"incident_source"`
	URL            types.String `tfsdk:"url"`
	Monitors       types.List   `tfsdk:"monitors"`
	Comments       types.List   `tfsdk:"comments"`
}

type incidentCommentValueModel struct {
	ID        types.String `tfsdk:"id"`
	Comment   types.String `tfsdk:"comment"`
	State     types.String `tfsdk:"state"`
	Timestamp types.Int64  `tfsdk:"timestamp"`
}

func (d *incidentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_incident"
}

func (d *incidentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single Kener incident by its numeric id, including its status-update comments.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Numeric identifier of the incident.",
				Required:            true,
			},
			"title":           schema.StringAttribute{Computed: true, MarkdownDescription: "Incident title."},
			"start_date_time": schema.Int64Attribute{Computed: true, MarkdownDescription: "Start time (Unix seconds)."},
			"end_date_time":   schema.Int64Attribute{Computed: true, MarkdownDescription: "End time (Unix seconds)."},
			"state":           schema.StringAttribute{Computed: true, MarkdownDescription: "Current incident state."},
			"incident_type":   schema.StringAttribute{Computed: true, MarkdownDescription: "Incident type."},
			"incident_source": schema.StringAttribute{Computed: true, MarkdownDescription: "Incident source."},
			"url":             schema.StringAttribute{Computed: true, MarkdownDescription: "Absolute public URL."},
			"monitors": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Monitors affected by this incident.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"monitor_tag": schema.StringAttribute{Computed: true, MarkdownDescription: "Affected monitor tag."},
						"impact":      schema.StringAttribute{Computed: true, MarkdownDescription: "Impact level."},
					},
				},
			},
			"comments": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Status-update comments on this incident.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":        schema.StringAttribute{Computed: true, MarkdownDescription: "Comment id."},
						"comment":   schema.StringAttribute{Computed: true, MarkdownDescription: "Comment body."},
						"state":     schema.StringAttribute{Computed: true, MarkdownDescription: "Incident state at this update."},
						"timestamp": schema.Int64Attribute{Computed: true, MarkdownDescription: "Comment time (Unix seconds)."},
					},
				},
			},
		},
	}
}

func (d *incidentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *incidentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data incidentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.ID.ValueString()
	in, err := d.client.GetIncident(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading incident", fmt.Sprintf("Could not read incident %q: %s", id, err))
		return
	}

	data.ID = types.StringValue(in.ID.String())
	data.Title = types.StringValue(in.Title)
	data.StartDateTime = int64Value(in.StartDateTime)
	data.EndDateTime = int64Value(in.EndDateTime)
	data.State = types.StringValue(in.State)
	data.IncidentType = types.StringValue(in.IncidentType)
	data.IncidentSource = types.StringValue(in.IncidentSource)
	data.URL = types.StringValue(in.URL)

	monitors, diags := monitorsToModel(ctx, in.Monitors)
	resp.Diagnostics.Append(diags...)
	data.Monitors = monitors

	comments, err := d.client.ListIncidentComments(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading incident comments", fmt.Sprintf("Could not read comments for incident %q: %s", id, err))
		return
	}
	values := make([]incidentCommentValueModel, 0, len(comments))
	for _, c := range comments {
		values = append(values, incidentCommentValueModel{
			ID:        types.StringValue(c.ID.String()),
			Comment:   types.StringValue(c.Comment),
			State:     types.StringValue(c.State),
			Timestamp: int64Value(c.Timestamp),
		})
	}
	list, diags := types.ListValueFrom(ctx, incidentCommentObjectType, values)
	resp.Diagnostics.Append(diags...)
	data.Comments = list

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

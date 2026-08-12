package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

var impactLevels = []string{"UP", "DOWN", "DEGRADED", "MAINTENANCE"}

// incidentMonitorObjectType is the element type of the monitors list.
var incidentMonitorObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"monitor_tag": types.StringType,
	"impact":      types.StringType,
}}

var (
	_ resource.Resource                = (*incidentResource)(nil)
	_ resource.ResourceWithConfigure   = (*incidentResource)(nil)
	_ resource.ResourceWithImportState = (*incidentResource)(nil)
)

// NewIncidentResource is the resource constructor registered with the provider.
func NewIncidentResource() resource.Resource {
	return &incidentResource{}
}

type incidentResource struct {
	client *client.Client
}

type incidentResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Title          types.String `tfsdk:"title"`
	StartDateTime  types.Int64  `tfsdk:"start_date_time"`
	EndDateTime    types.Int64  `tfsdk:"end_date_time"`
	Monitors       types.List   `tfsdk:"monitors"`
	State          types.String `tfsdk:"state"`
	IncidentType   types.String `tfsdk:"incident_type"`
	IncidentSource types.String `tfsdk:"incident_source"`
	URL            types.String `tfsdk:"url"`
}

type incidentMonitorModel struct {
	MonitorTag types.String `tfsdk:"monitor_tag"`
	Impact     types.String `tfsdk:"impact"`
}

func (r *incidentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_incident"
}

func (r *incidentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedStr := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: desc,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Kener incident. Incidents are identified by a numeric id. Attach one or more monitors with an impact level, and record progress with `kener_incident_comment` resources.",
		Attributes: map[string]schema.Attribute{
			"id":              computedStr("Numeric identifier assigned by Kener."),
			"state":           computedStr("Current incident state (derived from its latest comment)."),
			"incident_type":   computedStr("Incident type as classified by Kener."),
			"incident_source": computedStr("Source of the incident (e.g. `API`)."),
			"url":             computedStr("Absolute public URL of the incident."),
			"title": schema.StringAttribute{
				MarkdownDescription: "Incident title.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"start_date_time": schema.Int64Attribute{
				MarkdownDescription: "Incident start time as a Unix timestamp (seconds).",
				Required:            true,
			},
			"end_date_time": schema.Int64Attribute{
				MarkdownDescription: "Incident end time as a Unix timestamp (seconds). Omit while the incident is ongoing.",
				Optional:            true,
			},
			"monitors": schema.ListNestedAttribute{
				MarkdownDescription: "Monitors affected by this incident, with their impact.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"monitor_tag": schema.StringAttribute{
							MarkdownDescription: "Tag of the affected monitor.",
							Required:            true,
						},
						"impact": schema.StringAttribute{
							MarkdownDescription: "Impact on the monitor. One of `UP`, `DOWN`, `DEGRADED`, `MAINTENANCE`. Defaults to `DOWN`.",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("DOWN"),
							Validators:          []validator.String{stringvalidator.OneOf(impactLevels...)},
						},
					},
				},
			},
		},
	}
}

func (r *incidentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	r.client = c
}

// monitorsToClient converts the model's monitors list into the API shape. A nil
// return means "omit" (leave untouched); a non-nil empty slice clears them.
func monitorsToClient(ctx context.Context, l types.List) (*[]client.MonitorImpact, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var models []incidentMonitorModel
	diags := l.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]client.MonitorImpact, 0, len(models))
	for _, m := range models {
		out = append(out, client.MonitorImpact{
			MonitorTag: m.MonitorTag.ValueString(),
			Impact:     m.Impact.ValueString(),
		})
	}
	return &out, diags
}

// monitorsToModel converts the API monitors into a Terraform list value.
func monitorsToModel(ctx context.Context, m *[]client.MonitorImpact) (types.List, diag.Diagnostics) {
	if m == nil {
		return types.ListValueMust(incidentMonitorObjectType, nil), nil
	}
	models := make([]incidentMonitorModel, 0, len(*m))
	for _, mi := range *m {
		models = append(models, incidentMonitorModel{
			MonitorTag: types.StringValue(mi.MonitorTag),
			Impact:     types.StringValue(mi.Impact),
		})
	}
	return types.ListValueFrom(ctx, incidentMonitorObjectType, models)
}

func modelToIncident(ctx context.Context, m *incidentResourceModel) (*client.Incident, diag.Diagnostics) {
	monitors, diags := monitorsToClient(ctx, m.Monitors)
	in := &client.Incident{
		Title:         m.Title.ValueString(),
		StartDateTime: int64Ptr(m.StartDateTime),
		EndDateTime:   int64Ptr(m.EndDateTime),
		Monitors:      monitors,
	}
	return in, diags
}

// applyIncident copies the server's view of an incident onto the model. It does
// NOT touch start_date_time / end_date_time: Kener aligns those to the minute,
// so overwriting them with the server's rounded value would cause an
// inconsistent-result error and perpetual diffs. The caller keeps the
// configured timestamps (see setIncidentTimestamps for the data source).
func applyIncident(ctx context.Context, in *client.Incident, m *incidentResourceModel) diag.Diagnostics {
	m.ID = types.StringValue(in.ID.String())
	m.Title = types.StringValue(in.Title)
	m.State = types.StringValue(in.State)
	m.IncidentType = types.StringValue(in.IncidentType)
	m.IncidentSource = types.StringValue(in.IncidentSource)
	m.URL = types.StringValue(in.URL)
	list, diags := monitorsToModel(ctx, in.Monitors)
	m.Monitors = list
	return diags
}

// setIncidentTimestamps writes the server's timestamps onto the model. Used by
// the data source, which has no prior configured value to preserve.
func setIncidentTimestamps(in *client.Incident, m *incidentResourceModel) {
	m.StartDateTime = int64Value(in.StartDateTime)
	m.EndDateTime = int64Value(in.EndDateTime)
}

func (r *incidentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan incidentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, diags := modelToIncident(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateIncident(ctx, payload)
	if err != nil {
		resp.Diagnostics.AddError("Error creating incident", fmt.Sprintf("Could not create incident %q: %s", plan.Title.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(applyIncident(ctx, created, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *incidentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state incidentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetIncident(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading incident", fmt.Sprintf("Could not read incident %q: %s", state.ID.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(applyIncident(ctx, got, &state)...)
	// On import there is no prior timestamp to preserve, so seed it from the
	// server (which stores it minute-aligned). For an already-managed resource
	// we keep the configured value to avoid perpetual diffs from rounding.
	if state.StartDateTime.IsNull() {
		setIncidentTimestamps(got, &state)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *incidentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan incidentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, diags := modelToIncident(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateIncident(ctx, plan.ID.ValueString(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Error updating incident", fmt.Sprintf("Could not update incident %q: %s", plan.ID.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(applyIncident(ctx, updated, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *incidentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state incidentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteIncident(ctx, state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting incident", fmt.Sprintf("Could not delete incident %q: %s", state.ID.ValueString(), err))
	}
}

func (r *incidentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

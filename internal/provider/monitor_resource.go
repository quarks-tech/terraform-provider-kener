package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

// Monitor enums, from src/lib/types/monitor.ts and the v4 handlers.
var (
	monitorTypes    = []string{"API", "PING", "TCP", "DNS", "NONE", "GROUP", "SSL", "SQL", "HEARTBEAT", "GAMEDIG", "GRPC"}
	defaultStatuses = []string{"UP", "DOWN", "DEGRADED", "MAINTENANCE", "NO_DATA"}
	monitorStatuses = []string{"ACTIVE", "INACTIVE"}
	// tagPattern matches Kener's slug validation: a single [a-z0-9], or a string
	// that starts and ends with [a-z0-9] and may contain '-'/'_' in between.
	tagPattern = `^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$`
)

// Ensure the resource satisfies the framework interfaces.
var (
	_ resource.Resource                = (*monitorResource)(nil)
	_ resource.ResourceWithConfigure   = (*monitorResource)(nil)
	_ resource.ResourceWithImportState = (*monitorResource)(nil)
)

// NewMonitorResource is the resource constructor registered with the provider.
func NewMonitorResource() resource.Resource {
	return &monitorResource{}
}

type monitorResource struct {
	client *client.Client
}

// monitorResourceModel maps the resource schema to Go types.
type monitorResourceModel struct {
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

func (r *monitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *monitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Kener monitor (an uptime/health check). Monitors are identified by their immutable `tag`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Internal numeric identifier assigned by Kener.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tag": schema.StringAttribute{
				MarkdownDescription: "Immutable, URL-friendly slug that uniquely identifies the monitor. Changing it forces recreation. Must match `" + tagPattern + "`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(tagPattern), "must be a lowercase slug (letters, digits, '-' and '_'; start and end alphanumeric)"),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable monitor name.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description shown on the status page.",
				Optional:            true,
			},
			"image": schema.StringAttribute{
				MarkdownDescription: "Optional logo/icon URL for the monitor.",
				Optional:            true,
			},
			"cron": schema.StringAttribute{
				MarkdownDescription: "Cron expression controlling how often the monitor runs (e.g. `* * * * *`).",
				Optional:            true,
			},
			"category_name": schema.StringAttribute{
				MarkdownDescription: "Optional category used to group monitors on the status page.",
				Optional:            true,
			},
			"external_url": schema.StringAttribute{
				MarkdownDescription: "Optional external link associated with the monitor.",
				Optional:            true,
			},
			"default_status": schema.StringAttribute{
				MarkdownDescription: "Status shown when there is no realtime data. One of `UP`, `DOWN`, `DEGRADED`, `MAINTENANCE`, `NO_DATA`. Defaults to `UP`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.OneOf(defaultStatuses...)},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Whether the monitor is enabled. One of `ACTIVE`, `INACTIVE`. Defaults to `ACTIVE`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.OneOf(monitorStatuses...)},
			},
			"monitor_type": schema.StringAttribute{
				MarkdownDescription: "Kind of check. One of `API`, `PING`, `TCP`, `DNS`, `NONE`, `GROUP`, `SSL`, `SQL`, `HEARTBEAT`, `GAMEDIG`, `GRPC`. Defaults to `API`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.OneOf(monitorTypes...)},
			},
			"type_data": schema.StringAttribute{
				MarkdownDescription: "Type-specific configuration as a JSON object, e.g. `jsonencode({ url = \"https://example.com\" })`. The exact fields depend on `monitor_type` (see the Kener monitor docs). Kener merges server-side defaults into this value; the provider stores the configured value verbatim.",
				Optional:            true,
				CustomType:          jsontypes.NormalizedType{},
			},
			"monitor_settings_json": schema.StringAttribute{
				MarkdownDescription: "Free-form additional monitor settings as a JSON object.",
				Optional:            true,
				CustomType:          jsontypes.NormalizedType{},
			},
			"is_hidden": schema.BoolAttribute{
				MarkdownDescription: "Whether the monitor is hidden from the status page. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"include_degraded_in_downtime": schema.BoolAttribute{
				MarkdownDescription: "Whether DEGRADED status counts against uptime. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"confirmation_threshold": schema.Int64Attribute{
				MarkdownDescription: "Number of consecutive checks (1–60) required before a status change is recorded. Defaults to `1`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Validators:          []validator.Int64{int64validator.Between(1, 60)},
			},
		},
	}
}

func (r *monitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

// modelToMonitor builds an API monitor payload from the resource model.
func modelToMonitor(m *monitorResourceModel) *client.Monitor {
	return &client.Monitor{
		Tag:                       m.Tag.ValueString(),
		Name:                      m.Name.ValueString(),
		Description:               strPtr(m.Description),
		Image:                     strPtr(m.Image),
		Cron:                      strPtr(m.Cron),
		CategoryName:              strPtr(m.CategoryName),
		ExternalURL:               strPtr(m.ExternalURL),
		DefaultStatus:             strPtr(m.DefaultStatus),
		Status:                    strPtr(m.Status),
		MonitorType:               strPtr(m.MonitorType),
		TypeData:                  jsonRaw(m.TypeData),
		MonitorSettingsJSON:       jsonRaw(m.MonitorSettingsJSON),
		IsHidden:                  boolToYesNo(m.IsHidden),
		IncludeDegradedInDowntime: boolToYesNo(m.IncludeDegradedInDowntime),
		ConfirmationThreshold:     int64Ptr(m.ConfirmationThreshold),
	}
}

// applyMonitor copies the server's view of a monitor onto the model. The two
// JSON blob attributes (type_data, monitor_settings_json) are deliberately left
// untouched: Kener deep-merges server-side defaults into them, so overwriting
// from the response would cause perpetual diffs. Callers keep the configured
// value instead.
func applyMonitor(m *client.Monitor, model *monitorResourceModel) {
	model.ID = types.StringValue(m.ID.String())
	model.Tag = types.StringValue(m.Tag)
	model.Name = types.StringValue(m.Name)
	model.Description = strValue(m.Description)
	model.Image = strValue(m.Image)
	model.Cron = strValue(m.Cron)
	model.CategoryName = strValue(m.CategoryName)
	model.ExternalURL = strValue(m.ExternalURL)
	model.DefaultStatus = strValue(m.DefaultStatus)
	model.Status = strValue(m.Status)
	model.MonitorType = strValue(m.MonitorType)
	model.IsHidden = yesNoToBool(m.IsHidden)
	model.IncludeDegradedInDowntime = yesNoToBool(m.IncludeDegradedInDowntime)
	model.ConfirmationThreshold = int64Value(m.ConfirmationThreshold)
}

func (r *monitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan monitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateMonitor(ctx, modelToMonitor(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating monitor", fmt.Sprintf("Could not create monitor %q: %s", plan.Tag.ValueString(), err))
		return
	}

	// Keep the configured JSON blobs; refresh everything else from the server.
	applyMonitor(created, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *monitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state monitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.client.GetMonitor(ctx, state.Tag.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading monitor", fmt.Sprintf("Could not read monitor %q: %s", state.Tag.ValueString(), err))
		return
	}

	applyMonitor(got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *monitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan monitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateMonitor(ctx, plan.Tag.ValueString(), modelToMonitor(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating monitor", fmt.Sprintf("Could not update monitor %q: %s", plan.Tag.ValueString(), err))
		return
	}

	applyMonitor(updated, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *monitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state monitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteMonitor(ctx, state.Tag.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting monitor", fmt.Sprintf("Could not delete monitor %q: %s", state.Tag.ValueString(), err))
	}
}

func (r *monitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by tag; Read populates the remaining attributes.
	resource.ImportStatePassthroughID(ctx, path.Root("tag"), req, resp)
}

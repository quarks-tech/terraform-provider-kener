package provider

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

// secondsPerDay is used to turn the relative window (window_days) into an absolute
// timestamp range at create time.
const secondsPerDay = 24 * 60 * 60

// seedStatuses is the set of statuses the Kener data-write endpoint accepts. It is
// deliberately narrower than monitorResource's defaultStatuses (no MAINTENANCE /
// NO_DATA), because those are rejected by /monitors/{tag}/data.
var seedStatuses = []string{"UP", "DOWN", "DEGRADED"}

// Ensure the resource satisfies the framework interfaces. Note there is no
// ResourceWithImportState: a seed is a write-only side effect with no faithful
// read-back, so importing it is meaningless (see the Read doc below).
var (
	_ resource.Resource              = (*monitorSeedResource)(nil)
	_ resource.ResourceWithConfigure = (*monitorSeedResource)(nil)
)

// NewMonitorSeedResource is the resource constructor registered with the provider.
func NewMonitorSeedResource() resource.Resource {
	return &monitorSeedResource{}
}

type monitorSeedResource struct {
	client *client.Client
}

// monitorSeedResourceModel maps the resource schema to Go types.
type monitorSeedResourceModel struct {
	ID         types.String `tfsdk:"id"`
	MonitorTag types.String `tfsdk:"monitor_tag"`
	WindowDays types.Int64  `tfsdk:"window_days"`
	Status     types.String `tfsdk:"status"`
	Latency    types.Int64  `tfsdk:"latency"`
	Deviation  types.Int64  `tfsdk:"deviation"`
	Triggers   types.Map    `tfsdk:"triggers"`
	StartTS    types.Int64  `tfsdk:"start_ts"`
	EndTS      types.Int64  `tfsdk:"end_ts"`
}

func (r *monitorSeedResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor_seed"
}

func (r *monitorSeedResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Seeds a Kener monitor's status history over a relative window ending now, so a " +
			"brand-new monitor renders a populated (e.g. green) bar chart instead of `NO_DATA` for the period " +
			"before it existed.\n\n" +
			"This is a one-shot, fire-and-forget side effect: `Create` writes the history once, `Read` never " +
			"reads it back (the live monitor rewrites history every minute, so refreshing would cause perpetual " +
			"drift), and every input forces replacement. Re-applying an unchanged configuration is a no-op; " +
			"changing any attribute (or a `triggers` value) re-seeds and **overwrites** any data currently in the " +
			"new window. The resource is not importable.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic identifier, `\"<monitor_tag>:<start_ts>\"`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"monitor_tag": schema.StringAttribute{
				MarkdownDescription: "Tag of the monitor whose history is seeded. Changing it forces recreation.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(tagPattern), "must be a lowercase slug (letters, digits, '-' and '_'; start and end alphanumeric)"),
				},
			},
			"window_days": schema.Int64Attribute{
				MarkdownDescription: "Number of days of history to seed, counting back from the moment of creation. " +
					"Resolved to absolute `start_ts`/`end_ts` once at create time. Defaults to `90`. Changing it forces recreation.",
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(90),
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Validators:    []validator.Int64{int64validator.AtLeast(1)},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Status written across the whole window. One of `UP`, `DOWN`, `DEGRADED` " +
					"(narrower than a monitor's `default_status`). Defaults to `UP`. Changing it forces recreation.",
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("UP"),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.OneOf(seedStatuses...)},
			},
			"latency": schema.Int64Attribute{
				MarkdownDescription: "Latency in milliseconds recorded for each seeded point. Defaults to `0`. Changing it forces recreation.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Validators:          []validator.Int64{int64validator.AtLeast(0)},
			},
			"deviation": schema.Int64Attribute{
				MarkdownDescription: "Per-minute latency jitter: each point's latency is `latency + Uniform[-deviation, deviation]`, " +
					"clamped to `>= 0`. `0` (the default) means a constant `latency`. Changing it forces recreation.",
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
			},
			"triggers": schema.MapAttribute{
				MarkdownDescription: "Arbitrary map of values that, when changed, forces a re-seed (like `null_resource.triggers`). " +
					"Use it to deliberately re-write history without changing any other attribute.",
				Optional:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"start_ts": schema.Int64Attribute{
				MarkdownDescription: "Absolute start of the seeded window (UTC seconds), resolved at create time.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"end_ts": schema.Int64Attribute{
				MarkdownDescription: "Absolute end of the seeded window (UTC seconds), resolved at create time.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *monitorSeedResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *monitorSeedResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan monitorSeedResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve the relative window to an absolute range ONCE, here at create time.
	// end = now (minute-truncated), start = end - window_days. These absolute values
	// are stored in computed state and pinned with UseStateForUnknown, so `now` is
	// never re-derived on later plans and the window never drifts.
	end := time.Now().UTC().Truncate(time.Minute).Unix()
	start := end - plan.WindowDays.ValueInt64()*secondsPerDay

	tag := plan.MonitorTag.ValueString()
	body := &client.MonitorDataRange{
		StartTS:   start,
		EndTS:     end,
		Status:    plan.Status.ValueString(),
		Latency:   plan.Latency.ValueInt64(),
		Deviation: int64Ptr(plan.Deviation),
	}

	tflog.Debug(ctx, "seeding monitor history", map[string]any{
		"tag": tag, "start_ts": start, "end_ts": end, "status": body.Status,
	})
	updated, err := r.client.SeedMonitorData(ctx, tag, body)
	if err != nil {
		resp.Diagnostics.AddError("Error seeding monitor history", fmt.Sprintf("Could not seed history for monitor %q: %s", tag, err))
		return
	}
	tflog.Debug(ctx, "seeded monitor history", map[string]any{"tag": tag, "updated_count": updated})

	plan.StartTS = types.Int64Value(start)
	plan.EndTS = types.Int64Value(end)
	plan.ID = types.StringValue(fmt.Sprintf("%s:%d", tag, start))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read is intentionally a no-op that copies prior state through unchanged. The
// seeded history is rewritten every minute by the live monitor, so reading it back
// and comparing to config would report perpetual drift. Not reading it makes the
// resource drift-proof by construction.
func (r *monitorSeedResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.Append(resp.State.Set(ctx, req.State.Raw)...)
}

// Update is never exercised in practice: every configurable attribute is
// RequiresReplace, so any change destroys and recreates (re-seeds) instead. It is
// implemented as a pass-through only to satisfy the resource interface.
func (r *monitorSeedResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan monitorSeedResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: seeded history cannot be faithfully "un-seeded", and deleting
// the monitor cascades its data anyway. The framework simply drops the resource
// from state.
func (r *monitorSeedResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

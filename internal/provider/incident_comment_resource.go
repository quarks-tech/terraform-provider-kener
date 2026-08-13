package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

var commentStates = []string{"INVESTIGATING", "IDENTIFIED", "MONITORING", "RESOLVED"}

var (
	_ resource.Resource                = (*incidentCommentResource)(nil)
	_ resource.ResourceWithConfigure   = (*incidentCommentResource)(nil)
	_ resource.ResourceWithImportState = (*incidentCommentResource)(nil)
)

// NewIncidentCommentResource is the resource constructor registered with the provider.
func NewIncidentCommentResource() resource.Resource {
	return &incidentCommentResource{}
}

type incidentCommentResource struct {
	client *client.Client
}

type incidentCommentResourceModel struct {
	ID         types.String `tfsdk:"id"`
	IncidentID types.String `tfsdk:"incident_id"`
	Comment    types.String `tfsdk:"comment"`
	State      types.String `tfsdk:"state"`
	Timestamp  types.Int64  `tfsdk:"timestamp"`
}

func (r *incidentCommentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_incident_comment"
}

func (r *incidentCommentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a status update (comment) on a Kener incident.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Numeric identifier of the comment.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"incident_id": schema.StringAttribute{
				MarkdownDescription: "Id of the incident this comment belongs to. Changing it forces recreation.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"comment": schema.StringAttribute{
				MarkdownDescription: "Comment body.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Incident state at this update. One of `INVESTIGATING`, `IDENTIFIED`, `MONITORING`, `RESOLVED`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf(commentStates...)},
			},
			"timestamp": schema.Int64Attribute{
				MarkdownDescription: "Comment time as a Unix timestamp (seconds). Defaults to the time of creation.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *incidentCommentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func modelToComment(m *incidentCommentResourceModel) *client.IncidentComment {
	return &client.IncidentComment{
		Comment:   m.Comment.ValueString(),
		State:     m.State.ValueString(),
		Timestamp: int64Ptr(m.Timestamp),
	}
}

func applyComment(cm *client.IncidentComment, m *incidentCommentResourceModel) {
	m.ID = types.StringValue(cm.ID.String())
	if cm.IncidentID.String() != "" {
		m.IncidentID = types.StringValue(cm.IncidentID.String())
	}
	m.Comment = types.StringValue(cm.Comment)
	m.State = types.StringValue(cm.State)
	m.Timestamp = int64Value(cm.Timestamp)
}

func (r *incidentCommentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan incidentCommentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "creating incident comment", map[string]any{"incident_id": plan.IncidentID.ValueString()})
	created, err := r.client.CreateIncidentComment(ctx, plan.IncidentID.ValueString(), modelToComment(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating incident comment", fmt.Sprintf("Could not create comment on incident %q: %s", plan.IncidentID.ValueString(), err))
		return
	}
	applyComment(created, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *incidentCommentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state incidentCommentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "reading incident comment", map[string]any{"incident_id": state.IncidentID.ValueString(), "id": state.ID.ValueString()})
	got, err := r.client.GetIncidentComment(ctx, state.IncidentID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading incident comment", fmt.Sprintf("Could not read comment %q: %s", state.ID.ValueString(), err))
		return
	}
	applyComment(got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *incidentCommentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan incidentCommentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "updating incident comment", map[string]any{"incident_id": plan.IncidentID.ValueString(), "id": plan.ID.ValueString()})
	updated, err := r.client.UpdateIncidentComment(ctx, plan.IncidentID.ValueString(), plan.ID.ValueString(), modelToComment(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating incident comment", fmt.Sprintf("Could not update comment %q: %s", plan.ID.ValueString(), err))
		return
	}
	applyComment(updated, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *incidentCommentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state incidentCommentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "deleting incident comment", map[string]any{"incident_id": state.IncidentID.ValueString(), "id": state.ID.ValueString()})
	if err := r.client.DeleteIncidentComment(ctx, state.IncidentID.ValueString(), state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting incident comment", fmt.Sprintf("Could not delete comment %q: %s", state.ID.ValueString(), err))
	}
}

// ImportState parses a composite id of the form "incident_id:comment_id".
func (r *incidentCommentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"incident_id:comment_id\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("incident_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

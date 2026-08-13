package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

// pagePathPattern mirrors Kener's page path sanitisation: a lowercase slug.
var pagePathPattern = `^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$`

var (
	_ resource.Resource                = (*pageResource)(nil)
	_ resource.ResourceWithConfigure   = (*pageResource)(nil)
	_ resource.ResourceWithImportState = (*pageResource)(nil)
)

// NewPageResource is the resource constructor registered with the provider.
func NewPageResource() resource.Resource {
	return &pageResource{}
}

type pageResource struct {
	client *client.Client
}

type pageResourceModel struct {
	ID            types.String         `tfsdk:"id"`
	PagePath      types.String         `tfsdk:"page_path"`
	PageTitle     types.String         `tfsdk:"page_title"`
	PageHeader    types.String         `tfsdk:"page_header"`
	PageSubheader types.String         `tfsdk:"page_subheader"`
	PageLogo      types.String         `tfsdk:"page_logo"`
	Monitors      types.List           `tfsdk:"monitors"`
	PageSettings  jsontypes.Normalized `tfsdk:"page_settings"`
}

func (r *pageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_page"
}

func (r *pageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Kener status page. Pages are identified by their immutable `page_path` slug. The built-in home page is addressed by the special path `~home` (import it; it cannot be created or destroyed).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Internal numeric identifier assigned by Kener.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"page_path": schema.StringAttribute{
				MarkdownDescription: "Immutable URL slug for the page (e.g. `status`). Changing it forces recreation. Use `~home` to import the built-in home page.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(pagePathPattern), "must be a lowercase slug (letters, digits, '-' and '_'; start and end alphanumeric), or the literal '~home' on import"),
				},
			},
			"page_title": schema.StringAttribute{
				MarkdownDescription: "Page title (shown in the browser tab and header).",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"page_header": schema.StringAttribute{
				MarkdownDescription: "Large heading displayed at the top of the page.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"page_subheader": schema.StringAttribute{
				MarkdownDescription: "Optional subheading displayed under the header.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"page_logo": schema.StringAttribute{
				MarkdownDescription: "Optional logo URL for the page.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"monitors": schema.ListAttribute{
				MarkdownDescription: "Ordered list of monitor tags shown on the page.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"page_settings": schema.StringAttribute{
				MarkdownDescription: "Page display settings as a JSON object (see the Kener docs). Kener merges server-side defaults into this value; the provider stores the configured value verbatim, so it is not recovered on import.",
				Optional:            true,
				CustomType:          jsontypes.NormalizedType{},
			},
		},
	}
}

func (r *pageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func modelToPage(ctx context.Context, m *pageResourceModel) (*client.Page, diag.Diagnostics) {
	monitors, diags := stringSlice(ctx, m.Monitors)
	p := &client.Page{
		PagePath:      m.PagePath.ValueString(),
		PageTitle:     m.PageTitle.ValueString(),
		PageHeader:    m.PageHeader.ValueString(),
		PageSubheader: strPtr(m.PageSubheader),
		PageLogo:      strPtr(m.PageLogo),
		PageSettings:  jsonRaw(m.PageSettings),
	}
	// Only send monitors when the config set them (nil = omit). An explicit
	// empty list is sent as-is to clear all monitors.
	if monitors != nil {
		mt := client.MonitorTags(monitors)
		p.Monitors = &mt
	}
	return p, diags
}

// applyPage copies the server's view of a page onto the model, leaving
// page_settings as configured (Kener deep-merges server-side defaults).
func applyPage(ctx context.Context, p *client.Page, m *pageResourceModel) diag.Diagnostics {
	m.ID = types.StringValue(p.ID.String())
	m.PageTitle = types.StringValue(p.PageTitle)
	m.PageHeader = types.StringValue(p.PageHeader)
	m.PageSubheader = strValue(p.PageSubheader)
	m.PageLogo = strValue(p.PageLogo)
	// Keep the configured page_path (e.g. "~home" addresses an empty stored path).
	if m.PagePath.IsNull() || m.PagePath.IsUnknown() {
		m.PagePath = types.StringValue(p.PagePath)
	}
	// Keep the configured monitor order when known; seeding from the server echo
	// could reorder a known plan value and trigger an inconsistent-result error.
	// Only fall back to the server list when it is absent (import / omitted).
	if !m.Monitors.IsNull() && !m.Monitors.IsUnknown() {
		return nil
	}
	var mons []string
	if p.Monitors != nil {
		mons = []string(*p.Monitors)
	}
	list, diags := stringList(ctx, mons)
	m.Monitors = list
	return diags
}

func (r *pageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := modelToPage(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "creating page", map[string]any{"page_path": plan.PagePath.ValueString()})
	created, err := r.client.CreatePage(ctx, payload)
	if err != nil {
		resp.Diagnostics.AddError("Error creating page", fmt.Sprintf("Could not create page %q: %s", plan.PagePath.ValueString(), err))
		return
	}

	resp.Diagnostics.Append(applyPage(ctx, created, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "reading page", map[string]any{"page_path": state.PagePath.ValueString()})
	got, err := r.client.GetPage(ctx, state.PagePath.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading page", fmt.Sprintf("Could not read page %q: %s", state.PagePath.ValueString(), err))
		return
	}

	resp.Diagnostics.Append(applyPage(ctx, got, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *pageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan pageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := modelToPage(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "updating page", map[string]any{"page_path": plan.PagePath.ValueString()})
	updated, err := r.client.UpdatePage(ctx, plan.PagePath.ValueString(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Error updating page", fmt.Sprintf("Could not update page %q: %s", plan.PagePath.ValueString(), err))
		return
	}

	resp.Diagnostics.Append(applyPage(ctx, updated, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "deleting page", map[string]any{"page_path": state.PagePath.ValueString()})

	// The built-in home page cannot be deleted via the API; removing the resource
	// just drops it from state, leaving the page on the server unchanged.
	if state.PagePath.ValueString() == client.HomePageToken {
		resp.Diagnostics.AddWarning(
			"Home page left unchanged",
			"The Kener home page (~home) cannot be deleted. The resource has been removed from Terraform state, but the page remains on the server.",
		)
		return
	}

	if err := r.client.DeletePage(ctx, state.PagePath.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting page", fmt.Sprintf("Could not delete page %q: %s", state.PagePath.ValueString(), err))
	}
}

func (r *pageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("page_path"), req, resp)
}

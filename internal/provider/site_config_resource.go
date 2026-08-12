package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

var (
	_ resource.Resource                = (*siteConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*siteConfigResource)(nil)
	_ resource.ResourceWithImportState = (*siteConfigResource)(nil)
)

// NewSiteConfigResource is the resource constructor registered with the provider.
func NewSiteConfigResource() resource.Resource {
	return &siteConfigResource{}
}

type siteConfigResource struct {
	client *client.Client
}

type siteConfigResourceModel struct {
	Key      types.String         `tfsdk:"key"`
	Value    jsontypes.Normalized `tfsdk:"value"`
	DataType types.String         `tfsdk:"data_type"`
}

func (r *siteConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_config"
}

func (r *siteConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a single Kener global site-configuration value (e.g. `title`, `siteName`, `theme`, `nav`). These keys are a fixed set that can only be updated, not created or deleted: applying this resource sets the key's value, and destroying it removes the key from Terraform state while leaving the value on the server unchanged.",
		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				MarkdownDescription: "The site-configuration key to manage (e.g. `title`, `siteName`, `siteURL`, `theme`, `logo`, `nav`). See the Kener docs for the full list. Changing it forces replacement.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The value as JSON. Use `jsonencode(...)` — a string value is `jsonencode(\"My Title\")`, an object/array value is `jsonencode({ ... })`. Kener may normalise object values server-side.",
				Required:            true,
				CustomType:          jsontypes.NormalizedType{},
			},
			"data_type": schema.StringAttribute{
				MarkdownDescription: "The value's data type as reported by Kener (`string` or `object`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *siteConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteConfigResource) set(ctx context.Context, model *siteConfigResourceModel) error {
	entry, err := r.client.SetSiteConfig(ctx, model.Key.ValueString(), jsonRaw(model.Value))
	if err != nil {
		return err
	}
	model.DataType = types.StringValue(entry.DataType)
	// Keep the configured value verbatim; some object keys are normalised
	// server-side, which would otherwise cause perpetual diffs.
	return nil
}

func (r *siteConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.set(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error setting site config", fmt.Sprintf("Could not set site config %q: %s", plan.Key.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	entry, err := r.client.GetSiteConfig(ctx, state.Key.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading site config", fmt.Sprintf("Could not read site config %q: %s", state.Key.ValueString(), err))
		return
	}
	state.DataType = types.StringValue(entry.DataType)
	// On import there is no configured value yet, so seed it from the server.
	if state.Value.IsNull() {
		state.Value = jsonValue(entry.Value)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.set(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error updating site config", fmt.Sprintf("Could not update site config %q: %s", plan.Key.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Site-config keys cannot be deleted via the API; removing the resource just
	// drops it from state. The value remains on the server unchanged.
	resp.Diagnostics.AddWarning(
		"Site config value left unchanged",
		"Kener site-configuration keys cannot be deleted. The resource has been removed from Terraform state, but the value remains set on the server.",
	)
}

func (r *siteConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}

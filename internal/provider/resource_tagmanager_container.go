package provider

import (
	"context"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &tagManagerContainerResource{}
	_ resource.ResourceWithConfigure   = &tagManagerContainerResource{}
	_ resource.ResourceWithImportState = &tagManagerContainerResource{}
)

func NewTagManagerContainerResource() resource.Resource {
	return &tagManagerContainerResource{}
}

type tagManagerContainerResource struct {
	client *matomo.Client
}

type tagManagerContainerResourceModel struct {
	ID                                 types.String `tfsdk:"id"`
	SiteID                             types.String `tfsdk:"site_id"`
	Context                            types.String `tfsdk:"context"`
	Name                               types.String `tfsdk:"name"`
	Description                        types.String `tfsdk:"description"`
	IgnoreGtmDataLayer                 types.Bool   `tfsdk:"ignore_gtm_data_layer"`
	IsTagFireLimitAllowedInPreviewMode types.Bool   `tfsdk:"is_tag_fire_limit_allowed_in_preview_mode"`
	ActivelySyncGtmDataLayer           types.Bool   `tfsdk:"actively_sync_gtm_data_layer"`
}

func (r *tagManagerContainerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_container"
}

func (r *tagManagerContainerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/container_id\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"site_id": schema.StringAttribute{
				Required:    true,
				Description: "The owning site's id (matomo_site.x.id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"context": schema.StringAttribute{
				Required:    true,
				Description: "\"web\", \"android\", or \"ios\".",
				Validators: []validator.String{
					stringvalidator.OneOf("web", "android", "ios"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The container's name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The container's description.",
			},
			"ignore_gtm_data_layer": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "If true, this container ignores an existing GTM-style dataLayer on the page instead of reusing it.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_tag_fire_limit_allowed_in_preview_mode": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "If true, a tag's configured fire limit is also enforced while in preview/debug mode.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"actively_sync_gtm_data_layer": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "If true, this container actively keeps an existing GTM-style dataLayer in sync rather than only reading it once.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *tagManagerContainerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	r.client = client
}

func (r *tagManagerContainerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagManagerContainerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, err := strconv.Atoi(plan.SiteID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site_id", err.Error())
		return
	}
	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
	ignoreGtmDataLayer := false
	if !plan.IgnoreGtmDataLayer.IsUnknown() && !plan.IgnoreGtmDataLayer.IsNull() {
		ignoreGtmDataLayer = plan.IgnoreGtmDataLayer.ValueBool()
	}
	isTagFireLimitAllowedInPreviewMode := false
	if !plan.IsTagFireLimitAllowedInPreviewMode.IsUnknown() && !plan.IsTagFireLimitAllowedInPreviewMode.IsNull() {
		isTagFireLimitAllowedInPreviewMode = plan.IsTagFireLimitAllowedInPreviewMode.ValueBool()
	}
	activelySyncGtmDataLayer := false
	if !plan.ActivelySyncGtmDataLayer.IsUnknown() && !plan.ActivelySyncGtmDataLayer.IsNull() {
		activelySyncGtmDataLayer = plan.ActivelySyncGtmDataLayer.ValueBool()
	}

	idContainer, err := r.client.AddContainer(ctx, siteID, plan.Context.ValueString(), plan.Name.ValueString(), description, ignoreGtmDataLayer, isTagFireLimitAllowedInPreviewMode, activelySyncGtmDataLayer)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager container", err.Error())
		return
	}

	plan.ID = types.StringValue(buildContainerID(siteID, idContainer))
	plan.Description = types.StringValue(description)
	plan.IgnoreGtmDataLayer = types.BoolValue(ignoreGtmDataLayer)
	plan.IsTagFireLimitAllowedInPreviewMode = types.BoolValue(isTagFireLimitAllowedInPreviewMode)
	plan.ActivelySyncGtmDataLayer = types.BoolValue(activelySyncGtmDataLayer)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerContainerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagManagerContainerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	ct, err := r.client.GetContainer(ctx, siteID, idContainer)
	if err != nil {
		if isContainerNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager container", err.Error())
		return
	}

	state.SiteID = types.StringValue(strconv.Itoa(siteID))
	state.Context = types.StringValue(ct.Context)
	state.Name = types.StringValue(ct.Name)
	state.Description = types.StringValue(ct.Description)
	state.IgnoreGtmDataLayer = types.BoolValue(bool(ct.IgnoreGtmDataLayer))
	state.IsTagFireLimitAllowedInPreviewMode = types.BoolValue(bool(ct.IsTagFireLimitAllowedInPreviewMode))
	state.ActivelySyncGtmDataLayer = types.BoolValue(bool(ct.ActivelySyncGtmDataLayer))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagManagerContainerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tagManagerContainerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
	ignoreGtmDataLayer := false
	if !plan.IgnoreGtmDataLayer.IsUnknown() && !plan.IgnoreGtmDataLayer.IsNull() {
		ignoreGtmDataLayer = plan.IgnoreGtmDataLayer.ValueBool()
	}
	isTagFireLimitAllowedInPreviewMode := false
	if !plan.IsTagFireLimitAllowedInPreviewMode.IsUnknown() && !plan.IsTagFireLimitAllowedInPreviewMode.IsNull() {
		isTagFireLimitAllowedInPreviewMode = plan.IsTagFireLimitAllowedInPreviewMode.ValueBool()
	}
	activelySyncGtmDataLayer := false
	if !plan.ActivelySyncGtmDataLayer.IsUnknown() && !plan.ActivelySyncGtmDataLayer.IsNull() {
		activelySyncGtmDataLayer = plan.ActivelySyncGtmDataLayer.ValueBool()
	}

	if err := r.client.UpdateContainer(ctx, siteID, idContainer, plan.Name.ValueString(), description, ignoreGtmDataLayer, isTagFireLimitAllowedInPreviewMode, activelySyncGtmDataLayer); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager container", err.Error())
		return
	}

	plan.Description = types.StringValue(description)
	plan.IgnoreGtmDataLayer = types.BoolValue(ignoreGtmDataLayer)
	plan.IsTagFireLimitAllowedInPreviewMode = types.BoolValue(isTagFireLimitAllowedInPreviewMode)
	plan.ActivelySyncGtmDataLayer = types.BoolValue(activelySyncGtmDataLayer)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerContainerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagManagerContainerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	if err := r.client.DeleteContainer(ctx, siteID, idContainer); err != nil && !isContainerNotFoundError(err) {
		resp.Diagnostics.AddError("Error deleting Matomo Tag Manager container", err.Error())
	}
}

// isContainerNotFoundError reports whether err is Matomo's response for an
// unknown/already-deleted container. Real Matomo's message (confirmed
// against a live instance) is `The requested container "<id>" does not
// exist.` - the id is interpolated, so match on the fixed prefix/suffix
// rather than exact equality.
func isContainerNotFoundError(err error) bool {
	apiErr, ok := err.(*matomo.APIError)
	return ok &&
		strings.HasPrefix(apiErr.Message, "The requested container ") &&
		strings.HasSuffix(apiErr.Message, "does not exist.")
}

func (r *tagManagerContainerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

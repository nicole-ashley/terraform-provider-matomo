package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &customDimensionResource{}
	_ resource.ResourceWithConfigure   = &customDimensionResource{}
	_ resource.ResourceWithImportState = &customDimensionResource{}
)

func NewCustomDimensionResource() resource.Resource {
	return &customDimensionResource{}
}

type customDimensionResource struct {
	client *matomo.Client
}

type customDimensionResourceModel struct {
	ID     types.String `tfsdk:"id"`
	SiteID types.String `tfsdk:"site_id"`
	Index  types.Int64  `tfsdk:"index"`
	Scope  types.String `tfsdk:"scope"`
	Name   types.String `tfsdk:"name"`
	Active types.Bool   `tfsdk:"active"`
}

func (r *customDimensionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_dimension"
}

func (r *customDimensionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/index\".",
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
			"index": schema.Int64Attribute{
				Required:    true,
				Description: "The dimension's slot number within its scope. You choose this; Matomo does not support picking a slot on creation, so Create verifies the slot it assigns matches this value.",
			},
			"scope": schema.StringAttribute{
				Required:    true,
				Description: "\"visit\" or \"action\".",
				Validators: []validator.String{
					stringvalidator.OneOf("visit", "action"),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The dimension's display name.",
			},
			"active": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the dimension is active. Matomo has no delete API for custom dimensions; destroying this resource sets active=false rather than removing the slot.",
			},
		},
	}
}

func (r *customDimensionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *customDimensionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customDimensionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, err := strconv.Atoi(plan.SiteID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site_id", err.Error())
		return
	}
	declaredIndex := int(plan.Index.ValueInt64())
	scope := plan.Scope.ValueString()
	name := plan.Name.ValueString()
	active := true
	if !plan.Active.IsUnknown() && !plan.Active.IsNull() {
		active = plan.Active.ValueBool()
	}

	existing, err := r.client.GetConfiguredCustomDimensions(ctx, siteID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo custom dimensions", err.Error())
		return
	}

	var match *matomo.CustomDimension
	for i := range existing {
		if existing[i].Index == declaredIndex && existing[i].Scope == scope {
			match = &existing[i]
			break
		}
	}

	if match != nil {
		if err := r.client.ConfigureExistingCustomDimension(ctx, match.ID, siteID, name, active); err != nil {
			resp.Diagnostics.AddError("Error adopting existing Matomo custom dimension", err.Error())
			return
		}
	} else {
		newID, err := r.client.ConfigureNewCustomDimension(ctx, siteID, name, scope, active)
		if err != nil {
			resp.Diagnostics.AddError("Error creating Matomo custom dimension", err.Error())
			return
		}

		afterCreate, err := r.client.GetConfiguredCustomDimensions(ctx, siteID)
		if err != nil {
			resp.Diagnostics.AddError("Error listing Matomo custom dimensions", err.Error())
			return
		}
		var created *matomo.CustomDimension
		for i := range afterCreate {
			if afterCreate[i].ID == newID {
				created = &afterCreate[i]
				break
			}
		}
		if created == nil {
			resp.Diagnostics.AddError(
				"Custom dimension not found after creation",
				fmt.Sprintf("Matomo reported creating dimension id %d, but it was not found when listing configured custom dimensions for site %d.", newID, siteID),
			)
			return
		}
		if created.Index != declaredIndex {
			resp.Diagnostics.AddError(
				"Custom dimension slot mismatch",
				fmt.Sprintf(
					"Declared index %d for scope %q, but Matomo assigned slot %d instead (the next free slot was not %d — likely because a lower slot was consumed outside Terraform). "+
						"Slot %d has already been created in Matomo and cannot be deleted via its API; either declare index = %d for this resource, or bring slot %d under management with its own matomo_custom_dimension resource.",
					declaredIndex, scope, created.Index, declaredIndex, created.Index, created.Index, created.Index,
				),
			)
			return
		}
	}

	plan.ID = types.StringValue(buildDimensionID(siteID, declaredIndex))
	plan.Active = types.BoolValue(active)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customDimensionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customDimensionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, index, err := parseDimensionID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	dims, err := r.client.GetConfiguredCustomDimensions(ctx, siteID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Matomo custom dimensions", err.Error())
		return
	}

	scope := state.Scope.ValueString()
	var found *matomo.CustomDimension
	for i := range dims {
		if dims[i].Index == index && dims[i].Scope == scope {
			found = &dims[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.SiteID = types.StringValue(strconv.Itoa(siteID))
	state.Index = types.Int64Value(int64(found.Index))
	state.Scope = types.StringValue(found.Scope)
	state.Name = types.StringValue(found.Name)
	state.Active = types.BoolValue(found.Active)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *customDimensionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customDimensionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, index, err := parseDimensionID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}
	scope := plan.Scope.ValueString()

	active := true
	if !plan.Active.IsUnknown() && !plan.Active.IsNull() {
		active = plan.Active.ValueBool()
	}

	dims, err := r.client.GetConfiguredCustomDimensions(ctx, siteID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo custom dimensions", err.Error())
		return
	}
	var found *matomo.CustomDimension
	for i := range dims {
		if dims[i].Index == index && dims[i].Scope == scope {
			found = &dims[i]
			break
		}
	}
	if found == nil {
		resp.Diagnostics.AddError(
			"Custom dimension not found",
			fmt.Sprintf("No custom dimension found at index %d, scope %q for site %d. It may have been deleted or reconfigured outside Terraform.", index, scope, siteID),
		)
		return
	}

	if err := r.client.ConfigureExistingCustomDimension(ctx, found.ID, siteID, plan.Name.ValueString(), active); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo custom dimension", err.Error())
		return
	}

	plan.Active = types.BoolValue(active)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customDimensionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customDimensionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, index, err := parseDimensionID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}
	scope := state.Scope.ValueString()

	dims, err := r.client.GetConfiguredCustomDimensions(ctx, siteID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo custom dimensions", err.Error())
		return
	}
	var found *matomo.CustomDimension
	for i := range dims {
		if dims[i].Index == index && dims[i].Scope == scope {
			found = &dims[i]
			break
		}
	}
	if found == nil {
		resp.Diagnostics.AddError(
			"Custom dimension not found",
			fmt.Sprintf("No custom dimension found at index %d, scope %q for site %d. It may have already been deleted or reconfigured outside Terraform.", index, scope, siteID),
		)
		return
	}

	if err := r.client.ConfigureExistingCustomDimension(ctx, found.ID, siteID, state.Name.ValueString(), false); err != nil {
		resp.Diagnostics.AddError("Error deactivating Matomo custom dimension", err.Error())
	}
}

func (r *customDimensionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

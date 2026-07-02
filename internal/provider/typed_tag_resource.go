// internal/provider/typed_tag_resource.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// typedTagCommon holds the fields every generated tag model declares
// identically (see tools/gen/templates/schema.go.tmpl). Reading/writing
// through this shape (via req.Plan.Get / resp.State.Set, which populate
// only the fields present in both the config's object type and this
// struct's tfsdk tags) lets the shared runtime handle common fields
// without per-type reflection, while each model's own ToParams/FromParams
// handles the type-specific remainder.
type typedTagCommon struct {
	ID              types.String   `tfsdk:"id"`
	ContainerID     types.String   `tfsdk:"container_id"`
	Name            types.String   `tfsdk:"name"`
	Status          types.String   `tfsdk:"status"`
	FireTriggerIDs  []types.String `tfsdk:"fire_trigger_ids"`
	BlockTriggerIDs []types.String `tfsdk:"block_trigger_ids"`
}

var (
	_ resource.Resource                = &typedTagResource{}
	_ resource.ResourceWithConfigure   = &typedTagResource{}
	_ resource.ResourceWithImportState = &typedTagResource{}
)

// typedTagResource is the single CRUD implementation shared by every
// generated matomo_tagmanager_tag_<type> resource. newModel constructs a
// fresh, zero-valued instance of that type's generated model.
type typedTagResource struct {
	client   *matomo.Client
	newModel func() typedModel
}

func newTypedTagResource(newModel func() typedModel) resource.Resource {
	return &typedTagResource{newModel: newModel}
}

func (r *typedTagResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	// Meta().ResourceName is already the full Terraform type name (e.g.
	// "matomo_tagmanager_tag_customhtml"), not a suffix to append to
	// req.ProviderTypeName - unlike the hand-written resources
	// (resource_tagmanager_tag.go etc.), which only ever have one type
	// name and so build it from the provider prefix at registration time.
	resp.TypeName = r.newModel().Meta().ResourceName
}

func (r *typedTagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.newModel().Meta().Schema
}

func (r *typedTagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *typedTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.Plan.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var common typedTagCommon
	resp.Diagnostics.Append(req.Plan.Get(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(common.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	fireIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(common.FireTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid fire_trigger_ids", err.Error())
		return
	}
	blockIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(common.BlockTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid block_trigger_ids", err.Error())
		return
	}

	idTag, err := r.client.AddContainerTag(ctx, siteID, idContainer, versionID, matomo.TagParams{
		Type:            model.Meta().TypeID,
		Name:            common.Name.ValueString(),
		Parameters:      model.ToParams(),
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager tag", err.Error())
		return
	}

	status := "active"
	if !common.Status.IsUnknown() && !common.Status.IsNull() {
		status = common.Status.ValueString()
	}
	if status == "paused" {
		if err := r.client.PauseContainerTag(ctx, siteID, idContainer, versionID, idTag); err != nil {
			resp.Diagnostics.AddError("Error pausing Matomo Tag Manager tag", err.Error())
			return
		}
	}

	common.ID = types.StringValue(buildEntityID(siteID, idContainer, idTag))
	common.Status = types.StringValue(status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var common typedTagCommon
	resp.Diagnostics.Append(req.State.Get(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	tag, err := r.client.GetContainerTag(ctx, siteID, idContainer, versionID, idTag)
	if err != nil {
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Tag does not exist" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager tag", err.Error())
		return
	}

	common.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	common.Name = types.StringValue(tag.Name)
	common.Status = types.StringValue(tag.Status)
	common.FireTriggerIDs = stringModelFromSlice(compositeEntityIDs(siteID, idContainer, intsToStrings(tag.FireTriggerIDs)))
	common.BlockTriggerIDs = stringModelFromSlice(compositeEntityIDs(siteID, idContainer, intsToStrings(tag.BlockTriggerIDs)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := r.newModel()
	model.FromParams(tag.Parameters)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.Plan.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var common typedTagCommon
	resp.Diagnostics.Append(req.Plan.Get(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	fireIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(common.FireTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid fire_trigger_ids", err.Error())
		return
	}
	blockIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(common.BlockTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid block_trigger_ids", err.Error())
		return
	}

	if err := r.client.UpdateContainerTag(ctx, siteID, idContainer, versionID, idTag, matomo.TagParams{
		Type:            model.Meta().TypeID,
		Name:            common.Name.ValueString(),
		Parameters:      model.ToParams(),
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager tag", err.Error())
		return
	}

	status := "active"
	if !common.Status.IsUnknown() && !common.Status.IsNull() {
		status = common.Status.ValueString()
	}
	if status == "paused" {
		err = r.client.PauseContainerTag(ctx, siteID, idContainer, versionID, idTag)
	} else {
		err = r.client.ResumeContainerTag(ctx, siteID, idContainer, versionID, idTag)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager tag status", err.Error())
		return
	}

	common.Status = types.StringValue(status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var common typedTagCommon
	resp.Diagnostics.Append(req.State.Get(ctx, &common)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	if err := r.client.DeleteContainerTag(ctx, siteID, idContainer, versionID, idTag); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo Tag Manager tag", err.Error())
	}
}

func (r *typedTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

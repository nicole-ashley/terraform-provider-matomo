// internal/provider/typed_trigger_resource.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// typedTriggerCommon holds the fields every generated trigger model
// embeds (anonymously) identically (see
// tools/gen/templates/schema.go.tmpl). Triggers have no status or
// fire/block trigger id fields, unlike tags.
type typedTriggerCommon struct {
	ID          types.String `tfsdk:"id"`
	ContainerID types.String `tfsdk:"container_id"`
	Name        types.String `tfsdk:"name"`
}

var (
	_ resource.Resource                = &typedTriggerResource{}
	_ resource.ResourceWithConfigure   = &typedTriggerResource{}
	_ resource.ResourceWithImportState = &typedTriggerResource{}
)

// typedTriggerResource is the single CRUD implementation shared by every
// generated matomo_tagmanager_trigger_<type> resource. newModel constructs
// a fresh, zero-valued instance of that type's generated model.
type typedTriggerResource struct {
	client   *matomo.Client
	newModel func() typedTriggerModel
}

func newTypedTriggerResource(newModel func() typedTriggerModel) resource.Resource {
	return &typedTriggerResource{newModel: newModel}
}

func (r *typedTriggerResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	// Meta().ResourceName is already the full Terraform type name (e.g.
	// "matomo_tagmanager_trigger_pageview"), not a suffix to append to
	// req.ProviderTypeName - unlike the hand-written resources
	// (resource_tagmanager_trigger.go etc.), which only ever have one type
	// name and so build it from the provider prefix at registration time.
	resp.TypeName = r.newModel().Meta().ResourceName
}

func (r *typedTriggerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.newModel().Meta().Schema
}

func (r *typedTriggerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *typedTriggerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.Plan.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	common := model.Common()

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

	idTrigger, err := r.client.AddContainerTrigger(ctx, siteID, idContainer, versionID, matomo.TriggerParams{
		Type:       model.Meta().TypeID,
		Name:       common.Name.ValueString(),
		Parameters: model.ToParams(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager trigger", err.Error())
		return
	}

	common.ID = types.StringValue(buildEntityID(siteID, idContainer, idTrigger))
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedTriggerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.State.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	common := model.Common()

	siteID, idContainer, idTrigger, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	trig, err := r.client.GetContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger)
	if err != nil {
		// "Trigger does not exist" is confirmed (via the _disappears
		// acceptance test against a real Matomo instance) to be the exact
		// error string TagManager.getContainerTrigger returns for an unknown
		// trigger.
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Trigger does not exist" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager trigger", err.Error())
		return
	}

	common.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	common.Name = types.StringValue(trig.Name)

	model.FromParams(trig.Parameters)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedTriggerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.Plan.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	common := model.Common()

	siteID, idContainer, idTrigger, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	if err := r.client.UpdateContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger, matomo.TriggerParams{
		Type:       model.Meta().TypeID,
		Name:       common.Name.ValueString(),
		Parameters: model.ToParams(),
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager trigger", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedTriggerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.State.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	common := model.Common()

	siteID, idContainer, idTrigger, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	if err := r.client.DeleteContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo Tag Manager trigger", err.Error())
	}
}

func (r *typedTriggerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

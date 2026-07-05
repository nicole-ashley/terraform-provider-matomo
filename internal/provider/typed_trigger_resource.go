// internal/provider/typed_trigger_resource.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// typedTriggerCommon holds the fields every generated trigger model
// embeds (anonymously) identically (see
// tools/gen/templates/schema.go.tmpl). Triggers have no status or
// fire/block trigger id fields, unlike tags.
type typedTriggerCommon struct {
	ID          types.String            `tfsdk:"id"`
	ContainerID types.String            `tfsdk:"container_id"`
	Name        types.String            `tfsdk:"name"`
	Description types.String            `tfsdk:"description"`
	Condition   []triggerConditionModel `tfsdk:"condition"`
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

// Schema injects the shared "condition" block into every generated trigger
// type's own schema, rather than each generated_trigger_*.go file declaring
// it independently - this is what makes conditions apply automatically to
// every current and future generated trigger type with zero regeneration.
func (r *typedTriggerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := r.newModel().Meta().Schema
	if s.Blocks == nil {
		s.Blocks = map[string]schema.Block{}
	}
	s.Blocks["condition"] = schema.ListNestedBlock{
		Description: "Conditions that must all match for this trigger to fire.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"comparison": schema.StringAttribute{Required: true},
				"variable":   schema.StringAttribute{Required: true, Description: "A reference to a Matomo built-in variable (e.g. \"PagePath\" - see the matomo_tagmanager_builtin_variable data source) or a user-defined variable macro (e.g. \"{{My Variable}}\")."},
				"value":      schema.StringAttribute{Required: true},
			},
		},
	}
	resp.Schema = s
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
	savedListFields := snapshotListFields(model)

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

	description := ""
	if !common.Description.IsUnknown() && !common.Description.IsNull() {
		description = common.Description.ValueString()
	}

	idTrigger, err := r.client.AddContainerTrigger(ctx, siteID, idContainer, versionID, matomo.TriggerParams{
		Type:        model.Meta().TypeID,
		Name:        common.Name.ValueString(),
		Description: description,
		Parameters:  model.ToParams(),
		Conditions:  conditionsToParams(common.Condition),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager trigger", err.Error())
		return
	}

	// Read the trigger back rather than assembling state by hand: every
	// generated model's Optional parameter is now Optional+Computed (see
	// tools/gen/emit.go's NeedsBoolPlanModifierImport doc comment), so
	// terraform-plugin-framework requires Create to resolve them all the
	// way to known values before Set - state.Set refuses to persist an
	// Unknown value, which is exactly what an unset Optional+Computed
	// field still holds straight out of req.Plan.Get.
	trig, err := r.client.GetContainerTrigger(ctx, siteID, idContainer, versionID, idTrigger)
	if err != nil {
		resp.Diagnostics.AddError("Error reading back created Matomo Tag Manager trigger", err.Error())
		return
	}

	common.ID = types.StringValue(buildEntityID(siteID, idContainer, idTrigger))
	common.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	common.Name = types.StringValue(trig.Name)
	common.Description = types.StringValue(trig.Description)
	common.Condition = conditionsFromAPI(trig.Conditions)

	model.FromParams(trig.Parameters)
	restoreListFields(model, savedListFields)
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
	common.Description = types.StringValue(trig.Description)
	common.Condition = conditionsFromAPI(trig.Conditions)

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
		Type:        model.Meta().TypeID,
		Name:        common.Name.ValueString(),
		Description: common.Description.ValueString(),
		Parameters:  model.ToParams(),
		Conditions:  conditionsToParams(common.Condition),
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

// internal/provider/typed_variable_resource.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

// typedVariableCommon holds the fields every generated variable model
// embeds (anonymously) identically (see
// tools/gen/templates/schema.go.tmpl).
type typedVariableCommon struct {
	ID           types.String `tfsdk:"id"`
	ContainerID  types.String `tfsdk:"container_id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	DefaultValue types.String `tfsdk:"default_value"`
}

var (
	_ resource.Resource                = &typedVariableResource{}
	_ resource.ResourceWithConfigure   = &typedVariableResource{}
	_ resource.ResourceWithImportState = &typedVariableResource{}
)

// typedVariableResource is the single CRUD implementation shared by every
// generated matomo_tagmanager_variable_<type> resource. newModel constructs
// a fresh, zero-valued instance of that type's generated model.
type typedVariableResource struct {
	client   *matomo.Client
	newModel func() typedVariableModel
}

func newTypedVariableResource(newModel func() typedVariableModel) resource.Resource {
	return &typedVariableResource{newModel: newModel}
}

// variableDefaultValueFromAPI decodes Matomo's default_value response
// field (always a string, empty when unset) to null rather than
// StringValue("") - decoding an empty string straight through left an
// unset default_value looking non-null forever, the same "refresh plan
// not empty" bug the generated FromParams methods hit for their own
// Optional fields.
func variableDefaultValueFromAPI(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func (r *typedVariableResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	// Meta().ResourceName is already the full Terraform type name (e.g.
	// "matomo_tagmanager_variable_constant"), not a suffix to append to
	// req.ProviderTypeName - unlike the hand-written resources
	// (resource_tagmanager_variable.go etc.), which only ever have one type
	// name and so build it from the provider prefix at registration time.
	resp.TypeName = r.newModel().Meta().ResourceName
}

func (r *typedVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.newModel().Meta().Schema
}

func (r *typedVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *typedVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
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

	var defaultValue *string
	if !common.DefaultValue.IsUnknown() && !common.DefaultValue.IsNull() {
		v := common.DefaultValue.ValueString()
		defaultValue = &v
	}

	description := ""
	if !common.Description.IsUnknown() && !common.Description.IsNull() {
		description = common.Description.ValueString()
	}

	idVariable, err := r.client.AddContainerVariable(ctx, siteID, idContainer, versionID, matomo.VariableParams{
		Type:         model.Meta().TypeID,
		Name:         common.Name.ValueString(),
		Description:  description,
		Parameters:   model.ToParams(),
		DefaultValue: defaultValue,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager variable", err.Error())
		return
	}

	// Read the variable back rather than assembling state by hand: every
	// generated model's Optional parameter (and default_value) is now
	// Optional+Computed (see tools/gen/emit.go's
	// NeedsBoolPlanModifierImport doc comment), so terraform-plugin-
	// framework requires Create to resolve them all the way to known
	// values before Set - state.Set refuses to persist an Unknown
	// value, which is exactly what an unset Optional+Computed field
	// still holds straight out of req.Plan.Get.
	v, err := r.client.GetContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
	if err != nil {
		resp.Diagnostics.AddError("Error reading back created Matomo Tag Manager variable", err.Error())
		return
	}

	common.ID = types.StringValue(buildEntityID(siteID, idContainer, idVariable))
	common.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	common.Name = types.StringValue(v.Name)
	common.Description = types.StringValue(v.Description)
	common.DefaultValue = variableDefaultValueFromAPI(v.DefaultValue)

	model.FromParams(v.Parameters)
	restoreListFields(model, savedListFields)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.State.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	common := model.Common()

	siteID, idContainer, idVariable, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	v, err := r.client.GetContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
	if err != nil {
		// "Variable does not exist" is confirmed (via the _disappears
		// acceptance test against a real Matomo instance) to be the exact
		// error string TagManager.getContainerVariable returns for an
		// unknown variable.
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Variable does not exist" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager variable", err.Error())
		return
	}

	common.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	common.Name = types.StringValue(v.Name)
	common.Description = types.StringValue(v.Description)
	common.DefaultValue = variableDefaultValueFromAPI(v.DefaultValue)

	model.FromParams(v.Parameters)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.Plan.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	common := model.Common()

	siteID, idContainer, idVariable, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	var defaultValue *string
	if !common.DefaultValue.IsUnknown() && !common.DefaultValue.IsNull() {
		v := common.DefaultValue.ValueString()
		defaultValue = &v
	}

	if err := r.client.UpdateContainerVariable(ctx, siteID, idContainer, versionID, idVariable, matomo.VariableParams{
		Type:         model.Meta().TypeID,
		Name:         common.Name.ValueString(),
		Description:  common.Description.ValueString(),
		Parameters:   model.ToParams(),
		DefaultValue: defaultValue,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager variable", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *typedVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	model := r.newModel()
	resp.Diagnostics.Append(req.State.Get(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	common := model.Common()

	siteID, idContainer, idVariable, err := parseEntityID(common.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	if err := r.client.DeleteContainerVariable(ctx, siteID, idContainer, versionID, idVariable); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo Tag Manager variable", err.Error())
	}
}

func (r *typedVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &tagManagerVariableResource{}
	_ resource.ResourceWithConfigure   = &tagManagerVariableResource{}
	_ resource.ResourceWithImportState = &tagManagerVariableResource{}
)

func NewTagManagerVariableResource() resource.Resource {
	return &tagManagerVariableResource{}
}

type tagManagerVariableResource struct {
	client *matomo.Client
}

type tagManagerVariableResourceModel struct {
	ID            types.String         `tfsdk:"id"`
	ContainerID   types.String         `tfsdk:"container_id"`
	Type          types.String         `tfsdk:"type"`
	Name          types.String         `tfsdk:"name"`
	DefaultValue  types.String         `tfsdk:"default_value"`
	Parameter     []tagParameterModel  `tfsdk:"parameter"`
	ParameterList []parameterListModel `tfsdk:"parameter_list"`
}

func (r *tagManagerVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_variable"
}

func (r *tagManagerVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A generic Tag Manager variable, configured with untyped name/value parameter pairs. Use this only for variable types that don't have their own dedicated matomo_tagmanager_variable_<type> resource (e.g. a third-party-plugin-contributed type, or a type newer than this provider's last regeneration) - every built-in Matomo variable type has a typed resource with real, validated fields; see the matomo_tagmanager_variable_types data source to check what's available.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/container_id/variable_id\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The owning container's id (matomo_tagmanager_container.x.id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The Matomo variable type, e.g. \"Constant\". See the matomo_tagmanager_variable_types data source for valid values.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The variable's display name.",
			},
			"default_value": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Value used when the variable cannot be resolved.",
			},
		},
		Blocks: map[string]schema.Block{
			"parameter": schema.ListNestedBlock{
				Description: "Type-specific configuration as name/value pairs.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":  schema.StringAttribute{Required: true},
						"value": schema.StringAttribute{Required: true},
					},
				},
			},
			"parameter_list": schema.ListNestedBlock{
				Description: "A single named parameter whose value is a list of rows, each with arbitrary key/value items - for parameter types the generic parameter{} block cannot represent (e.g. Matomo's UI_CONTROL_MULTI_TUPLE fields, which need each row's fields sent as name[i][key]=value, not a flat list). Prefer a typed resource over this when one exists for your type - a typed resource's real nested block (e.g. custom_dimension{index,value}) is validated and self-documenting; this generic form is not.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{Required: true},
					},
					Blocks: map[string]schema.Block{
						"row": schema.ListNestedBlock{
							NestedObject: schema.NestedBlockObject{
								Blocks: map[string]schema.Block{
									"item": schema.ListNestedBlock{
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"key":   schema.StringAttribute{Required: true},
												"value": schema.StringAttribute{Required: true},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *tagManagerVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tagManagerVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagManagerVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(plan.ContainerID.ValueString())
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
	if !plan.DefaultValue.IsUnknown() && !plan.DefaultValue.IsNull() {
		v := plan.DefaultValue.ValueString()
		defaultValue = &v
	}

	params := parametersToMap(plan.Parameter)
	for k, v := range parameterListsToMap(plan.ParameterList) {
		params[k] = v
	}

	idVariable, err := r.client.AddContainerVariable(ctx, siteID, idContainer, versionID, matomo.VariableParams{
		Type:         plan.Type.ValueString(),
		Name:         plan.Name.ValueString(),
		Parameters:   params,
		DefaultValue: defaultValue,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager variable", err.Error())
		return
	}

	plan.ID = types.StringValue(buildEntityID(siteID, idContainer, idVariable))

	// default_value is Optional+Computed (Matomo can default it
	// server-side even when never sent) - reading it back here, the same
	// way Read does, is required so an unconfigured default_value never
	// stays Unknown in the state Create writes: confirmed against a real
	// acceptance-test run ("Provider returned invalid result object
	// after apply ... default_value: all values must be known").
	v, err := r.client.GetContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
	if err != nil {
		resp.Diagnostics.AddError("Error reading back created Matomo Tag Manager variable", err.Error())
		return
	}
	plan.DefaultValue = types.StringValue(v.DefaultValue)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagManagerVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idVariable, err := parseEntityID(state.ID.ValueString())
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

	state.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	state.Type = types.StringValue(v.Type)
	state.Name = types.StringValue(v.Name)
	state.DefaultValue = types.StringValue(v.DefaultValue)

	params := make([]tagParameterModel, 0, len(v.Parameters))
	for name, value := range v.Parameters {
		params = append(params, tagParameterModel{Name: types.StringValue(name), Value: types.StringValue(paramValueDisplayString(value))})
	}
	// v.Parameters is a map, so Go's iteration order above is randomized per
	// process. Sort by name so Read's output is deterministic across runs,
	// which is what avoids perpetual plan diffs on parameter (an
	// order-sensitive ListNestedBlock) — otherwise the resulting slice order
	// would vary from refresh to refresh even when the underlying data is
	// unchanged. (Tasks 16 and 17 both hit or proactively avoided this exact
	// bug; this resource follows the same fix from the start.)
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name.ValueString() < params[j].Name.ValueString()
	})
	state.Parameter = params
	state.ParameterList = parameterListsFromAPI(v.Parameters)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagManagerVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tagManagerVariableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idVariable, err := parseEntityID(plan.ID.ValueString())
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
	if !plan.DefaultValue.IsUnknown() && !plan.DefaultValue.IsNull() {
		v := plan.DefaultValue.ValueString()
		defaultValue = &v
	}

	params := parametersToMap(plan.Parameter)
	for k, v := range parameterListsToMap(plan.ParameterList) {
		params[k] = v
	}

	if err := r.client.UpdateContainerVariable(ctx, siteID, idContainer, versionID, idVariable, matomo.VariableParams{
		Type:         plan.Type.ValueString(),
		Name:         plan.Name.ValueString(),
		Parameters:   params,
		DefaultValue: defaultValue,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager variable", err.Error())
		return
	}

	// See Create's identical read-back for why this is required whenever
	// default_value is left unconfigured.
	v, err := r.client.GetContainerVariable(ctx, siteID, idContainer, versionID, idVariable)
	if err != nil {
		resp.Diagnostics.AddError("Error reading back updated Matomo Tag Manager variable", err.Error())
		return
	}
	plan.DefaultValue = types.StringValue(v.DefaultValue)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagManagerVariableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idVariable, err := parseEntityID(state.ID.ValueString())
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

func (r *tagManagerVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

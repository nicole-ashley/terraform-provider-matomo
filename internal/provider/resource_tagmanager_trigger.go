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
	_ resource.Resource                = &tagManagerTriggerResource{}
	_ resource.ResourceWithConfigure   = &tagManagerTriggerResource{}
	_ resource.ResourceWithImportState = &tagManagerTriggerResource{}
)

func NewTagManagerTriggerResource() resource.Resource {
	return &tagManagerTriggerResource{}
}

type tagManagerTriggerResource struct {
	client *matomo.Client
}

type triggerConditionModel struct {
	Comparison types.String `tfsdk:"comparison"`
	Actual     types.String `tfsdk:"actual"`
	Value      types.String `tfsdk:"value"`
}

type tagManagerTriggerResourceModel struct {
	ID          types.String            `tfsdk:"id"`
	ContainerID types.String            `tfsdk:"container_id"`
	Type        types.String            `tfsdk:"type"`
	Name        types.String            `tfsdk:"name"`
	Parameter   []tagParameterModel     `tfsdk:"parameter"`
	Condition   []triggerConditionModel `tfsdk:"condition"`
}

func (r *tagManagerTriggerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_trigger"
}

func (r *tagManagerTriggerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/container_id/trigger_id\".",
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
				Description: "The Matomo trigger type, e.g. \"PageView\". See the matomo_tagmanager_trigger_types data source for valid values.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The trigger's display name.",
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
			"condition": schema.ListNestedBlock{
				Description: "Conditions that must all match for this trigger to fire.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"comparison": schema.StringAttribute{Required: true},
						"actual":     schema.StringAttribute{Required: true, Description: "A Matomo \"actual value\" identifier (e.g. \"url_path\") or a variable macro reference (e.g. \"{{My Variable}}\")."},
						"value":      schema.StringAttribute{Required: true},
					},
				},
			},
		},
	}
}

func (r *tagManagerTriggerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func conditionsToParams(conditions []triggerConditionModel) []matomo.Condition {
	out := make([]matomo.Condition, len(conditions))
	for i, c := range conditions {
		out[i] = matomo.Condition{
			Comparison:            c.Comparison.ValueString(),
			ActualValueVariableID: c.Actual.ValueString(),
			ExpectedValue:         c.Value.ValueString(),
		}
	}
	return out
}

func conditionsFromAPI(conditions []matomo.Condition) []triggerConditionModel {
	out := make([]triggerConditionModel, len(conditions))
	for i, c := range conditions {
		out[i] = triggerConditionModel{
			Comparison: types.StringValue(c.Comparison),
			Actual:     types.StringValue(c.ActualValueVariableID),
			Value:      types.StringValue(c.ExpectedValue),
		}
	}
	return out
}

func (r *tagManagerTriggerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagManagerTriggerResourceModel
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

	idTrigger, err := r.client.AddContainerTrigger(ctx, siteID, idContainer, versionID, matomo.TriggerParams{
		Type:       plan.Type.ValueString(),
		Name:       plan.Name.ValueString(),
		Parameters: parametersToMap(plan.Parameter),
		Conditions: conditionsToParams(plan.Condition),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager trigger", err.Error())
		return
	}

	plan.ID = types.StringValue(buildEntityID(siteID, idContainer, idTrigger))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerTriggerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagManagerTriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTrigger, err := parseEntityID(state.ID.ValueString())
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

	state.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	state.Type = types.StringValue(trig.Type)
	state.Name = types.StringValue(trig.Name)
	state.Condition = conditionsFromAPI(trig.Conditions)

	params := make([]tagParameterModel, 0, len(trig.Parameters))
	for name, value := range trig.Parameters {
		params = append(params, tagParameterModel{Name: types.StringValue(name), Value: types.StringValue(paramValueDisplayString(value))})
	}
	// trig.Parameters is a map, so Go's iteration order above is randomized
	// per-process. Sort by name so Read's output is deterministic across
	// runs, which is what avoids perpetual plan diffs on parameter (an
	// order-sensitive ListNestedBlock) — otherwise the resulting slice order
	// would vary from refresh to refresh even when the underlying data is
	// unchanged. (trig.Conditions is a real ordered []Condition slice from
	// the JSON array response, so it needs no such sorting.)
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name.ValueString() < params[j].Name.ValueString()
	})
	state.Parameter = params

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagManagerTriggerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tagManagerTriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTrigger, err := parseEntityID(plan.ID.ValueString())
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
		Type:       plan.Type.ValueString(),
		Name:       plan.Name.ValueString(),
		Parameters: parametersToMap(plan.Parameter),
		Conditions: conditionsToParams(plan.Condition),
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager trigger", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerTriggerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagManagerTriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTrigger, err := parseEntityID(state.ID.ValueString())
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

func (r *tagManagerTriggerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

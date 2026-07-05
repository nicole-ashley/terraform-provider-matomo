package provider

import (
	"context"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &tagManagerTagResource{}
	_ resource.ResourceWithConfigure   = &tagManagerTagResource{}
	_ resource.ResourceWithImportState = &tagManagerTagResource{}
)

func NewTagManagerTagResource() resource.Resource {
	return &tagManagerTagResource{}
}

type tagManagerTagResource struct {
	client *matomo.Client
}

type tagParameterModel struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

type tagManagerTagResourceModel struct {
	ID              types.String         `tfsdk:"id"`
	ContainerID     types.String         `tfsdk:"container_id"`
	Type            types.String         `tfsdk:"type"`
	Name            types.String         `tfsdk:"name"`
	Status          types.String         `tfsdk:"status"`
	Description     types.String         `tfsdk:"description"`
	Priority        types.Int64          `tfsdk:"priority"`
	FireTriggerIDs  []types.String       `tfsdk:"fire_trigger_ids"`
	BlockTriggerIDs []types.String       `tfsdk:"block_trigger_ids"`
	Parameter       []tagParameterModel  `tfsdk:"parameter"`
	ParameterList   []parameterListModel `tfsdk:"parameter_list"`
}

func (r *tagManagerTagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_tag"
}

func (r *tagManagerTagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A generic Tag Manager tag, configured with untyped name/value parameter pairs. Use this only for tag types that don't have their own dedicated matomo_tagmanager_tag_<type> resource (e.g. a third-party-plugin-contributed type, or a type newer than this provider's last regeneration) - every built-in Matomo tag type has a typed resource with real, validated fields; see the matomo_tagmanager_tag_types data source to check what's available.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite \"site_id/container_id/tag_id\".",
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
				Description: "The Matomo tag type, e.g. \"CustomHtml\". See the matomo_tagmanager_tag_types data source for valid values.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The tag's display name.",
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "\"active\" or \"paused\". Changing this edits the draft version only — like every other field on this resource, it has no effect on a live container until a new version is created and published.",
				Validators: []validator.String{
					stringvalidator.OneOf("active", "paused"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional free-text description, shown in Matomo's Tag Manager UI.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"priority": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Execution priority - lower values fire earlier when multiple tags fire on the same trigger. Matomo defaults to 999 when unset.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"fire_trigger_ids": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Trigger ids (matomo_tagmanager_trigger.x.id) that fire this tag. Matomo requires at least one.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"block_trigger_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Trigger ids (matomo_tagmanager_trigger.x.id) that block this tag from firing. Note: writing an explicit empty list (`[]`) rather than omitting this attribute will produce a one-time diff to null on the first refresh after apply; this is harmless and converges after one plan/apply cycle.",
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

func (r *tagManagerTagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// parametersToMap builds the flat parameters map the generic tag/trigger/
// variable resources send: every entry is user-typed as a single string
// (there's no way to declare an array-typed parameter through the
// generic parameter{name=...,value=...} block), so every value is a
// matomo.ScalarParam.
func parametersToMap(params []tagParameterModel) matomo.ParamsMap {
	m := make(matomo.ParamsMap, len(params))
	for _, p := range params {
		m[p.Name.ValueString()] = matomo.ScalarParam(p.Value.ValueString())
	}
	return m
}

// paramValueDisplayString renders a matomo.ParamValue for the generic
// resources' string-only parameter{} block - a list-typed value (which
// that block has no way to represent structurally) is joined with commas
// for read-only display purposes only; it is never sent back to Matomo
// in this form (parametersToMap above always sends a plain scalar).
func paramValueDisplayString(v matomo.ParamValue) string {
	if v.IsList() {
		return strings.Join(v.List, ",")
	}
	return v.Scalar
}

// parameterListModel is the generic resources' equivalent of a typed
// resource's real nested block (e.g. custom_dimension{index,value}) - since
// the generic resources have no per-type knowledge of a parameter's real
// row shape, each row is a generic list of key/value items rather than
// named attributes.
type parameterListModel struct {
	Name types.String        `tfsdk:"name"`
	Row  []parameterRowModel `tfsdk:"row"`
}

type parameterRowModel struct {
	Item []parameterItemModel `tfsdk:"item"`
}

type parameterItemModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

// parameterListsToMap builds the ListOfObjects-shaped entries of a
// Tag/Trigger/Variable's "parameters" map from the generic parameter_list
// blocks - the counterpart to parametersToMap, which only builds scalar
// entries from the plain parameter{} blocks.
func parameterListsToMap(lists []parameterListModel) matomo.ParamsMap {
	m := make(matomo.ParamsMap, len(lists))
	for _, pl := range lists {
		rows := make([]map[string]string, len(pl.Row))
		for i, row := range pl.Row {
			r := make(map[string]string, len(row.Item))
			for _, item := range row.Item {
				r[item.Key.ValueString()] = item.Value.ValueString()
			}
			rows[i] = r
		}
		m[pl.Name.ValueString()] = matomo.ListOfObjectsParam(rows)
	}
	return m
}

// parameterListsFromAPI extracts every ListOfObjects-shaped entry of params
// back into parameter_list blocks, sorting each row's items by key
// alphabetically for deterministic state (Matomo's own response has no
// inherent key order within a row, since it's decoded from a JSON object).
func parameterListsFromAPI(params matomo.ParamsMap) []parameterListModel {
	var lists []parameterListModel
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		val := params[name]
		if !val.IsListOfObjects() {
			continue
		}
		rows := make([]parameterRowModel, len(val.ListOfObjects))
		for i, row := range val.ListOfObjects {
			keys := make([]string, 0, len(row))
			for k := range row {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			items := make([]parameterItemModel, len(keys))
			for j, k := range keys {
				items[j] = parameterItemModel{Key: types.StringValue(k), Value: types.StringValue(row[k])}
			}
			rows[i] = parameterRowModel{Item: items}
		}
		lists = append(lists, parameterListModel{Name: types.StringValue(name), Row: rows})
	}
	return lists
}

func stringSliceFromModel(in []types.String) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = v.ValueString()
	}
	return out
}

// stringModelFromSlice converts a slice into its tfsdk model form. An empty
// or nil slice becomes a nil (null) Go slice rather than an empty one, so
// that state agrees with a plan where the corresponding list attribute was
// simply omitted from config — otherwise Terraform sees a perpetual diff
// between an omitted (null) list and Matomo's empty-list API response.
func stringModelFromSlice(in []string) []types.String {
	if len(in) == 0 {
		return nil
	}
	out := make([]types.String, len(in))
	for i, v := range in {
		out[i] = types.StringValue(v)
	}
	return out
}

func (r *tagManagerTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagManagerTagResourceModel
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

	fireIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(plan.FireTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid fire_trigger_ids", err.Error())
		return
	}
	blockIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(plan.BlockTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid block_trigger_ids", err.Error())
		return
	}

	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
	priority := int64(999)
	if !plan.Priority.IsUnknown() && !plan.Priority.IsNull() {
		priority = plan.Priority.ValueInt64()
	}

	params := parametersToMap(plan.Parameter)
	for k, v := range parameterListsToMap(plan.ParameterList) {
		params[k] = v
	}

	idTag, err := r.client.AddContainerTag(ctx, siteID, idContainer, versionID, matomo.TagParams{
		Type:            plan.Type.ValueString(),
		Name:            plan.Name.ValueString(),
		Description:     description,
		Priority:        int(priority),
		Parameters:      params,
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager tag", err.Error())
		return
	}

	status := "active"
	if !plan.Status.IsUnknown() && !plan.Status.IsNull() {
		status = plan.Status.ValueString()
	}
	if status == "paused" {
		if err := r.client.PauseContainerTag(ctx, siteID, idContainer, versionID, idTag); err != nil {
			resp.Diagnostics.AddError("Error pausing Matomo Tag Manager tag", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(buildEntityID(siteID, idContainer, idTag))
	plan.Status = types.StringValue(status)
	plan.Description = types.StringValue(description)
	plan.Priority = types.Int64Value(priority)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagManagerTagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(state.ID.ValueString())
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
		// "Tag does not exist" is confirmed (via the _disappears acceptance
		// test against a real Matomo instance) to be the exact error string
		// TagManager.getContainerTag returns for an unknown tag.
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Tag does not exist" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo Tag Manager tag", err.Error())
		return
	}

	state.ContainerID = types.StringValue(buildContainerID(siteID, idContainer))
	state.Type = types.StringValue(tag.Type)
	state.Name = types.StringValue(tag.Name)
	state.Status = types.StringValue(tag.Status)
	state.Description = types.StringValue(tag.Description)
	state.Priority = types.Int64Value(int64(tag.Priority))
	state.FireTriggerIDs = stringModelFromSlice(compositeEntityIDs(siteID, idContainer, intsToStrings(tag.FireTriggerIDs)))
	state.BlockTriggerIDs = stringModelFromSlice(compositeEntityIDs(siteID, idContainer, intsToStrings(tag.BlockTriggerIDs)))

	params := make([]tagParameterModel, 0, len(tag.Parameters))
	for name, value := range tag.Parameters {
		if value.IsListOfObjects() {
			continue // represented by the parameter_list block instead
		}
		params = append(params, tagParameterModel{Name: types.StringValue(name), Value: types.StringValue(paramValueDisplayString(value))})
	}
	// tag.Parameters is a map, so Go's iteration order above is randomized
	// per-process. Sort by name so Read's output is deterministic across
	// runs, which is what avoids perpetual plan diffs on parameter (a
	// order-sensitive ListNestedBlock) — otherwise the resulting slice order
	// would vary from refresh to refresh even when the underlying data is
	// unchanged.
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name.ValueString() < params[j].Name.ValueString()
	})
	state.Parameter = params
	state.ParameterList = parameterListsFromAPI(tag.Parameters)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagManagerTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tagManagerTagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id in state", err.Error())
		return
	}

	versionID, err := resolveDraftVersionID(ctx, r.client, siteID, idContainer)
	if err != nil {
		resp.Diagnostics.AddError("Error resolving draft container version", err.Error())
		return
	}

	fireIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(plan.FireTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid fire_trigger_ids", err.Error())
		return
	}
	blockIDs, err := bareEntityIDs(siteID, idContainer, stringSliceFromModel(plan.BlockTriggerIDs))
	if err != nil {
		resp.Diagnostics.AddError("Invalid block_trigger_ids", err.Error())
		return
	}

	description := ""
	if !plan.Description.IsUnknown() && !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}
	priority := int64(999)
	if !plan.Priority.IsUnknown() && !plan.Priority.IsNull() {
		priority = plan.Priority.ValueInt64()
	}

	params := parametersToMap(plan.Parameter)
	for k, v := range parameterListsToMap(plan.ParameterList) {
		params[k] = v
	}

	if err := r.client.UpdateContainerTag(ctx, siteID, idContainer, versionID, idTag, matomo.TagParams{
		Type:            plan.Type.ValueString(),
		Name:            plan.Name.ValueString(),
		Description:     description,
		Priority:        int(priority),
		Parameters:      params,
		FireTriggerIDs:  fireIDs,
		BlockTriggerIDs: blockIDs,
	}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo Tag Manager tag", err.Error())
		return
	}

	status := "active"
	if !plan.Status.IsUnknown() && !plan.Status.IsNull() {
		status = plan.Status.ValueString()
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

	plan.Status = types.StringValue(status)
	plan.Description = types.StringValue(description)
	plan.Priority = types.Int64Value(priority)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagManagerTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagManagerTagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, idTag, err := parseEntityID(state.ID.ValueString())
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

func (r *tagManagerTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

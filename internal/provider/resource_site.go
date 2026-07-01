package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ resource.Resource                = &siteResource{}
	_ resource.ResourceWithConfigure   = &siteResource{}
	_ resource.ResourceWithImportState = &siteResource{}
)

func NewSiteResource() resource.Resource {
	return &siteResource{}
}

type siteResource struct {
	client *matomo.Client
}

type siteResourceModel struct {
	ID       types.String   `tfsdk:"id"`
	Name     types.String   `tfsdk:"name"`
	URLs     []types.String `tfsdk:"urls"`
	Timezone types.String   `tfsdk:"timezone"`
	Currency types.String   `tfsdk:"currency"`
}

func (r *siteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site"
}

func (r *siteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The site's numeric ID, assigned by Matomo on creation.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The site's name.",
			},
			"urls": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "URLs of the site (at least one is required). The first URL is the site's main URL; additional URLs are treated as aliases by Matomo's tracking.",
			},
			"timezone": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The site's timezone, e.g. \"UTC\" or \"America/New_York\".",
			},
			"currency": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The site's currency code, e.g. \"USD\".",
			},
		},
	}
}

func (r *siteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *siteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan siteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := matomo.AddSiteParams{Name: plan.Name.ValueString(), URLs: stringSliceFromModel(plan.URLs)}
	if !plan.Timezone.IsUnknown() && !plan.Timezone.IsNull() {
		tz := plan.Timezone.ValueString()
		params.Timezone = &tz
	}
	if !plan.Currency.IsUnknown() && !plan.Currency.IsNull() {
		cur := plan.Currency.ValueString()
		params.Currency = &cur
	}

	idSite, err := r.client.AddSite(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo site", err.Error())
		return
	}

	r.readIntoModel(ctx, idSite, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state siteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSite, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site id in state", err.Error())
		return
	}

	site, err := r.client.GetSiteFromID(ctx, idSite)
	if err != nil {
		// NOTE: "Website id Not found" is the exact error string this provider
		// has always assumed Matomo's getSiteFromId returns for an unknown
		// idSite, but it has never been verified against a live Matomo
		// instance. If Matomo's real wire format differs, a site deleted out
		// of band (e.g. directly in Matomo) will surface as a hard error here
		// instead of being silently removed from state. Verifying this string
		// is a gate for the acceptance-test plan that stands up a real
		// Matomo fixture.
		if apiErr, ok := err.(*matomo.APIError); ok && apiErr.Message == "Website id Not found" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Matomo site", err.Error())
		return
	}

	state.ID = types.StringValue(strconv.Itoa(site.IDSite))
	state.Name = types.StringValue(site.Name)
	state.URLs = stringModelFromSlice(site.URLs)
	state.Timezone = types.StringValue(site.Timezone)
	state.Currency = types.StringValue(site.Currency)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *siteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan siteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state siteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSite, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site id in state", err.Error())
		return
	}

	params := matomo.AddSiteParams{Name: plan.Name.ValueString(), URLs: stringSliceFromModel(plan.URLs)}
	if !plan.Timezone.IsUnknown() && !plan.Timezone.IsNull() {
		tz := plan.Timezone.ValueString()
		params.Timezone = &tz
	}
	if !plan.Currency.IsUnknown() && !plan.Currency.IsNull() {
		cur := plan.Currency.ValueString()
		params.Currency = &cur
	}

	if err := r.client.UpdateSite(ctx, idSite, matomo.UpdateSiteParams{AddSiteParams: params}); err != nil {
		resp.Diagnostics.AddError("Error updating Matomo site", err.Error())
		return
	}

	r.readIntoModel(ctx, idSite, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *siteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state siteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSite, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid site id in state", err.Error())
		return
	}

	if err := r.client.DeleteSite(ctx, idSite); err != nil {
		resp.Diagnostics.AddError("Error deleting Matomo site", err.Error())
	}
}

func (r *siteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// readIntoModel fetches the site by ID and copies its fields into model,
// preserving model.ID. Used by Create/Update to refresh computed fields
// after a write.
func (r *siteResource) readIntoModel(ctx context.Context, idSite int, model *siteResourceModel, diags *diag.Diagnostics) {
	site, err := r.client.GetSiteFromID(ctx, idSite)
	if err != nil {
		diags.AddError("Error reading Matomo site after write", err.Error())
		return
	}
	model.ID = types.StringValue(strconv.Itoa(site.IDSite))
	model.Name = types.StringValue(site.Name)
	model.URLs = stringModelFromSlice(site.URLs)
	model.Timezone = types.StringValue(site.Timezone)
	model.Currency = types.StringValue(site.Currency)
}

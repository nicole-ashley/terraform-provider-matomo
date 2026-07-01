package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &siteDataSource{}
	_ datasource.DataSourceWithConfigure = &siteDataSource{}
)

func NewSiteDataSource() datasource.DataSource {
	return &siteDataSource{}
}

type siteDataSource struct {
	client *matomo.Client
}

type siteDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Timezone types.String `tfsdk:"timezone"`
	Currency types.String `tfsdk:"currency"`
}

func (d *siteDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site"
}

func (d *siteDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The site's numeric ID. Exactly one of id or name is required.",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(
						stringAttrPath("id"), stringAttrPath("name"),
					),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The site's name. Exactly one of id or name is required.",
			},
			"timezone": schema.StringAttribute{Computed: true},
			"currency": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *siteDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	d.client = client
}

func (d *siteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config siteDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var site *matomo.Site

	if !config.ID.IsNull() {
		idSite, err := strconv.Atoi(config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid id", err.Error())
			return
		}
		s, err := d.client.GetSiteFromID(ctx, idSite)
		if err != nil {
			resp.Diagnostics.AddError("Error reading Matomo site", err.Error())
			return
		}
		site = s
	} else {
		sites, err := d.client.GetAllSites(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error listing Matomo sites", err.Error())
			return
		}
		for i := range sites {
			if sites[i].Name == config.Name.ValueString() {
				site = &sites[i]
				break
			}
		}
		if site == nil {
			resp.Diagnostics.AddError("Site not found", fmt.Sprintf("no site named %q", config.Name.ValueString()))
			return
		}
	}

	config.ID = types.StringValue(strconv.Itoa(site.IDSite))
	config.Name = types.StringValue(site.Name)
	config.Timezone = types.StringValue(site.Timezone)
	config.Currency = types.StringValue(site.Currency)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

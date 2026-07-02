package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &tagManagerEnvironmentsDataSource{}
	_ datasource.DataSourceWithConfigure = &tagManagerEnvironmentsDataSource{}
)

func NewTagManagerEnvironmentsDataSource() datasource.DataSource {
	return &tagManagerEnvironmentsDataSource{}
}

type tagManagerEnvironmentsDataSource struct {
	client *matomo.Client
}

type tagManagerEnvironmentModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type tagManagerEnvironmentsDataSourceModel struct {
	ID           types.String                 `tfsdk:"id"`
	Environments []tagManagerEnvironmentModel `tfsdk:"environments"`
}

func (d *tagManagerEnvironmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_environments"
}

func (d *tagManagerEnvironmentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every Tag Manager publish environment (e.g. \"live\", \"dev\", \"staging\") configured on this Matomo instance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Synthetic identifier for this data source, always \"environments\".",
			},
			"environments": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Every configured environment.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true, Description: "The environment's id, e.g. \"live\"."},
						"name": schema.StringAttribute{Computed: true, Description: "The environment's display name."},
					},
				},
			},
		},
	}
}

func (d *tagManagerEnvironmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *tagManagerEnvironmentsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	environments, err := d.client.GetAvailableEnvironments(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo Tag Manager environments", err.Error())
		return
	}

	state := tagManagerEnvironmentsDataSourceModel{
		ID:           types.StringValue("environments"),
		Environments: make([]tagManagerEnvironmentModel, len(environments)),
	}
	for i, e := range environments {
		state.Environments[i] = tagManagerEnvironmentModel{ID: types.StringValue(e.ID), Name: types.StringValue(e.Name)}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

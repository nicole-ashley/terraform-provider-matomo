package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &tagManagerContextsDataSource{}
	_ datasource.DataSourceWithConfigure = &tagManagerContextsDataSource{}
)

func NewTagManagerContextsDataSource() datasource.DataSource {
	return &tagManagerContextsDataSource{}
}

type tagManagerContextsDataSource struct {
	client *matomo.Client
}

type tagManagerContextModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type tagManagerContextsDataSourceModel struct {
	ID       types.String             `tfsdk:"id"`
	Contexts []tagManagerContextModel `tfsdk:"contexts"`
}

func (d *tagManagerContextsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_contexts"
}

func (d *tagManagerContextsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every Tag Manager context (e.g. \"web\", \"amp\") this Matomo instance supports.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Synthetic identifier for this data source, always \"contexts\".",
			},
			"contexts": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Every available context.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true, Description: "The context's id, e.g. \"web\"."},
						"name": schema.StringAttribute{Computed: true, Description: "The context's display name."},
					},
				},
			},
		},
	}
}

func (d *tagManagerContextsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *tagManagerContextsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	contexts, err := d.client.GetAvailableContexts(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo Tag Manager contexts", err.Error())
		return
	}

	state := tagManagerContextsDataSourceModel{
		ID:       types.StringValue("contexts"),
		Contexts: make([]tagManagerContextModel, len(contexts)),
	}
	for i, c := range contexts {
		state.Contexts[i] = tagManagerContextModel{ID: types.StringValue(c.ID), Name: types.StringValue(c.Name)}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

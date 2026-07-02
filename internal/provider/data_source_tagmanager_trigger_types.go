// internal/provider/data_source_tagmanager_trigger_types.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &tagManagerTriggerTypesDataSource{}
	_ datasource.DataSourceWithConfigure = &tagManagerTriggerTypesDataSource{}
)

func NewTagManagerTriggerTypesDataSource() datasource.DataSource {
	return &tagManagerTriggerTypesDataSource{}
}

type tagManagerTriggerTypesDataSource struct {
	client *matomo.Client
}

type tagManagerTriggerTypesDataSourceModel struct {
	ID           types.String                 `tfsdk:"id"`
	Context      types.String                 `tfsdk:"context"`
	TriggerTypes []tagManagerTypeSummaryModel `tfsdk:"trigger_types"`
}

func (d *tagManagerTriggerTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_trigger_types"
}

func (d *tagManagerTriggerTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every trigger type available in a given Tag Manager context, including third-party-plugin-contributed ones.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Set to the context value this data source was queried with.",
			},
			"context": schema.StringAttribute{
				Required:    true,
				Description: "The Tag Manager context to query, e.g. \"web\". See the matomo_tagmanager_contexts data source for valid values.",
			},
			"trigger_types": tagManagerTypeSummarySchema("Every trigger type available in the given context."),
		},
	}
}

func (d *tagManagerTriggerTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *tagManagerTriggerTypesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config tagManagerTriggerTypesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	templates, err := d.client.GetAvailableTriggerTypes(ctx, config.Context.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo Tag Manager trigger types", err.Error())
		return
	}

	config.ID = config.Context
	config.TriggerTypes = make([]tagManagerTypeSummaryModel, len(templates))
	for i, t := range templates {
		config.TriggerTypes[i] = tagManagerTypeSummaryModel{
			ID:          types.StringValue(t.ID),
			Name:        types.StringValue(t.Name),
			Description: types.StringValue(t.Description),
			Category:    types.StringValue(t.Category),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

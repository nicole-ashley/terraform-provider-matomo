package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ datasource.DataSource              = &tagManagerTagTypesDataSource{}
	_ datasource.DataSourceWithConfigure = &tagManagerTagTypesDataSource{}
)

func NewTagManagerTagTypesDataSource() datasource.DataSource {
	return &tagManagerTagTypesDataSource{}
}

type tagManagerTagTypesDataSource struct {
	client *matomo.Client
}

type tagManagerTypeSummaryModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Category    types.String `tfsdk:"category"`
}

type tagManagerTagTypesDataSourceModel struct {
	ID       types.String                 `tfsdk:"id"`
	Context  types.String                 `tfsdk:"context"`
	TagTypes []tagManagerTypeSummaryModel `tfsdk:"tag_types"`
}

func (d *tagManagerTagTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_tag_types"
}

func tagManagerTypeSummarySchema(description string) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed:    true,
		Description: description,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"id":          schema.StringAttribute{Computed: true, Description: "The type's id, e.g. \"CustomHtml\"."},
				"name":        schema.StringAttribute{Computed: true, Description: "The type's display name."},
				"description": schema.StringAttribute{Computed: true, Description: "The type's description."},
				"category":    schema.StringAttribute{Computed: true, Description: "The category this type is grouped under in Matomo's UI."},
			},
		},
	}
}

func (d *tagManagerTagTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every tag type available in a given Tag Manager context, including third-party-plugin-contributed ones.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Set to the context value this data source was queried with.",
			},
			"context": schema.StringAttribute{
				Required:    true,
				Description: "The Tag Manager context to query, e.g. \"web\". See the matomo_tagmanager_contexts data source for valid values.",
			},
			"tag_types": tagManagerTypeSummarySchema("Every tag type available in the given context."),
		},
	}
}

func (d *tagManagerTagTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *tagManagerTagTypesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config tagManagerTagTypesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	templates, err := d.client.GetAvailableTagTypes(ctx, config.Context.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing Matomo Tag Manager tag types", err.Error())
		return
	}

	config.ID = config.Context
	config.TagTypes = make([]tagManagerTypeSummaryModel, len(templates))
	for i, t := range templates {
		config.TagTypes[i] = tagManagerTypeSummaryModel{
			ID:          types.StringValue(t.ID),
			Name:        types.StringValue(t.Name),
			Description: types.StringValue(t.Description),
			Category:    types.StringValue(t.Category),
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

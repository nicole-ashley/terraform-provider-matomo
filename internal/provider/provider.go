package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ provider.Provider = &MatomoProvider{}

type MatomoProvider struct {
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &MatomoProvider{version: version}
	}
}

func (p *MatomoProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "matomo"
	resp.Version = p.version
}

func (p *MatomoProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of the Matomo instance, e.g. https://analytics.example.com. May also be set via the MATOMO_BASE_URL environment variable.",
			},
			"api_token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Matomo API token (token_auth). May also be set via the MATOMO_API_TOKEN environment variable.",
			},
			"insecure_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Description: "Skip TLS certificate verification. Only use for self-hosted instances with internal CAs.",
			},
		},
	}
}

func (p *MatomoProvider) Configure(_ context.Context, _ provider.ConfigureRequest, _ *provider.ConfigureResponse) {
}

func (p *MatomoProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *MatomoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

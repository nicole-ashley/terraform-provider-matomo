package provider

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
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

type matomoProviderModel struct {
	BaseURL            types.String `tfsdk:"base_url"`
	APIToken           types.String `tfsdk:"api_token"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
}

func (p *MatomoProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config matomoProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("MATOMO_BASE_URL")
	}
	if baseURL == "" {
		resp.Diagnostics.AddError(
			"Missing Matomo base URL",
			"Set base_url in the provider configuration or the MATOMO_BASE_URL environment variable.",
		)
	}

	apiToken := config.APIToken.ValueString()
	if apiToken == "" {
		apiToken = os.Getenv("MATOMO_API_TOKEN")
	}
	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing Matomo API token",
			"Set api_token in the provider configuration or the MATOMO_API_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	httpClient := &http.Client{}
	if config.InsecureSkipVerify.ValueBool() {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // explicit opt-in via provider config
		}
	}

	client := matomo.NewClient(baseURL, apiToken, httpClient)
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *MatomoProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSiteResource,
		NewCustomDimensionResource,
	}
}

func (p *MatomoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSiteDataSource,
	}
}

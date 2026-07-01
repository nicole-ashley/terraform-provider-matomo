package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

func TestMatomoProvider_Metadata(t *testing.T) {
	p := New("test")()
	resp := &provider.MetadataResponse{}
	p.Metadata(nil, provider.MetadataRequest{}, resp)

	if resp.TypeName != "matomo" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "matomo")
	}
}

func TestMatomoProvider_Schema(t *testing.T) {
	p := New("test")()
	resp := &provider.SchemaResponse{}
	p.Schema(nil, provider.SchemaRequest{}, resp)

	for _, attr := range []string{"base_url", "api_token", "insecure_skip_verify"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("schema missing attribute %q", attr)
		}
	}
}

func TestMatomoProvider_Configure(t *testing.T) {
	t.Setenv("MATOMO_BASE_URL", "")
	t.Setenv("MATOMO_API_TOKEN", "")

	p := New("test")().(*MatomoProvider)
	req := provider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw: tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"base_url":             tftypes.String,
					"api_token":            tftypes.String,
					"insecure_skip_verify": tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"base_url":             tftypes.NewValue(tftypes.String, "https://matomo.example.com"),
				"api_token":            tftypes.NewValue(tftypes.String, "test-token"),
				"insecure_skip_verify": tftypes.NewValue(tftypes.Bool, nil),
			}),
			Schema: func() schema.Schema {
				resp := &provider.SchemaResponse{}
				p.Schema(context.Background(), provider.SchemaRequest{}, resp)
				return resp.Schema
			}(),
		},
	}
	resp := &provider.ConfigureResponse{}
	p.Configure(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure() diagnostics = %v", resp.Diagnostics)
	}
	if resp.ResourceData == nil {
		t.Fatal("Configure() resp.ResourceData = nil, want *matomo.Client")
	}
	if _, ok := resp.ResourceData.(*matomo.Client); !ok {
		t.Fatalf("Configure() resp.ResourceData type = %T, want *matomo.Client", resp.ResourceData)
	}
}

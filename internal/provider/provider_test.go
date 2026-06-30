package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
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

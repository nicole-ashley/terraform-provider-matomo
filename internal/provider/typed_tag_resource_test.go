package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type fakeTagModel struct {
	Value types.String `tfsdk:"value"`
}

func (m *fakeTagModel) Meta() typedMeta {
	return typedMeta{
		TypeID:       "FakeType",
		ResourceName: "matomo_tagmanager_tag_faketype",
		Schema: schema.Schema{
			Attributes: map[string]schema.Attribute{
				"value": schema.StringAttribute{Required: true},
			},
		},
	}
}

func (m *fakeTagModel) ToParams() map[string]string {
	return map[string]string{"value": m.Value.ValueString()}
}
func (m *fakeTagModel) FromParams(p map[string]string) { m.Value = types.StringValue(p["value"]) }

func TestTypedTagResource_metadataAndSchemaDispatchToModel(t *testing.T) {
	r := newTypedTagResource(func() typedModel { return &fakeTagModel{} }).(*typedTagResource)

	var metaResp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{}, &metaResp)
	if metaResp.TypeName != "matomo_tagmanager_tag_faketype" {
		t.Errorf("TypeName = %q, want matomo_tagmanager_tag_faketype", metaResp.TypeName)
	}

	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	if _, ok := schemaResp.Schema.Attributes["value"]; !ok {
		t.Error("Schema() did not include the model's \"value\" attribute")
	}
}

var _ typedModel = &fakeTagModel{}

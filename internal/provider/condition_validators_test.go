package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func rawConfigWithString(t *testing.T, attrName, value string) tftypes.Value {
	t.Helper()
	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			attrName: tftypes.String,
			"value":  tftypes.String,
		},
	}
	return tftypes.NewValue(objType, map[string]tftypes.Value{
		attrName: tftypes.NewValue(tftypes.String, value),
		"value":  tftypes.NewValue(tftypes.String, "anything"),
	})
}

func testSchemaWithStringAttrs(names ...string) schema.Schema {
	attrs := map[string]schema.Attribute{}
	for _, n := range names {
		attrs[n] = schema.StringAttribute{Optional: true}
	}
	return schema.Schema{Attributes: attrs}
}

func TestConditionEqualsValidator(t *testing.T) {
	v := conditionEqualsValidator{Field: "trigger_type", Value: "pageview"}

	req := validator.StringRequest{
		Path:           path.Root("value"),
		PathExpression: path.MatchRoot("value"),
		ConfigValue:    types.StringValue("anything"),
	}

	// Build a minimal raw config where trigger_type == "pageview": the
	// validator should report no error.
	config := tfsdk.Config{
		Raw:    rawConfigWithString(t, "trigger_type", "pageview"),
		Schema: testSchemaWithStringAttrs("trigger_type", "value"),
	}
	req.Config = config

	var resp validator.StringResponse
	v.ValidateString(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ValidateString() with matching trigger_type produced errors: %v", resp.Diagnostics)
	}

	// Now trigger_type == "click": the validator should report an error.
	config2 := tfsdk.Config{
		Raw:    rawConfigWithString(t, "trigger_type", "click"),
		Schema: testSchemaWithStringAttrs("trigger_type", "value"),
	}
	req.Config = config2
	var resp2 validator.StringResponse
	v.ValidateString(context.Background(), req, &resp2)
	if !resp2.Diagnostics.HasError() {
		t.Fatal("ValidateString() with non-matching trigger_type produced no error, want one")
	}
}

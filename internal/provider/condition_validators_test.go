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

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
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

// TestConditionRequiredValidator mirrors the real EtrackerTag case this
// validator was built for: etrackerAddToCartProduct (stood in for here by
// the "value" attribute) is only actually required once tracking_type ==
// "addtocart".
func TestConditionRequiredValidator(t *testing.T) {
	v := conditionRequiredValidator{Condition: matomo.EqNode{Field: "tracking_type", Value: "addtocart"}}
	sch := testSchemaWithStringAttrs("tracking_type", "value")

	cases := []struct {
		name        string
		trackingVal string
		configValue types.String
		wantError   bool
	}{
		{"condition holds, value empty -> error", "addtocart", types.StringValue(""), true},
		{"condition holds, value null -> error", "addtocart", types.StringNull(), true},
		{"condition holds, value set -> ok", "addtocart", types.StringValue("some-product"), false},
		{"condition does not hold, value empty -> ok", "event", types.StringValue(""), false},
		{"condition does not hold, value null -> ok", "event", types.StringNull(), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validator.StringRequest{
				Path:           path.Root("value"),
				PathExpression: path.MatchRoot("value"),
				ConfigValue:    c.configValue,
				Config: tfsdk.Config{
					Raw:    rawConfigWithString(t, "tracking_type", c.trackingVal),
					Schema: sch,
				},
			}
			var resp validator.StringResponse
			v.ValidateString(context.Background(), req, &resp)
			if resp.Diagnostics.HasError() != c.wantError {
				t.Errorf("ValidateString() diagnostics.HasError() = %v, want %v (diags: %v)", resp.Diagnostics.HasError(), c.wantError, resp.Diagnostics)
			}
		})
	}
}

// TestConditionRequiredValidator_unknownValueNeverErrors confirms an
// unknown ConfigValue (e.g. it references a resource attribute not yet
// known during plan) never trips the validator, even when the condition
// holds - Terraform re-validates once the value becomes known.
func TestConditionRequiredValidator_unknownValueNeverErrors(t *testing.T) {
	v := conditionRequiredValidator{Condition: matomo.EqNode{Field: "tracking_type", Value: "addtocart"}}
	req := validator.StringRequest{
		Path:           path.Root("value"),
		PathExpression: path.MatchRoot("value"),
		ConfigValue:    types.StringUnknown(),
		Config: tfsdk.Config{
			Raw:    rawConfigWithString(t, "tracking_type", "addtocart"),
			Schema: testSchemaWithStringAttrs("tracking_type", "value"),
		},
	}
	var resp validator.StringResponse
	v.ValidateString(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ValidateString() with unknown ConfigValue produced errors: %v", resp.Diagnostics)
	}
}

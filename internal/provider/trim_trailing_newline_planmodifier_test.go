// internal/provider/trim_trailing_newline_planmodifier_test.go
package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestChompTrailingNewline(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no trailing newline", "function() { return 1; }", "function() { return 1; }"},
		{"single trailing \\n", "function() { return 1; }\n", "function() { return 1; }"},
		{"single trailing \\r\\n", "function() { return 1; }\r\n", "function() { return 1; }"},
		{"double trailing \\n only strips one", "function() { return 1; }\n\n", "function() { return 1; }\n"},
		{"empty string", "", ""},
		{"only a newline", "\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chompTrailingNewline(tt.in); got != tt.want {
				t.Errorf("chompTrailingNewline(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTrimTrailingNewlinePlanModifier_suppressesTrailingNewlineOnlyDiff(t *testing.T) {
	m := trimTrailingNewlinePlanModifier{}

	req := planmodifier.StringRequest{
		StateValue: types.StringValue("function() { return 1; }"),
		PlanValue:  types.StringValue("function() { return 1; }\n"),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	m.PlanModifyString(context.Background(), req, resp)

	if resp.PlanValue.ValueString() != "function() { return 1; }" {
		t.Errorf("PlanValue = %q, want the prior state value unchanged (diff suppressed)", resp.PlanValue.ValueString())
	}
}

func TestTrimTrailingNewlinePlanModifier_realChangeStillDiffs(t *testing.T) {
	m := trimTrailingNewlinePlanModifier{}

	req := planmodifier.StringRequest{
		StateValue: types.StringValue("function() { return 1; }"),
		PlanValue:  types.StringValue("function() { return 2; }\n"),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	m.PlanModifyString(context.Background(), req, resp)

	if resp.PlanValue.ValueString() != "function() { return 2; }\n" {
		t.Errorf("PlanValue = %q, want the configured value unchanged (real diff must not be suppressed)", resp.PlanValue.ValueString())
	}
}

func TestTrimTrailingNewlinePlanModifier_noPriorStateIsNoop(t *testing.T) {
	m := trimTrailingNewlinePlanModifier{}

	req := planmodifier.StringRequest{
		StateValue: types.StringNull(),
		PlanValue:  types.StringValue("function() { return 1; }\n"),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	m.PlanModifyString(context.Background(), req, resp)

	if resp.PlanValue.ValueString() != "function() { return 1; }\n" {
		t.Errorf("PlanValue = %q, want unchanged on create (no prior state)", resp.PlanValue.ValueString())
	}
}

func TestTrimTrailingNewlinePlanModifier_unknownPlanValueIsNoop(t *testing.T) {
	m := trimTrailingNewlinePlanModifier{}

	req := planmodifier.StringRequest{
		StateValue: types.StringValue("function() { return 1; }"),
		PlanValue:  types.StringUnknown(),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	m.PlanModifyString(context.Background(), req, resp)

	if !resp.PlanValue.IsUnknown() {
		t.Errorf("PlanValue = %#v, want to remain Unknown", resp.PlanValue)
	}
}

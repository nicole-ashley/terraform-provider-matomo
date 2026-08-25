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

// On create there is no prior state to compare against, but Create still
// sets state from the value Matomo hands back - with the trailing newline
// stripped - so the plan has to carry the chomped value or Terraform
// rejects the apply with "Provider produced inconsistent result after
// apply". Without this, a heredoc value could not be created at all
// without the user wrapping it in chomp() by hand.
func TestTrimTrailingNewlinePlanModifier_chompsOnCreate(t *testing.T) {
	tests := []struct {
		name string
		plan string
		want string
	}{
		{"trailing \\n chomped", "function() { return 1; }\n", "function() { return 1; }"},
		{"trailing \\r\\n chomped", "function() { return 1; }\r\n", "function() { return 1; }"},
		{"no trailing newline untouched", "function() { return 1; }", "function() { return 1; }"},
		{"only the last of several newlines chomped", "function() { return 1; }\n\n", "function() { return 1; }\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := trimTrailingNewlinePlanModifier{}

			req := planmodifier.StringRequest{
				StateValue: types.StringNull(),
				PlanValue:  types.StringValue(tt.plan),
			}
			resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
			m.PlanModifyString(context.Background(), req, resp)

			if resp.PlanValue.ValueString() != tt.want {
				t.Errorf("PlanValue = %q, want %q (chomped on create, no prior state)", resp.PlanValue.ValueString(), tt.want)
			}
		})
	}
}

// A change to the value itself must still plan as a real diff - only the
// trailing newline is ever normalized away, never the change. The chomp
// applies here for the same reason it does on create: Update writes the
// planned value straight to state, so planning the un-chomped value would
// leave state carrying a newline Matomo never stored.
func TestTrimTrailingNewlinePlanModifier_realChangeStillDiffsAndIsChomped(t *testing.T) {
	m := trimTrailingNewlinePlanModifier{}

	req := planmodifier.StringRequest{
		StateValue: types.StringValue("function() { return 1; }"),
		PlanValue:  types.StringValue("function() { return 2; }\n"),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	m.PlanModifyString(context.Background(), req, resp)

	if resp.PlanValue.ValueString() != "function() { return 2; }" {
		t.Errorf("PlanValue = %q, want the chomped new value (a real change, not suppressed to prior state)", resp.PlanValue.ValueString())
	}
}

// Prior state that only differs by the trailing newline is planned back
// verbatim rather than chomped, so state written by an earlier provider
// version doesn't churn through a no-op update.
func TestTrimTrailingNewlinePlanModifier_priorStateWithNewlineKeptVerbatim(t *testing.T) {
	m := trimTrailingNewlinePlanModifier{}

	req := planmodifier.StringRequest{
		StateValue: types.StringValue("function() { return 1; }\n"),
		PlanValue:  types.StringValue("function() { return 1; }"),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	m.PlanModifyString(context.Background(), req, resp)

	if resp.PlanValue.ValueString() != "function() { return 1; }\n" {
		t.Errorf("PlanValue = %q, want the prior state value verbatim (no churn)", resp.PlanValue.ValueString())
	}
}

func TestTrimTrailingNewlinePlanModifier_nullPlanValueIsNoop(t *testing.T) {
	m := trimTrailingNewlinePlanModifier{}

	req := planmodifier.StringRequest{
		StateValue: types.StringValue("function() { return 1; }"),
		PlanValue:  types.StringNull(),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	m.PlanModifyString(context.Background(), req, resp)

	if !resp.PlanValue.IsNull() {
		t.Errorf("PlanValue = %#v, want to remain Null (destroy plan)", resp.PlanValue)
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

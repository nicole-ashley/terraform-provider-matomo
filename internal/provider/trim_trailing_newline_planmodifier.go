// internal/provider/trim_trailing_newline_planmodifier.go
package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// trimTrailingNewlinePlanModifier normalizes away a single trailing
// newline on the planned value - the shape Terraform's own heredoc
// syntax produces (a heredoc's closing newline is always part of the
// string value, even with the `<<-` indent-strip variant; only wrapping
// it in chomp() removes it), while Matomo's stored value doesn't
// necessarily retain that trailing newline. Applied to
// variable_customjsfunction.js_function and tag_customhtml.custom_html,
// both free-form multi-line code fields users commonly configure via
// heredoc.
//
// It fixes two distinct failures, which is why it has to act both with
// and without prior state:
//
//   - On update, a field configured via heredoc reported a perpetual
//     "diff" on every plan even though nothing about the user's
//     configuration changed, because the prior state read back from
//     Matomo had lost the trailing newline the config still carried.
//     Planning the prior state verbatim suppresses that non-diff.
//
//   - On create there is no prior state to compare against, so the
//     planned value kept its trailing newline while Create reads the tag
//     or variable back from Matomo before setting state (see
//     typedTagResource.Create's read-back comment) - and that read-back
//     value has the newline stripped. Since these attributes are
//     Required, not Computed, terraform-plugin-framework holds the final
//     state to the planned value exactly and Terraform rejects the apply
//     outright with "Provider produced inconsistent result after apply",
//     so a heredoc value could not be created at all without the user
//     wrapping it in chomp() by hand. Planning the chomped value makes
//     the plan agree with what Matomo will store.
//
// Mirrors Terraform's own chomp() function exactly (strip one trailing
// "\r\n" or "\n", not every trailing newline) rather than trimming all
// trailing whitespace, so a value that legitimately ends with several
// blank lines still reports a real diff if the blank-line count changes.
type trimTrailingNewlinePlanModifier struct{}

func (m trimTrailingNewlinePlanModifier) Description(ctx context.Context) string {
	return m.MarkdownDescription(ctx)
}

func (trimTrailingNewlinePlanModifier) MarkdownDescription(_ context.Context) string {
	return "Ignores a single trailing newline (as commonly introduced by a heredoc), which Matomo does not retain."
}

func (trimTrailingNewlinePlanModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// A null plan value is a destroy plan, and an unknown one has no
	// string to normalize yet - neither is ours to touch.
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	chomped := chompTrailingNewline(req.PlanValue.ValueString())

	// Prior state that already matches modulo the trailing newline is not
	// a diff at all: plan it verbatim rather than the chomped value, so
	// state written by an earlier provider version (which could still
	// carry the newline) doesn't churn through a no-op update.
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() && chomped == chompTrailingNewline(req.StateValue.ValueString()) {
		resp.PlanValue = req.StateValue
		return
	}

	// Otherwise - a create, or a genuine change on update - plan the
	// chomped value, which is what Matomo will store and hand back.
	resp.PlanValue = types.StringValue(chomped)
}

// chompTrailingNewline strips exactly one trailing "\r\n" or "\n" from s,
// matching Terraform's own chomp() function semantics.
func chompTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\r\n") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "\n") {
		return s[:len(s)-1]
	}
	return s
}

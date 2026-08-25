// internal/provider/trim_trailing_newline_planmodifier.go
package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// trimTrailingNewlinePlanModifier suppresses a plan diff that is purely a
// single trailing-newline difference between the configured value and the
// prior state - the shape Terraform's own heredoc syntax produces (a
// heredoc's closing newline is always part of the string value, even with
// the `<<-` indent-strip variant; only wrapping it in chomp() removes
// it), while Matomo's stored value doesn't necessarily retain that
// trailing newline. Applied to variable_customjsfunction.js_function and
// tag_customhtml.custom_html, both free-form multi-line code fields users
// commonly configure via heredoc.
//
// This modifier only ever plans the prior state value, never a chomped
// one. Terraform requires the planned value of a non-Computed attribute
// to equal either the config value or the prior state value - retaining
// prior state is explicitly permitted ("the provider wishes to retain the
// prior value rather than the config value"), but a chomped value is
// neither, and planning one fails with "Provider produced invalid plan".
// That also means this modifier can do nothing on create, where there is
// no prior state to retain; keeping a heredoc value consistent through a
// create is chompedStringType's job (see its doc comment - it is the
// attribute's CustomType, and the two work together).
//
// With that semantic equality in place, state normally holds the
// configured value verbatim, newline and all, so this modifier is
// redundant in the steady state. It still earns its keep for state
// written before the custom type existed, where the stored value is
// already chomped: it absorbs that one-time difference at plan time
// instead of putting a no-op update in the user's plan.
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
	return "Suppresses a plan diff that is purely a single trailing-newline difference from the prior state (as commonly introduced by a heredoc)."
}

func (trimTrailingNewlinePlanModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if chompTrailingNewline(req.PlanValue.ValueString()) == chompTrailingNewline(req.StateValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
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
